package cli

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

const barNameWidth = 36

// ui shows download progress: progress bars on a terminal, one line per
// finished file otherwise.
type ui struct {
	a     *app
	p     *mpb.Progress
	total *mpb.Bar
	n     int
	root  string // file names are shown relative to this directory

	mu    sync.Mutex
	done  int
	lines int
}

func (a *app) newUI(n int, root string) *ui {
	u := &ui{a: a, n: n, root: root}
	if !a.tty {
		return u
	}
	u.p = mpb.New(
		mpb.WithOutput(a.stderr),
		mpb.WithWidth(40),
		mpb.WithRefreshRate(150*time.Millisecond),
		// Finished bars (our log lines) scroll up and stay on screen.
		mpb.PopCompletedMode(),
	)
	if n > 1 {
		u.total = u.p.AddBar(int64(n),
			mpb.BarPriority(math.MaxInt32),
			mpb.BarNoPop(),
			mpb.PrependDecorators(
				decor.Name("Total", decor.WC{W: barNameWidth + 1, C: decor.DindentRight}),
				decor.CountersNoUnit("%d / %d files", decor.WC{W: 20, C: decor.DindentRight}),
			),
			mpb.AppendDecorators(decor.Percentage(decor.WC{W: 5})),
		)
	}
	return u
}

// printf prints a line above the progress bars.
func (u *ui) printf(format string, args ...any) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.println(fmt.Sprintf(format, args...))
}

// println prints a line; u.mu must be held. On a terminal the line is a
// finished bar rather than a write to the container: mpb renders every bar
// before it shuts down, while writes still buffered at that point are lost.
// Bars finishing in the same refresh are popped in descending priority, so
// earlier lines get higher priorities to stay in order.
func (u *ui) println(line string) {
	if u.p == nil {
		fmt.Fprintln(u.a.stderr, line)
		return
	}
	u.lines++
	b := u.p.New(0, mpb.NopStyle(),
		mpb.BarPriority(math.MaxInt32-1-u.lines),
		mpb.BarFillerTrim(),
		mpb.PrependDecorators(decor.Name(line)),
	)
	b.SetTotal(-1, true)
}

// finished records the outcome of a job.
func (u *ui) finished(j *job, r result) {
	name := r.path
	if rel, err := filepath.Rel(u.root, r.path); err == nil && !strings.HasPrefix(rel, "..") {
		name = rel
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.done++
	prefix := ""
	if u.n > 1 {
		prefix = fmt.Sprintf("[%d/%d] ", u.done, u.n)
	}
	switch r.status {
	case statusDownloaded:
		u.println(fmt.Sprintf("%s✓ %s (%s)", prefix, name, humanBytes(r.size)))
	case statusSkipped:
		u.println(fmt.Sprintf("%s= %s (already downloaded)", prefix, name))
	case statusFailed:
		u.println(fmt.Sprintf("%s✗ %s: %v", prefix, firstNonEmpty(name, j.base), r.err))
	}
	for _, w := range r.warnings {
		u.println("  warning: " + w)
	}
	if u.total != nil {
		u.total.Increment()
	}
}

// wait stops the display once all jobs are done or abandoned.
func (u *ui) wait() {
	if u.p == nil {
		return
	}
	if u.total != nil && !u.total.Completed() {
		u.total.Abort(false)
	}
	u.p.Wait()
}

// fileBar is the progress bar of one file. It implements
// download.Progress.
type fileBar struct {
	u    *ui
	name string
	bar  *mpb.Bar
}

func (u *ui) file(name string) *fileBar { return &fileBar{u: u, name: name} }

func (f *fileBar) Start(total, offset int64) {
	if f.u.p == nil {
		return
	}
	if f.bar == nil {
		f.bar = f.u.p.AddBar(total,
			// In PopCompletedMode a finished bar is kept on screen unless it
			// also opts out of popping.
			mpb.BarNoPop(),
			mpb.BarRemoveOnComplete(),
			mpb.PrependDecorators(decor.Name(shorten(f.name, barNameWidth), decor.WC{W: barNameWidth + 1, C: decor.DindentRight})),
			mpb.AppendDecorators(
				decor.CountersKiloByte("% .1f / % .1f", decor.WC{W: 20, C: decor.DindentRight}),
				decor.Percentage(decor.WC{W: 5}),
			),
		)
	} else {
		f.bar.SetTotal(total, false)
	}
	f.bar.SetCurrent(offset)
}

func (f *fileBar) Wrap(r io.Reader) io.Reader {
	if f.bar == nil {
		return r
	}
	return f.bar.ProxyReader(r)
}

// end completes or removes the bar. Every bar must end, or the display
// never stops.
func (f *fileBar) end(err error) {
	if f.bar == nil {
		return
	}
	if err != nil {
		f.bar.Abort(true)
		return
	}
	f.bar.SetTotal(-1, true)
}
