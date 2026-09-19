package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type BackupManifest struct {
	Version   int               `json:"version"`
	CreatedAt string            `json:"createdAt"`
	Files     map[string]string `json:"files"`
}

func digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func copyFile(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}
func (s *Store) Check(ctx context.Context) error {
	if e := s.checkIntegrity(ctx); e != nil {
		return e
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT id FROM media")
	if e != nil {
		return e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if !idPattern.MatchString(id) {
			return fmt.Errorf("invalid media id")
		}
		for _, ext := range []string{".jpg", ".webp"} {
			info, e := os.Stat(filepath.Join(s.Dir, "media", id+ext))
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() || info.Size() == 0 {
				return fmt.Errorf("invalid media file")
			}
		}
	}
	for _, trash := range []bool{false, true} {
		records, e := s.List(ctx, trash)
		if e != nil {
			return e
		}
		for _, r := range records {
			if e = r.Validate(); e != nil {
				return fmt.Errorf("invalid record %s: %w", r.ID, e)
			}
			for _, id := range r.PhotoIDs {
				var n int
				if e = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM media WHERE id=? AND record_id=? AND removed_at IS NULL", id, r.ID).Scan(&n); e != nil {
					return e
				}
				if n != 1 {
					return fmt.Errorf("missing media reference for record %s", r.ID)
				}
			}
		}
	}
	return nil
}

func (s *Store) checkIntegrity(ctx context.Context) error {
	var result string
	if e := s.DB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); e != nil {
		return e
	}
	if result != "ok" {
		return fmt.Errorf("integrity check: %s", result)
	}
	rows, e := s.DB.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("foreign key check failed")
	}
	return rows.Err()
}
func (s *Store) Backup(ctx context.Context, out string) error {
	unlock, e := s.Lock(true)
	if e != nil {
		return e
	}
	defer unlock()
	if e = os.Mkdir(out, 0700); e != nil {
		return e
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(out)
		}
	}()
	if e = os.Mkdir(filepath.Join(out, "media"), 0700); e != nil {
		return e
	}
	dbPath := filepath.Join(out, "recipebox.db")
	if _, e = s.DB.ExecContext(ctx, "VACUUM INTO ?", dbPath); e != nil {
		return e
	}
	if e = os.Chmod(dbPath, 0600); e != nil {
		return e
	}
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if e != nil {
		return e
	}
	defer db.Close()
	var integrity string
	if e = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); e != nil || integrity != "ok" {
		return fmt.Errorf("snapshot integrity failure: %v", e)
	}
	rows, e := db.QueryContext(ctx, "SELECT id FROM media")
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	manifest := BackupManifest{Version: 1, CreatedAt: now(), Files: map[string]string{}}
	manifest.Files["recipebox.db"], e = digest(dbPath)
	if e != nil {
		return e
	}
	for _, id := range ids {
		if !idPattern.MatchString(id) {
			return fmt.Errorf("invalid media id")
		}
		for _, ext := range []string{".jpg", ".webp"} {
			rel := "media/" + id + ext
			src := filepath.Join(s.Dir, rel)
			dst := filepath.Join(out, rel)
			if e = os.Link(src, dst); e != nil {
				return fmt.Errorf("hard-link media backup: %w", e)
			}
			manifest.Files[rel], e = digest(dst)
			if e != nil {
				return e
			}
		}
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if e = os.WriteFile(filepath.Join(out, "manifest.json"), b, 0600); e != nil {
		return e
	}
	if e = syncFile(filepath.Join(out, "manifest.json")); e != nil {
		return e
	}
	complete = true
	return nil
}
func VerifyBackup(dir string) (BackupManifest, error) {
	var m BackupManifest
	b, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return m, e
	}
	if e = json.Unmarshal(b, &m); e != nil {
		return m, e
	}
	if m.Version != 1 || m.Files["recipebox.db"] == "" {
		return m, fmt.Errorf("invalid backup manifest")
	}
	for rel, want := range m.Files {
		if rel != "recipebox.db" {
			base := strings.TrimPrefix(rel, "media/")
			ext := filepath.Ext(base)
			if !strings.HasPrefix(rel, "media/") || !idPattern.MatchString(strings.TrimSuffix(base, ext)) || (ext != ".jpg" && ext != ".webp") {
				return m, fmt.Errorf("invalid backup path")
			}
		}
		got, e := digest(filepath.Join(dir, rel))
		if e != nil || got != want {
			return m, fmt.Errorf("backup checksum mismatch for %s", rel)
		}
	}
	return m, nil
}
func Restore(ctx context.Context, src, dst string) error {
	m, e := VerifyBackup(src)
	if e != nil {
		return e
	}
	if e = os.Mkdir(dst, 0700); e != nil {
		return e
	}
	if e = os.Mkdir(filepath.Join(dst, "media"), 0700); e != nil {
		return e
	}
	for rel := range m.Files {
		if e = copyFile(filepath.Join(src, rel), filepath.Join(dst, rel)); e != nil {
			return e
		}
	}
	s, e := Open(dst, false)
	if e != nil {
		return e
	}
	defer s.DB.Close()
	// Restored files are independent copies; verify before serving.
	return s.Check(ctx)
}
func (s *Store) Cleanup(ctx context.Context) error {
	unlock, e := s.Lock(true)
	if e != nil {
		return e
	}
	defer unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	cut := time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano)
	stale := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339Nano)
	if _, e = tx.ExecContext(ctx, "DELETE FROM records WHERE deleted_at<?", cut); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM media WHERE (record_id IS NULL AND created_at<?) OR removed_at<?", stale, cut); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM operations WHERE created_at<?", time.Now().UTC().Add(-7*24*time.Hour).Format(time.RFC3339Nano)); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, "SELECT id FROM media")
	if e != nil {
		return e
	}
	keep := map[string]bool{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		keep[id] = true
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	entries, e := os.ReadDir(filepath.Join(s.Dir, "media"))
	if e != nil {
		return e
	}
	for _, ent := range entries {
		ext := filepath.Ext(ent.Name())
		id := strings.TrimSuffix(ent.Name(), ext)
		if ent.IsDir() || !idPattern.MatchString(id) || (ext != ".jpg" && ext != ".webp") || keep[id] {
			continue
		}
		info, e := ent.Info()
		if e != nil {
			return e
		}
		if time.Since(info.ModTime()) > 24*time.Hour {
			if e = os.Remove(filepath.Join(s.Dir, "media", ent.Name())); e != nil {
				return e
			}
		}
	}
	entries, e = os.ReadDir(filepath.Join(s.Dir, "tmp"))
	if e != nil {
		return e
	}
	for _, ent := range entries {
		info, e := ent.Info()
		if e != nil {
			return e
		}
		if strings.HasPrefix(ent.Name(), "upload-") && time.Since(info.ModTime()) > 24*time.Hour {
			if e = os.RemoveAll(filepath.Join(s.Dir, "tmp", ent.Name())); e != nil {
				return e
			}
		}
	}
	return nil
}
func PruneBackups(dir string, keep int) error {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	names := []string{}
	for _, x := range entries {
		if x.IsDir() && strings.HasPrefix(x.Name(), "daily-") {
			if _, e = os.Stat(filepath.Join(dir, x.Name(), "manifest.json")); e == nil {
				names = append(names, x.Name())
			}
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i := keep; i < len(names); i++ {
		if e = os.RemoveAll(filepath.Join(dir, names[i])); e != nil {
			return e
		}
	}
	return nil
}
