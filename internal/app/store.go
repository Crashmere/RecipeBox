package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"

	"os"
	"path/filepath"

	"time"
)

const applicationID = 1380077378
const schema = `PRAGMA application_id=1380077378; PRAGMA user_version=1;
CREATE TABLE records(id TEXT PRIMARY KEY, revision INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body)),created_at TEXT NOT NULL,updated_at TEXT NOT NULL,deleted_at TEXT);
CREATE INDEX record_updated ON records(deleted_at,updated_at DESC,id DESC);
CREATE INDEX record_purchase ON records(deleted_at,coalesce(nullif(json_extract(body,'$.purchaseDate'),''),substr(created_at,1,10)) DESC,id DESC);
CREATE TABLE media(id TEXT PRIMARY KEY,record_id TEXT REFERENCES records(id) ON DELETE CASCADE, body TEXT NOT NULL CHECK(json_valid(body)),created_at TEXT NOT NULL,removed_at TEXT);
CREATE INDEX media_record ON media(record_id);
CREATE TABLE changes(record_id TEXT NOT NULL REFERENCES records(id) ON DELETE CASCADE,revision INTEGER NOT NULL,body TEXT NOT NULL,PRIMARY KEY(record_id,revision));
CREATE TABLE operations(key TEXT PRIMARY KEY,fingerprint TEXT NOT NULL,result TEXT NOT NULL,created_at TEXT NOT NULL);
`

type Store struct {
	DB          *sql.DB
	Dir         string
	UploadSlot  chan struct{}
	ExportSlot  chan struct{}
	MediaLimit  int64
	FreeReserve uint64
}

