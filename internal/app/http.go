package app

import (
	"archive/zip"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func errorResponse(w http.ResponseWriter, e error) {
	var p *Problem
	if !errors.As(e, &p) {
		log.Printf("request error: %v", e)
		p = &Problem{500, "internal", "服务暂时无法完成请求，请稍后重试"}
	}
	jsonResponse(w, p.Status, p)
}
func originOK(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, e := url.Parse(o)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return e == nil && u.Host == r.Host && u.Scheme == scheme && u.Path == ""
}
func (s *Store) Handler(assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	limits := &limiter{}
	handle := func(p string, f func(http.ResponseWriter, *http.Request) error) {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			if e := f(w, r); e != nil {
				errorResponse(w, e)
			}
		})
	}
	handle("GET /healthz", func(w http.ResponseWriter, r *http.Request) error {
		if e := s.DB.PingContext(r.Context()); e != nil {
			return e
		}
		jsonResponse(w, 200, map[string]string{"status": "ok"})
		return nil
	})
	handle("GET /api/records", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.List(r.Context(), r.URL.Query().Get("trash") == "1")
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	handle("GET /api/records/{id}", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.Get(r.Context(), r.PathValue("id"))
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	write := func(w http.ResponseWriter, r *http.Request) error {
		start := time.Now()
		b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
		if e != nil {
			return fail(413, "size", "表单内容过大")
		}
		var in WriteInput
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if e = d.Decode(&in); e != nil {
			return fail(400, "json", "表单格式不正确")
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return fail(400, "json", "表单格式不正确")
		}
		out, replayed, e := s.write(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"), fingerprint(r.Method, r.URL.Path, b), in)
		status := 200
		var p *Problem
		if errors.As(e, &p) {
			status = p.Status
		} else if e != nil {
			status = 500
		}
		log.Printf("write %s action=%s status=%d replay=%t %s", r.URL.Path, cmp.Or(in.Action, "save"), status, replayed, time.Since(start).Round(time.Microsecond))
		if e == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(out)
		}
		return e
	}
	handle("POST /api/records", write)
	handle("POST /api/records/{id}", write)
	handle("GET /api/operations/{key}", func(w http.ResponseWriter, r *http.Request) error {
		var b string
		if e := s.DB.QueryRowContext(r.Context(), "SELECT result FROM operations WHERE key=?", r.PathValue("key")).Scan(&b); e != nil {
			return fail(404, "not_found", "提交记录不存在")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(b))
		return nil
	})
	handle("POST /api/uploads", func(w http.ResponseWriter, r *http.Request) error {
		m, e := s.Upload(r.Context(), r.Header.Get("Idempotency-Key"), http.MaxBytesReader(w, r.Body, MaxUpload+1))
		if e == nil {
			jsonResponse(w, 201, m)
		}
		return e
	})
	handle("GET /api/media/{id}/{variant}", func(w http.ResponseWriter, r *http.Request) error {
		p, e := s.MediaPath(r.Context(), r.PathValue("id"), r.PathValue("variant"))
		if e != nil {
			return e
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeFile(w, r, p)
		return nil
	})
	handle("GET /api/export", func(w http.ResponseWriter, r *http.Request) error {
		select {
		case s.ExportSlot <- struct{}{}:
			defer func() { <-s.ExportSlot }()
		default:
			return fail(429, "busy", "正在导出，请稍后再试")
		}
		unlock, e := s.Lock(false)
		if e != nil {
			return e
		}
		defer unlock()
		records, e := s.List(r.Context(), false)
		if e != nil {
			return e
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=recipebox.zip")
		z := zip.NewWriter(w)
		defer z.Close()
		f, e := z.Create("recipes-and-pantry.json")
		if e != nil {
			return e
		}
		if e = json.NewEncoder(f).Encode(records); e != nil {
			return e
		}
		for _, record := range records {
			for _, id := range record.PhotoIDs {
				src, e := os.Open(filepath.Join(s.Dir, "media", id+".jpg"))
				if e != nil {
					return e
				}
				dst, e := z.Create("photos/" + id + ".jpg")
				if e != nil {
					src.Close()
					return e
				}
				_, e = io.Copy(dst, src)
				src.Close()
				if e != nil {
					return e
				}
			}
		}
		return nil
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(p, "api/") {
			http.NotFound(w, r)
			return
		}
		if p == "" {
			p = "index.html"
		}
		b, e := fs.ReadFile(assets, p)
		if e != nil {
			if strings.Contains(filepath.Base(p), ".") {
				http.NotFound(w, r)
				return
			}
			b, e = fs.ReadFile(assets, "index.html")
			p = "index.html"
		}
		if e != nil {
			http.NotFound(w, r)
			return
		}
		if p == "index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
		} else if strings.HasSuffix(p, ".js") {
			w.Header().Set("Content-Type", "text/javascript")
		} else if strings.HasSuffix(p, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(p, ".svg") {
			w.Header().Set("Content-Type", "image/svg+xml")
		}
		w.Write(b)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" {
			if !originOK(r) {
				errorResponse(w, fail(403, "origin", "请从本站提交操作"))
				return
			}
			if !limits.allow(r) {
				errorResponse(w, fail(429, "rate", "操作太频繁，请稍后再试"))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
