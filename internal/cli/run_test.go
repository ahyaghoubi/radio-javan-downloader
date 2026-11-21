package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ahyaghoubi/radio-javan-downloader/v2/internal/download"
	"github.com/ahyaghoubi/radio-javan-downloader/v2/internal/rj"
)

// fileServer serves files by path, supports Range requests and records
// every request it gets.
type fileServer struct {
	*httptest.Server
	files map[string][]byte

	mu       sync.Mutex
	requests []string // "GET /a.mp3 bytes=10-"
}

func newFileServer(t *testing.T, files map[string][]byte) *fileServer {
	fs := &fileServer{files: files}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		fs.requests = append(fs.requests, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+r.Header.Get("Range")))
		fs.mu.Unlock()
		data, ok := fs.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeContent(w, r, r.URL.Path, time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fileServer) took() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	r := fs.requests
	fs.requests = nil
	return r
}

func testApp(t *testing.T) (*app, *bytes.Buffer) {
	stderr := &bytes.Buffer{}
	dl := download.New("rjdl-test")
	dl.Retries = 0
	a := &app{
		opts:   options{output: t.TempDir(), audioFormat: "mp3", videoQuality: "best", jobs: 2},
		client: rj.NewClient(),
		dl:     dl,
		stdout: &bytes.Buffer{},
		stderr: stderr,
	}
	return a, stderr
}

func song(id, title string, formats ...rj.Format) *rj.Media {
	return &rj.Media{Kind: rj.KindSong, ID: id, Title: title, Artist: "Raha", Formats: formats}
}