func Open(dir string, init bool) (*Store, error) {
	dir, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	dbPath := filepath.Join(dir, "recipebox.db")
	if init {
		if e = os.MkdirAll(dir, 0700); e != nil {
			return nil, e
		}
		f, e := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		f.Close()
	} else {
		if _, e = os.Stat(dbPath); e != nil {
			return nil, fmt.Errorf("database must exist; use init only for a new installation: %w", e)
		}
	}
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	good := false
	defer func() {
		if !good {
			db.Close()
		}
	}()
	if _, e = db.Exec("PRAGMA journal_mode=WAL"); e != nil {
		return nil, e
	}
	if init {
		if _, e = db.Exec(schema); e != nil {
			return nil, e
		}
	}
	var appID, version int
	if e = db.QueryRow("PRAGMA application_id").Scan(&appID); e != nil {
		return nil, e
	}
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		return nil, e
	}
	if appID != applicationID || version != 1 {
		return nil, fmt.Errorf("incompatible database identity or schema")
	}
	for _, d := range []string{"media", "tmp"} {
		if e = os.MkdirAll(filepath.Join(dir, d), 0700); e != nil {
			return nil, e
		}
	}
	good = true
	return &Store{DB: db, Dir: dir, UploadSlot: make(chan struct{}, 1), ExportSlot: make(chan struct{}, 1), MediaLimit: 5 << 30, FreeReserve: 5 << 30}, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func load(ctx context.Context, q queryer, id string) (Record, error) {
	var f Record
	var b string
	e := q.QueryRowContext(ctx, "SELECT body FROM records WHERE id=?", id).Scan(&b)
	if errors.Is(e, sql.ErrNoRows) {
		return f, fail(404, "not_found", "记录不存在")
	}
	if e != nil {
		return f, e
	}
	e = json.Unmarshal([]byte(b), &f)
	return f, e
}
func (s *Store) Get(ctx context.Context, id string) (Record, error) { return load(ctx, s.DB, id) }
func fingerprint(method, path string, b []byte) string {
	h := sha256.Sum256(append([]byte(method+" "+path+"\n"), b...))
	return hex.EncodeToString(h[:])
}
func replay(ctx context.Context, tx *sql.Tx, key, fp string) ([]byte, error) {
	var old, b string
	e := tx.QueryRowContext(ctx, "SELECT fingerprint,result FROM operations WHERE key=?", key).Scan(&old, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if old != fp {
		return nil, fail(409, "key_reused", "此提交编号已被使用，请重新核对")
	}
	return []byte(b), nil
}
func (s *Store) Write(ctx context.Context, id, key, fp string, in WriteInput) ([]byte, error) {
	if !idPattern.MatchString(key) {
		return nil, fail(400, "key", "缺少有效提交编号")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if cached, e := replay(ctx, tx, key, fp); cached != nil || e != nil {
		return cached, e
	}
	r := Record{ID: ID(), Kind: in.Kind, CreatedAt: now(), Logs: []CookLog{}}
	action := in.Action
	if action == "" {
		action = "save"
	}
	if id != "" {
		r, e = load(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if r.Revision != in.Revision {
			return nil, fail(409, "revision_conflict", "内容已在另一台设备更新。你的输入已保留，请刷新后核对。")
		}
		if r.DeletedAt != nil && action != "restore" {
			return nil, fail(409, "deleted", "记录已移入回收站")
		}
	} else if action != "save" {
		return nil, fail(400, "action", "请先保存记录")
	}
	rev := r.Revision
	switch action {
	case "save":
		old := r
		r = in.Record
		r.ID = old.ID
		r.Kind = old.Kind
		r.CreatedAt = old.CreatedAt
		r.DeletedAt = nil
		r.Logs = old.Logs
		if e = r.Validate(); e != nil {
			return nil, e
		}
	case "favorite":
		if r.Kind != "recipe" {
			return nil, fail(422, "kind", "仅菜谱可收藏")
		}
		r.Favorite = in.Favorite
	case "consume":
		if r.Kind != "pantry" || r.Quantity == 0 {
			return nil, fail(409, "quantity", "已经吃完了，可以编辑数量补货")
		}
		r.Quantity--
	case "log":
		if r.Kind != "recipe" {
			return nil, fail(422, "kind", "仅菜谱有下厨记录")
		}
		l := in.Log
		if e = l.Validate(); e != nil {
			return nil, e
		}
		if l.ID == "" {
			if len(r.Logs) >= 1000 {
				return nil, fail(422, "limit", "此菜谱下厨记录已达 1000 条")
			}
			l.ID = ID()
			r.Logs = append(r.Logs, l)
		} else {
			found := false
			for i := range r.Logs {
				if r.Logs[i].ID == l.ID {
					r.Logs[i] = l
					found = true
				}
			}
			if !found {
				return nil, fail(404, "log", "下厨记录不存在")
			}
		}
	case "remove_log":
		found := false
		for i, l := range r.Logs {
			if l.ID == in.Log.ID {
				r.Logs = append(r.Logs[:i], r.Logs[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return nil, fail(404, "log", "下厨记录不存在")
		}
	case "delete":
		t := now()
		r.DeletedAt = &t
	case "restore":
		if r.DeletedAt == nil {
			return nil, fail(409, "restore", "记录不在回收站")
		}
		t, _ := time.Parse(time.RFC3339Nano, *r.DeletedAt)
		if time.Since(t) > 30*24*time.Hour {
			return nil, fail(410, "expired", "回收期限已过")
		}
		r.DeletedAt = nil
	default:
		return nil, fail(400, "action", "未知操作")
	}
	r.Revision = rev + 1
	r.UpdatedAt = now()
	if action == "save" {
		for _, mid := range r.PhotoIDs {
			var owner, removed sql.NullString
			var created string
			e = tx.QueryRowContext(ctx, "SELECT record_id,removed_at,created_at FROM media WHERE id=?", mid).Scan(&owner, &removed, &created)
			if e != nil {
				return nil, fail(422, "photos", "照片不存在，请重新上传")
			}
			t, _ := time.Parse(time.RFC3339Nano, created)
			if (owner.Valid && owner.String != r.ID) || removed.Valid || (!owner.Valid && time.Since(t) > 24*time.Hour) {
				return nil, fail(422, "photos", "照片已过期或属于其他记录，请重新上传")
			}
		}
	}
	body, _ := json.Marshal(r)
	_, e = tx.ExecContext(ctx, "INSERT INTO records(id,revision,body,created_at,updated_at,deleted_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,body=excluded.body,updated_at=excluded.updated_at,deleted_at=excluded.deleted_at", r.ID, r.Revision, string(body), r.CreatedAt, r.UpdatedAt, r.DeletedAt)
	if e != nil {
		return nil, e
	}
	if action == "save" {
		if _, e = tx.ExecContext(ctx, "UPDATE media SET removed_at=? WHERE record_id=? AND removed_at IS NULL", r.UpdatedAt, r.ID); e != nil {
			return nil, e
		}
		for _, mid := range r.PhotoIDs {
			if _, e = tx.ExecContext(ctx, "UPDATE media SET record_id=?,removed_at=NULL WHERE id=?", r.ID, mid); e != nil {
				return nil, e
			}
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO changes VALUES(?,?,?)", r.ID, r.Revision, string(body)); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(body), now()); e != nil {
		return nil, e
	}
	return body, tx.Commit()
}
func (s *Store) List(ctx context.Context, trash bool) ([]Record, error) {
	where := "deleted_at IS NULL"
	if trash {
		where = "deleted_at IS NOT NULL"
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT body FROM records WHERE "+where+" ORDER BY updated_at DESC,id DESC")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var b string
		var r Record
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(b), &r); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
