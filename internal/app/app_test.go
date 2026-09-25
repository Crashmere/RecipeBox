package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(filepath.Join(t.TempDir(), "data"), true)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	return s
}
func save(t *testing.T, s *Store, id string, in WriteInput) Record {
	t.Helper()
	b, _ := json.Marshal(in)
	out, e := s.Write(context.Background(), id, ID(), fingerprint("POST", id, b), in)
	if e != nil {
		t.Fatal(e)
	}
	var r Record
	if e = json.Unmarshal(out, &r); e != nil {
		t.Fatal(e)
	}
	return r
}
func assertStatus(t *testing.T, e error, status int) {
	t.Helper()
	var p *Problem
	if !errors.As(e, &p) || p.Status != status {
		t.Fatalf("expected %d, got %v", status, e)
	}
}
func TestRecordsRevisionsAndIdempotency(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	in := WriteInput{Record: Record{Name: "番茄炒蛋", Kind: "recipe", Ingredients: []Ingredient{{"番茄", "2个"}, {"", ""}}, Steps: []string{"切番茄", "", "炒蛋"}}}
	key := ID()
	a, replayed, e := s.write(ctx, "", key, "same", in)
	if e != nil || replayed {
		t.Fatal(replayed, e)
	}
	b, replayed, e := s.write(ctx, "", key, "same", in)
	if e != nil || !replayed || !bytes.Equal(a, b) {
		t.Fatalf("replay %t %s %v", replayed, b, e)
	}
	_, e = s.Write(ctx, "", key, "different", in)
	assertStatus(t, e, 409)
	var r Record
	json.Unmarshal(a, &r)
	if len(r.Ingredients) != 1 || len(r.Steps) != 2 {
		t.Fatal(r)
	}
	log := save(t, s, r.ID, WriteInput{Record: Record{Revision: r.Revision}, Action: "log", Log: CookLog{Date: "2026-09-19", Note: "少放盐", Rating: 5}})
	if len(log.Logs) != 1 {
		t.Fatal(log)
	}
	_, e = s.Write(ctx, r.ID, ID(), "stale", WriteInput{Record: r})
	assertStatus(t, e, 409)
	log.Name = "番茄炒蛋 · 家常"
	log.Logs = nil
	edited := save(t, s, r.ID, WriteInput{Record: log})
	if len(edited.Logs) != 1 {
		t.Fatal("editing removed logs")
	}
	l := edited.Logs[0]
	l.Note = "再少一点盐"
	edited = save(t, s, r.ID, WriteInput{Record: Record{Revision: edited.Revision}, Action: "log", Log: l})
	if edited.Logs[0].Note != l.Note || len(edited.Logs) != 1 {
		t.Fatal(edited.Logs)
	}
	deleted := save(t, s, r.ID, WriteInput{Record: Record{Revision: edited.Revision}, Action: "delete"})
	list, _ := s.List(ctx, false)
	if len(list) != 0 || deleted.DeletedAt == nil {
		t.Fatal(list)
	}
	list, _ = s.List(ctx, true)
	if len(list) != 1 {
		t.Fatal(list)
	}
	restored := save(t, s, r.ID, WriteInput{Record: Record{Revision: deleted.Revision}, Action: "restore"})
	if restored.DeletedAt != nil || len(restored.Logs) != 1 {
		t.Fatal(restored)
	}
}
func TestConcurrentPantryConsumption(t *testing.T) {
	s := testStore(t)
	r := save(t, s, "", WriteInput{Record: Record{Name: "挂面", Kind: "pantry", Quantity: 2, Unit: "包"}})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Write(context.Background(), r.ID, ID(), ID(), WriteInput{Record: Record{Revision: r.Revision}, Action: "consume"})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	conflicts := 0
	for e := range results {
		if e == nil {
			ok++
		} else {
			assertStatus(t, e, 409)
			conflicts++
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal(ok, conflicts)
	}
	r, _ = s.Get(context.Background(), r.ID)
	if r.Quantity != 1 {
		t.Fatal(r.Quantity)
	}
	r = save(t, s, r.ID, WriteInput{Record: Record{Revision: r.Revision}, Action: "consume"})
	_, e := s.Write(context.Background(), r.ID, ID(), "empty", WriteInput{Record: Record{Revision: r.Revision}, Action: "consume"})
	assertStatus(t, e, 409)
}
func TestValidationAndMissingDatabase(t *testing.T) {
	if _, e := Open(filepath.Join(t.TempDir(), "missing"), false); e == nil {
		t.Fatal("missing DB silently initialized")
	}
	for _, r := range []Record{{Name: "", Kind: "recipe"}, {Name: "a", Kind: "bad"}, {Name: "a", Kind: "pantry", Quantity: -1}, {Name: "a", Kind: "pantry", PurchaseDate: "2026-10-20", ExpiryDate: "2026-01-01"}, {Name: "a", Kind: "recipe", PhotoIDs: []string{"../bad"}}, {Name: "a", Kind: "recipe", Ingredients: []Ingredient{{"", "100g"}}}} {
		if r.Validate() == nil {
			t.Fatalf("accepted %#v", r)
		}
	}
}
func imageFixture(t *testing.T) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			m.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 30, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, m)
	return b.Bytes()
}
func TestMediaBackupRestoreAndCleanup(t *testing.T) {
	if _, e := exec.LookPath("vips"); e != nil {
		t.Fatal("libvips required for media integration test")
	}
	s := testStore(t)
	ctx := context.Background()
	s.FreeReserve = 0
	m, e := s.Upload(ctx, ID(), bytes.NewReader(imageFixture(t)))
	if e != nil {
		t.Fatal(e)
	}
	r := save(t, s, "", WriteInput{Record: Record{Name: "测试菜", Kind: "recipe", PhotoIDs: []string{m.ID}}})
	if _, e = s.MediaPath(ctx, m.ID, "main"); e != nil {
		t.Fatal(e)
	}
	_, e = s.Write(ctx, "", ID(), "steal", WriteInput{Record: Record{Name: "他菜", Kind: "recipe", PhotoIDs: []string{m.ID}}})
	assertStatus(t, e, 422)
	out := filepath.Join(t.TempDir(), "snapshot")
	if e = s.Backup(ctx, out); e != nil {
		t.Fatal(e)
	}
	if _, e = VerifyBackup(out); e != nil {
		t.Fatal(e)
	}
	dst := filepath.Join(t.TempDir(), "restore")
	if e = Restore(ctx, out, dst); e != nil {
		t.Fatal(e)
	}
	restored, e := Open(dst, false)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.DB.Close()
	rr, e := restored.Get(ctx, r.ID)
	if e != nil || len(rr.PhotoIDs) != 1 {
		t.Fatal(rr, e)
	}
	if e = restored.Check(ctx); e != nil {
		t.Fatal(e)
	}
	a, _ := os.Stat(filepath.Join(s.Dir, "media", m.ID+".jpg"))
	b, _ := os.Stat(filepath.Join(dst, "media", m.ID+".jpg"))
	if os.SameFile(a, b) {
		t.Fatal("restore must copy media")
	}
	del := save(t, s, r.ID, WriteInput{Record: Record{Revision: r.Revision}, Action: "delete"})
	_, e = s.MediaPath(ctx, m.ID, "main")
	assertStatus(t, e, 404)
	r = save(t, s, r.ID, WriteInput{Record: Record{Revision: del.Revision}, Action: "restore"})
	if _, e = s.MediaPath(ctx, m.ID, "main"); e != nil {
		t.Fatal(e)
	}
	r.PhotoIDs = nil
	save(t, s, r.ID, WriteInput{Record: r})
	s.DB.Exec("UPDATE media SET removed_at=?", time.Now().Add(-31*24*time.Hour).UTC().Format(time.RFC3339Nano))
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(filepath.Join(s.Dir, "media", m.ID+".jpg"), past, past)
	os.Chtimes(filepath.Join(s.Dir, "media", m.ID+".webp"), past, past)
	if e = s.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.Dir, "media", m.ID+".jpg")); !os.IsNotExist(e) {
		t.Fatal("orphan not removed")
	}
	if _, e = VerifyBackup(out); e != nil {
		t.Fatal("cleanup damaged backup", e)
	}
	os.WriteFile(filepath.Join(out, "recipebox.db"), []byte("corrupt"), 0600)
	if _, e = VerifyBackup(out); e == nil {
		t.Fatal("accepted corrupt backup")
	}
}
func TestHTTPOriginLimitsExportAndRoutes(t *testing.T) {
	s := testStore(t)
	h := s.Handler(fstest.MapFS{"index.html": {Data: []byte("<html>RecipeBox</html>")}, "assets/app.js": {Data: []byte("export{}")}})
	for _, path := range []string{"/", "/new", "/recipes/123", "/healthz", "/assets/app.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal(path, w.Code)
		}
	}
	in := `{"kind":"recipe","name":"蛋炒饭"}`
	request := func(origin, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://example.com/api/records", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Idempotency-Key", ID())
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if request("http://evil.com", in).Code != 403 {
		t.Fatal("cross origin allowed")
	}
	if request("http://example.com", in+" {}").Code != 400 {
		t.Fatal("trailing json allowed")
	}
	w := request("http://example.com", in)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/export", nil))
	z, e := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	f, e := z.Open("recipes-and-pantry.json")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if !bytes.Contains(b, []byte("蛋炒饭")) {
		t.Fatal(string(b))
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/unknown", nil))
	if w.Code != http.StatusNotFound {
		t.Fatal(w.Code)
	}
	for _, bad := range []string{"<svg>hello</svg>", fmt.Sprintf("%030d", 1)} {
		_, e = s.Upload(context.Background(), ID(), strings.NewReader(bad))
		assertStatus(t, e, 415)
	}
}
