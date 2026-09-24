// Package download saves remote files to disk. Transfers go to a ".part"
// file that is resumed after interruptions and renamed when complete.
package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MahdiGraph/radio-javan-downloader/internal/netutil"
)

// ErrUnavailable is returned when the server does not have the file.
var ErrUnavailable = errors.New("file is not available on the server")

// ErrNoFFmpeg is returned by HLS when ffmpeg is not installed.
var ErrNoFFmpeg = errors.New("this item is only available as a stream; install ffmpeg to download it")

var errStalled = errors.New("download stalled")

// Progress receives the progress of one file.
type Progress interface {
	// Start is called whenever a transfer (re)starts. total is -1 when the
	// size is unknown; offset is the number of bytes kept from an earlier
	// attempt.
	Start(total, offset int64)
	// Wrap wraps the response body to observe the bytes read.
	Wrap(r io.Reader) io.Reader
}

// Downloader fetches files over HTTP.
type Downloader struct {
	Client    *http.Client
	UserAgent string
	// Retries is how many times a failed transfer is retried.
	Retries int
	// StallTimeout aborts and retries a transfer that receives no data for
	// this long.
	StallTimeout time.Duration
}

// New returns a Downloader with sensible timeouts.
func New(userAgent string) *Downloader {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 30 * time.Second
	return &Downloader{
		Client:       &http.Client{Transport: tr},
		UserAgent:    userAgent,
		Retries:      3,
		StallTimeout: time.Minute,
	}
}

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func permanent(err error) error { return &permanentError{err} }

// File downloads url to dst and returns the size of the file. p may be nil.
func (d *Downloader) File(ctx context.Context, url, dst string, p Progress) (int64, error) {
	part := dst + ".part"
	var lastErr error
	for attempt := 0; attempt <= d.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		size, err := d.fetch(ctx, url, part, p)
		if err == nil {
			return size, os.Rename(part, dst)
		}
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if netutil.Filtered(err) {
			return 0, err
		}
		var perm *permanentError
		if errors.As(err, &perm) {
			return 0, perm.err
		}
		lastErr = err
	}
	return 0, lastErr
}

func (d *Downloader) fetch(ctx context.Context, url, part string, p Progress) (int64, error) {
	var offset int64
	if fi, err := os.Stat(part); err == nil {
		offset = fi.Size()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, permanent(err)
	}
	if d.UserAgent != "" {
		req.Header.Set("User-Agent", d.UserAgent)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch {
	case resp.StatusCode == http.StatusPartialContent && offset > 0:
		if rangeStart(resp.Header.Get("Content-Range")) != offset {
			os.Remove(part)
			return 0, errors.New("server resumed at the wrong position")
		}
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		offset = 0
		flags |= os.O_TRUNC
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0:
		os.Remove(part)
		return 0, errors.New("could not resume the partial file")
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return 0, permanent(ErrUnavailable)
	default:
		return 0, fmt.Errorf("HTTP %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/") || strings.Contains(ct, "xml") {
		return 0, permanent(fmt.Errorf("%w (server sent %s)", ErrUnavailable, ct))
	}

	total := int64(-1)
	if resp.ContentLength >= 0 {
		total = offset + resp.ContentLength
	}
	if p != nil {
		p.Start(total, offset)
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return 0, permanent(err)
	}
	var body io.Reader = resp.Body
	var stalled atomic.Bool
	if d.StallTimeout > 0 {
		timer := time.AfterFunc(d.StallTimeout, func() {
			stalled.Store(true)
			cancel()
		})
		defer timer.Stop()
		body = &watchdogReader{r: body, timer: timer, timeout: d.StallTimeout}
	}
	if p != nil {
		body = p.Wrap(body)
	}
	n, copyErr := io.Copy(f, body)
	closeErr := f.Close()
	switch {
	case copyErr != nil && stalled.Load():
		return 0, errStalled
	case copyErr != nil:
		return 0, copyErr
	case closeErr != nil:
		return 0, permanent(closeErr)
	}
	size := offset + n
	if total >= 0 && size != total {
		return 0, fmt.Errorf("incomplete download: got %d of %d bytes", size, total)
	}
	return size, nil
}

// rangeStart parses the first byte position of a Content-Range header.
func rangeStart(h string) int64 {
	h = strings.TrimPrefix(strings.TrimSpace(h), "bytes ")
	if i := strings.IndexByte(h, '-'); i > 0 {
		if n, err := strconv.ParseInt(h[:i], 10, 64); err == nil {
			return n
		}
	}
	return -1
}

// watchdogReader pushes a stall timer back on every successful read.
type watchdogReader struct {
	r       io.Reader
	timer   *time.Timer
	timeout time.Duration
}

func (w *watchdogReader) Read(b []byte) (int, error) {
	n, err := w.r.Read(b)
	if n > 0 {
		w.timer.Reset(w.timeout)
	}
	return n, err
}

// Exists reports whether url can be downloaded.
func (d *Downloader) Exists(ctx context.Context, url string) bool {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	if d.UserAgent != "" {
		req.Header.Set("User-Agent", d.UserAgent)
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	return resp.StatusCode == http.StatusOK && !strings.HasPrefix(ct, "text/") && !strings.Contains(ct, "xml")
}

// HaveFFmpeg reports whether ffmpeg is installed.
func HaveFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// HLS saves an HLS stream to dst with ffmpeg, without re-encoding, and
// returns the size of the file.
func HLS(ctx context.Context, playlist, dst string) (int64, error) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return 0, ErrNoFFmpeg
	}
	ext := filepath.Ext(dst)
	tmp := strings.TrimSuffix(dst, ext) + ".part" + ext // ffmpeg picks the format from the extension
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-i", playlist, "-c", "copy", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(string(out)))
	}
	fi, err := os.Stat(tmp)
	if err != nil {
		return 0, err
	}
	return fi.Size(), os.Rename(tmp, dst)
}