func mp3(url string) rj.Format { return rj.Format{ID: "mp3-256", Ext: "mp3", Bitrate: 256, URL: url} }

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestExecuteDownloadsSkipsAndForces(t *testing.T) {
	srv := newFileServer(t, map[string][]byte{"/a.mp3": []byte("song a"), "/b.mp3": []byte("song b")})
	a, stderr := testApp(t)
	mix := &rj.Collection{Kind: rj.KindPlaylist, Title: "Mix", Items: []*rj.Media{
		song("1", "Oh Nana", mp3(srv.URL+"/a.mp3")),
		song("2", "Jaded", mp3(srv.URL+"/b.mp3")),
	}}
	run := func() error { return a.execute(context.Background(), a.plan(mix)) }

	if err := run(); err != nil {
		t.Fatalf("first run: %v\n%s", err, stderr)
	}
	dir := filepath.Join(a.opts.output, "Mix")
	if got := readFile(t, filepath.Join(dir, "Raha - Oh Nana.mp3")); got != "song a" {
		t.Errorf("Oh Nana = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Raha - Jaded.mp3")); got != "song b" {
		t.Errorf("Jaded = %q", got)
	}
	if n := len(srv.took()); n != 2 {
		t.Errorf("first run made %d requests, want 2", n)
	}
	if !strings.Contains(stderr.String(), "2 downloaded") {
		t.Errorf("summary missing:\n%s", stderr)
	}

	// Running again finds both files and downloads nothing.
	stderr.Reset()
	if err := run(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if reqs := srv.took(); len(reqs) != 0 {
		t.Errorf("second run requested %v", reqs)
	}
	if !strings.Contains(stderr.String(), "0 downloaded (0 B), 2 already there") {
		t.Errorf("second run summary:\n%s", stderr)
	}

	// --force downloads and overwrites.
	srv.files["/a.mp3"] = []byte("song a v2")
	a.opts.force = true
	if err := run(); err != nil {
		t.Fatalf("forced run: %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "Raha - Oh Nana.mp3")); got != "song a v2" {
		t.Errorf("after --force Oh Nana = %q", got)
	}
}

func TestExecuteResumesPartFile(t *testing.T) {
	data := []byte(strings.Repeat("0123456789", 100))
	srv := newFileServer(t, map[string][]byte{"/a.mp3": data})
	a, stderr := testApp(t)
	m := song("1", "Oh Nana", mp3(srv.URL+"/a.mp3"))
	dst := filepath.Join(a.opts.output, "Raha - Oh Nana.mp3")
	if err := os.WriteFile(dst+".part", data[:400], 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.execute(context.Background(), a.plan(m)); err != nil {
		t.Fatalf("%v\n%s", err, stderr)
	}
	if got := readFile(t, dst); got != string(data) {
		t.Errorf("resumed file has %d bytes, want %d", len(got), len(data))
	}
	if reqs := srv.took(); len(reqs) != 1 || reqs[0] != "GET /a.mp3 bytes=400-" {
		t.Errorf("requests = %v, want one ranged GET from byte 400", reqs)
	}
	if _, err := os.Stat(dst + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".part file left behind: %v", err)
	}
}

func TestExecuteReportsFailuresAndKeepsGoing(t *testing.T) {
	srv := newFileServer(t, map[string][]byte{"/a.mp3": []byte("song a")})
	a, stderr := testApp(t)
	mix := &rj.Collection{Kind: rj.KindPlaylist, Title: "Mix", Items: []*rj.Media{
		song("1", "Oh Nana", mp3(srv.URL+"/a.mp3")),
		song("2", "Missing", mp3(srv.URL+"/gone.mp3")),
		song("3", "No Files"),
	}}

	err := a.execute(context.Background(), a.plan(mix))
	if !errors.Is(err, errSilent) {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if got := readFile(t, filepath.Join(a.opts.output, "Mix", "Raha - Oh Nana.mp3")); got != "song a" {
		t.Errorf("good file = %q", got)
	}
	out := stderr.String()
	for _, want := range []string{"1 downloaded", "2 failed", "Raha - Missing:", "Raha - No Files: Radio Javan offers no file"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(a.opts.output, "Mix", "Raha - Missing.mp3")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("failed download left a file: %v", err)
	}
}

func TestExecuteAudioFormat(t *testing.T) {
	srv := newFileServer(t, map[string][]byte{"/a.mp3": []byte("mp3"), "/a.m4a": []byte("aac")})
	a, stderr := testApp(t)
	a.opts.audioFormat = "m4a"
	m := song("1", "Oh Nana", mp3(srv.URL+"/a.mp3"), rj.Format{ID: "aac-256", Ext: "m4a", Bitrate: 256, URL: srv.URL + "/a.m4a"})

	for run := 1; run <= 2; run++ {
		if err := a.execute(context.Background(), a.plan(m)); err != nil {
			t.Fatalf("run %d: %v\n%s", run, err, stderr)
		}
	}
	if got := readFile(t, filepath.Join(a.opts.output, "Raha - Oh Nana.m4a")); got != "aac" {
		t.Errorf("m4a = %q", got)
	}
	if _, err := os.Stat(filepath.Join(a.opts.output, "Raha - Oh Nana.mp3")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("mp3 downloaded with --audio-format m4a: %v", err)
	}
	if reqs := srv.took(); len(reqs) != 1 || reqs[0] != "GET /a.m4a" {
		t.Errorf("requests = %v, want one GET of the m4a", reqs)
	}
}

func TestExecuteWritesLyricsAndMetadata(t *testing.T) {
	srv := newFileServer(t, map[string][]byte{"/a.mp3": []byte("song a"), "/b.mp3": []byte("song b")})
	a, stderr := testApp(t)
	a.opts.lyrics, a.opts.writeJSON = true, true
	synced := song("1", "Oh Nana", mp3(srv.URL+"/a.mp3"))
	synced.SyncedLyrics = []rj.LyricLine{{Time: 1.5, Text: "oh nana"}}
	plain := song("2", "Jaded", mp3(srv.URL+"/b.mp3"))
	plain.Lyrics = "line one"
	mix := &rj.Collection{Kind: rj.KindPlaylist, ID: "p1", Title: "Mix", Items: []*rj.Media{synced, plain}}

	if err := a.execute(context.Background(), a.plan(mix)); err != nil {
		t.Fatalf("%v\n%s", err, stderr)
	}
	dir := filepath.Join(a.opts.output, "Mix")
	if got := readFile(t, filepath.Join(dir, "Raha - Oh Nana.lrc")); !strings.Contains(got, "[00:01.50]oh nana") {
		t.Errorf("lrc = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Raha - Jaded.txt")); got != "line one\n" {
		t.Errorf("txt = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Raha - Jaded.json")); !strings.Contains(got, `"title": "Jaded"`) {
		t.Errorf("song json = %s", got)
	}
	if got := readFile(t, filepath.Join(dir, "info.json")); !strings.Contains(got, `"title": "Mix"`) {
		t.Errorf("playlist json = %s", got)
	}
}
