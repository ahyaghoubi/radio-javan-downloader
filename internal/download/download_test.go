package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// flakyServer serves data with Range support. The first response is cut off
// after half of the file to force a resume.
func flakyServer(t *testing.T, data []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		start := 0
		if rg := r.Header.Get("Range"); rg != "" {
			start, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(len(data)-start))
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", "audio/mpeg")
		}
		body := data[start:]
		if n == 1 {
			w.Write(body[:len(body)/2])
			// Drop the connection without sending the rest.
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

type recordProgress struct {
	starts []string
	read   int64
}

func (p *recordProgress) Start(total, offset int64) {
	p.starts = append(p.starts, fmt.Sprintf("%d/%d", offset, total))
}

func (p *recordProgress) Wrap(r io.Reader) io.Reader { return &countReader{r, p} }

type countReader struct {
	r io.Reader
	p *recordProgress
}

func (c *countReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.p.read += int64(n)
	return n, err
}

func fastDownloader() *Downloader {
	d := New("test")
	d.Retries = 2
	return d
}

func TestFileResumesAfterDisconnect(t *testing.T) {
	data := bytes.Repeat([]byte("radiojavan"), 50_000)
	srv, calls := flakyServer(t, data)
	dst := filepath.Join(t.TempDir(), "song.mp3")
	p := &recordProgress{}
	size, err := fastDownloader().File(context.Background(), srv.URL, dst, p)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if size != int64(len(data)) || !bytes.Equal(got, data) {
		t.Fatalf("size %d, content equal %v", size, bytes.Equal(got, data))
	}
	if calls.Load() != 2 {
		t.Errorf("server calls = %d, want 2", calls.Load())
	}
	// The second attempt continues where the first stopped.
	if len(p.starts) != 2 || p.starts[0] != fmt.Sprintf("0/%d", len(data)) || strings.HasPrefix(p.starts[1], "0/") {
		t.Errorf("progress starts = %v", p.starts)
	}
	if _, err := os.Stat(dst + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".part file left behind")
	}
}

func TestFileUnavailable(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<Error>AccessDenied</Error>"))
	}))
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "x.mp3")
	_, err := fastDownloader().File(context.Background(), srv.URL, dst, nil)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if calls.Load() != 1 {
		t.Errorf("a missing file was retried %d times", calls.Load()-1)
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file created for a failed download")
	}
}

func TestFileRejectsHTMLBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("Not found"))
	}))
	defer srv.Close()
	_, err := fastDownloader().File(context.Background(), srv.URL, filepath.Join(t.TempDir(), "x.mp3"), nil)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestFileStallIsRetried(t *testing.T) {
	data := []byte(strings.Repeat("x", 1000))
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data[:10])
			w.(http.Flusher).Flush()
			<-r.Context().Done() // never send the rest
			return
		}
		w.Write(data)
	}))
	defer srv.Close()
	d := fastDownloader()
	d.StallTimeout = 200 * time.Millisecond
	dst := filepath.Join(t.TempDir(), "x.bin")
	if _, err := d.File(context.Background(), srv.URL, dst, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, data) {
		t.Errorf("content mismatch")
	}
}

func TestRangeStart(t *testing.T) {
	for in, want := range map[string]int64{"bytes 100-199/200": 100, "bytes 0-0/5": 0, "": -1, "bytes */200": -1} {
		if got := rangeStart(in); got != want {
			t.Errorf("rangeStart(%q) = %d, want %d", in, got, want)
		}
	}
}
