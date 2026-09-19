package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const MaxUpload = 25 << 20

func (s *Store) Lock(exclusive bool) (func(), error) {
	f, e := os.OpenFile(filepath.Join(s.Dir, "maintenance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	kind := unix.LOCK_SH
	if exclusive {
		kind = unix.LOCK_EX
	}
	if e = unix.Flock(int(f.Fd()), kind|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, fail(429, "maintenance", "维护任务正在进行，请稍后重试")
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
}
func (s *Store) capacity() error {
	var stat unix.Statfs_t
	if e := unix.Statfs(s.Dir, &stat); e != nil {
		return e
	}
	if uint64(stat.Bavail)*uint64(stat.Bsize) < s.FreeReserve {
		return fail(507, "disk_full", "服务器空间不足，暂时无法上传")
	}
	var used int64
	e := s.DB.QueryRow("SELECT coalesce(sum(json_extract(body,'$.bytes')),0) FROM media").Scan(&used)
	if e != nil {
		return e
	}
	if used >= s.MediaLimit {
		return fail(507, "quota", "照片存储已达到限额，请联系维护者扩容")
	}
	return nil
}
func imageKind(b []byte) string {
	switch {
	case len(b) >= 3 && bytes.Equal(b[:3], []byte{255, 216, 255}):
		return "jpeg"
	case bytes.HasPrefix(b, []byte{137, 80, 78, 71, 13, 10, 26, 10}):
		return "png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp"
	case len(b) >= 16 && string(b[4:8]) == "ftyp":
		for i := 8; i+4 <= len(b) && i < 64; i += 4 {
			if i == 12 {
				continue
			}
			switch string(b[i : i+4]) {
			case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
				return "heif"
			}
		}
	}
	return ""
}
func runImage(ctx context.Context, tool string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = append(os.Environ(), "VIPS_CONCURRENCY=1", "VIPS_DISC_THRESHOLD=32m", "MALLOC_ARENA_MAX=2")
	out, e := cmd.CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("%s failed: %w: %.500s", tool, e, out)
	}
	return out, nil
}
func headerInt(ctx context.Context, file, field string) (int, error) {
	b, e := runImage(ctx, "vipsheader", "-f", field, file)
	if e != nil {
		return 0, e
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}
func syncFile(path string) error {
	f, e := os.OpenFile(path, os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = f.Chmod(0600); e != nil {
		return e
	}
	return f.Sync()
}
func (s *Store) Upload(ctx context.Context, key string, r io.Reader) (Media, error) {
	var m Media
	if !idPattern.MatchString(key) {
		return m, fail(400, "idempotency_required", "缺少上传编号")
	}
	select {
	case s.UploadSlot <- struct{}{}:
		defer func() { <-s.UploadSlot }()
	default:
		return m, fail(429, "busy", "正在处理照片，请稍后重试")
	}
	unlock, e := s.Lock(false)
	if e != nil {
		return m, e
	}
	defer unlock()
	if e = s.capacity(); e != nil {
		return m, e
	}
	tmp, e := os.MkdirTemp(filepath.Join(s.Dir, "tmp"), "upload-")
	if e != nil {
		return m, e
	}
	defer os.RemoveAll(tmp)
	input := filepath.Join(tmp, "input")
	f, e := os.OpenFile(input, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return m, e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, MaxUpload+1))
	closeErr := f.Close()
	if e != nil {
		return m, e
	}
	if closeErr != nil {
		return m, closeErr
	}
	if n > MaxUpload {
		return m, fail(413, "too_large", "单张照片不能超过 25 MB")
	}
	fp := "upload:" + hex.EncodeToString(h.Sum(nil))
	var old, result string
	e = s.DB.QueryRowContext(ctx, "SELECT fingerprint,result FROM operations WHERE key=?", key).Scan(&old, &result)
	if e == nil {
		if old != fp {
			return m, fail(409, "key_reused", "上传编号已被使用")
		}
		e = json.Unmarshal([]byte(result), &m)
		return m, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return m, e
	}
	src, e := os.Open(input)
	if e != nil {
		return m, e
	}
	prefix := make([]byte, 64)
	nread, _ := src.Read(prefix)
	src.Close()
	kind := imageKind(prefix[:nread])
	if kind == "" {
		return m, fail(415, "format", "请选择 JPG、PNG、WebP 或 HEIC 照片")
	}
	ictx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	w, e := headerInt(ictx, input, "width")
	if e != nil {
		return m, fail(422, "image_decode", "照片无法读取，请重试或选择其他照片")
	}
	height, e := headerInt(ictx, input, "height")
	if e != nil || w <= 0 || height <= 0 || int64(w)*int64(height) > 80000000 {
		return m, fail(422, "pixels", "照片尺寸无效或超过 8000 万像素")
	}
	if kind != "heif" {
		pages, pe := headerInt(ictx, input, "n-pages")
		if pe == nil && pages > 1 {
			return m, fail(415, "animated", "暂不支持动态图片，请选择静态照片")
		}
	}
	main := filepath.Join(tmp, "main.jpg")
	thumb := filepath.Join(tmp, "thumb.webp")
	// export-profile is also accepted by newer libvips; output-profile is
	// unavailable in the 8.15 packages shipped by the CI runner.
	if _, e = runImage(ictx, "vips", "thumbnail", input, main+"[Q=88,strip,background=248 247 243]", "2560", "--height=2560", "--size=down", "--export-profile=srgb", "--fail-on=error"); e != nil {
		log.Printf("image conversion: %v", e)
		return m, fail(422, "image_decode", "照片处理失败，请重试或换一张照片")
	}
	if _, e = runImage(ictx, "vips", "thumbnail", main, thumb+"[Q=80,strip]", "640", "--height=640", "--size=down"); e != nil {
		return m, e
	}
	w, e = headerInt(ictx, main, "width")
	if e != nil {
		return m, e
	}
	height, e = headerInt(ictx, main, "height")
	if e != nil {
		return m, e
	}
	m = Media{ID: ID(), Width: w, Height: height, CreatedAt: now()}
	for _, p := range []string{main, thumb} {
		if e = syncFile(p); e != nil {
			return m, e
		}
		info, e := os.Stat(p)
		if e != nil {
			return m, e
		}
		m.Bytes += info.Size()
	}
	if e = os.Rename(main, filepath.Join(s.Dir, "media", m.ID+".jpg")); e != nil {
		return m, e
	}
	if e = os.Rename(thumb, filepath.Join(s.Dir, "media", m.ID+".webp")); e != nil {
		return m, e
	}
	d, e := os.Open(filepath.Join(s.Dir, "media"))
	if e != nil {
		return m, e
	}
	e = d.Sync()
	d.Close()
	if e != nil {
		return m, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return m, e
	}
	defer tx.Rollback()
	b, _ := json.Marshal(m)
	if _, e = tx.ExecContext(ctx, "INSERT INTO media(id,body,created_at) VALUES(?,?,?)", m.ID, string(b), m.CreatedAt); e != nil {
		return m, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(b), now()); e != nil {
		return m, e
	}
	return m, tx.Commit()
}
func (s *Store) MediaPath(ctx context.Context, id, variant string) (string, error) {
	if !idPattern.MatchString(id) || (variant != "main" && variant != "thumb") {
		return "", fail(404, "not_found", "照片不存在")
	}
	var owner, removed, deleted sql.NullString
	var created string
	e := s.DB.QueryRowContext(ctx, "SELECT m.record_id,m.removed_at,m.created_at,f.deleted_at FROM media m LEFT JOIN records f ON f.id=m.record_id WHERE m.id=?", id).Scan(&owner, &removed, &created, &deleted)
	if e != nil || removed.Valid || deleted.Valid {
		return "", fail(404, "not_found", "照片不存在")
	}
	t, _ := time.Parse(time.RFC3339Nano, created)
	if !owner.Valid && time.Since(t) > 24*time.Hour {
		return "", fail(404, "expired", "照片已过期")
	}
	ext := ".jpg"
	if variant == "thumb" {
		ext = ".webp"
	}
	return filepath.Join(s.Dir, "media", id+ext), nil
}
