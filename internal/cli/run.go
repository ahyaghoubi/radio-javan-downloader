package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MahdiGraph/radio-javan-downloader/internal/download"
	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

const (
	statusDownloaded = "downloaded"
	statusSkipped    = "skipped"
	statusFailed     = "failed"
)

type result struct {
	status   string
	path     string
	size     int64
	err      error
	warnings []string
}

// runDownload downloads every input and reports failures at the end.
func (a *app) runDownload(ctx context.Context, inputs []string) error {
	ctx, stop := withInterrupt(ctx)
	defer stop()
	failed := 0
	for i, in := range inputs {
		if i > 0 {
			fmt.Fprintln(a.stderr)
		}
		err := a.download(ctx, in)
		if ctx.Err() != nil {
			a.statusf("\nInterrupted. Run the same command again to resume.")
			return errSilent
		}
		if err != nil {
			failed++
			if !errors.Is(err, errSilent) {
				fmt.Fprintf(a.stderr, "Error: %s: %v\n", in, explain(err))
			}
		}
	}
	if failed > 0 {
		return errSilent
	}
	return nil
}

func (a *app) download(ctx context.Context, input string) error {
	ent, err := a.load(ctx, input)
	if err != nil {
		return err
	}
	p := a.plan(ent)
	if err := p.selectItems(a.opts.items); err != nil {
		return err
	}
	if a.opts.links {
		return a.printLinks(ctx, p)
	}
	if a.opts.dryRun {
		a.printPlan(ent, p)
		return nil
	}
	a.statusf("%s", describe(ent))
	if len(p.jobs) == 0 {
		a.statusf("Nothing to download.")
		return nil
	}
	if len(p.folders) > 0 {
		a.statusf("Saving to %s", displayPath(p.folders[0].dir))
	}
	return a.execute(ctx, p)
}

// execute downloads the jobs of p with a pool of workers.
func (a *app) execute(ctx context.Context, p *plan) error {
	root := a.opts.output
	if len(p.folders) > 0 {
		root = p.folders[0].dir
	}
	u := a.newUI(len(p.jobs), root)
	for _, f := range p.folders {
		for _, w := range a.saveFolderExtras(ctx, f) {
			u.printf("warning: %s", w)
		}
	}
	results := make([]result, len(p.jobs))
	queue := make(chan *job)
	var wg sync.WaitGroup
	for w := 0; w < min(a.opts.jobs, len(p.jobs)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range queue {
				r := a.runJob(ctx, u, j)
				results[j.index-1] = r
				if ctx.Err() == nil {
					u.finished(j, r)
				}
			}
		}()
	}
feed:
	for _, j := range p.jobs {
		select {
		case queue <- j:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()
	u.wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var downloaded, skipped int
	var bytes int64
	var failed []string
	for i, r := range results {
		switch r.status {
		case statusDownloaded:
			downloaded++
			bytes += r.size
		case statusSkipped:
			skipped++
		case statusFailed:
			failed = append(failed, fmt.Sprintf("  %s: %v", p.jobs[i].base, r.err))
		}
	}
	if len(p.jobs) > 1 {
		a.statusf("Finished: %d downloaded (%s), %d already there, %d failed.", downloaded, humanBytes(bytes), skipped, len(failed))
	}
	if len(failed) > 0 {
		// Progress lines are cut to the terminal width; repeat errors in full.
		a.statusf("Failed:\n%s", strings.Join(failed, "\n"))
		return errSilent
	}
	return nil
}

// runJob downloads one media file and its extras.
func (a *app) runJob(ctx context.Context, u *ui, j *job) result {
	m := j.media
	base := filepath.Join(j.dir, j.base)
	c, existing, err := a.choose(ctx, m, base)
	if err != nil {
		return result{status: statusFailed, err: err}
	}
	var r result
	if existing != "" {
		r = result{status: statusSkipped, path: existing}
		if fi, err := os.Stat(existing); err == nil {
			r.size = fi.Size()
		}
	} else {
		r.path = base + "." + fileExt(m, c.Format)
		if err := os.MkdirAll(j.dir, 0o755); err != nil {
			return result{status: statusFailed, path: r.path, err: err}
		}
		var size int64
		if c.IsHLS() {
			size, err = download.HLS(ctx, c.URL, r.path)
		} else {
			bar := u.file(filepath.Base(r.path))
			size, err = a.dl.File(ctx, c.URL, r.path, bar)
			bar.end(err)
		}
		if err != nil {
			return result{status: statusFailed, path: r.path, err: explain(err)}
		}
		r.status, r.size = statusDownloaded, size
	}
	r.warnings = a.saveMediaExtras(ctx, j, base)
	return r
}

// choose picks the file to download for m following the user's
// preference. When one of the acceptable files already exists, its path
// is returned instead (unless --force).
func (a *app) choose(ctx context.Context, m *rj.Media, base string) (rj.Candidate, string, error) {
	cands := m.Candidates(a.preference())
	if len(cands) == 0 {
		if m.PremiumOnly {
			return rj.Candidate{}, "", rj.ErrPremiumRequired
		}
		return rj.Candidate{}, "", errors.New("Radio Javan offers no file for this item")
	}
	needFFmpeg := false
	for _, c := range cands {
		if base != "" && !a.opts.force {
			if p := base + "." + fileExt(m, c.Format); fileExists(p) {
				return c, p, nil
			}
		}
		if c.IsHLS() && !download.HaveFFmpeg() {
			needFFmpeg = true
			continue
		}
		if c.Derived && !a.dl.Exists(ctx, c.URL) {
			continue
		}
		return c, "", nil
	}
	if needFFmpeg {
		return rj.Candidate{}, "", download.ErrNoFFmpeg
	}
	return rj.Candidate{}, "", download.ErrUnavailable
}

// fileExt is the extension a format is saved with.
func fileExt(m *rj.Media, f rj.Format) string {
	switch {
	case f.IsHLS() && m.Kind == rj.KindVideo:
		return "mp4"
	case f.IsHLS():
		return "m4a"
	case f.Ext == "":
		return "bin"
	}
	return f.Ext
}

// printPlan lists the files a download would create, numbered for --items.
// The extension is the preferred one; the file actually saved can differ
// when that format is missing.
func (a *app) printPlan(ent rj.Entity, p *plan) {
	a.statusf("%s", describe(ent))
	root := a.opts.output
	if len(p.folders) > 0 {
		root = p.folders[0].dir
		fmt.Fprintf(a.stdout, "%s%c\n", displayPath(root), filepath.Separator)
	}
	for _, j := range p.jobs {
		name := filepath.Join(j.dir, j.base)
		if rel, err := filepath.Rel(root, name); err == nil {
			name = rel
		}
		if cands := j.media.Candidates(a.preference()); len(cands) > 0 {
			name += "." + fileExt(j.media, cands[0].Format)
		}
		fmt.Fprintf(a.stdout, "%4d. %s\n", j.index, name)
	}
}

// printLinks prints the direct link of every job instead of downloading.
func (a *app) printLinks(ctx context.Context, p *plan) error {
	failed := 0
	for _, j := range p.jobs {
		c, _, err := a.choose(ctx, j.media, "")
		if err != nil {
			a.warnf("%s: %v", j.base, err)
			failed++
			continue
		}
		fmt.Fprintln(a.stdout, c.URL)
	}
	if failed > 0 {
		return errSilent
	}
	return nil
}

// saveMediaExtras writes the cover, metadata and lyrics files of a job
// next to its media file. Problems are returned as warnings.
func (a *app) saveMediaExtras(ctx context.Context, j *job, base string) []string {
	var warnings []string
	m := j.media
	if a.opts.lyrics && m.Kind == rj.KindSong {
		if !m.HasLyrics() {
			// Lists do not include lyrics; the song itself does.
			if full, err := a.client.Media(ctx, rj.KindSong, firstNonEmpty(m.Permlink, m.ID)); err == nil {
				m.Lyrics, m.SyncedLyrics = full.Lyrics, full.SyncedLyrics
			}
		}
		if err := a.saveLyrics(m, base); err != nil {
			warnings = append(warnings, "lyrics: "+err.Error())
		}
	}
	if a.opts.writeJSON {
		if err := a.saveJSON(base+".json", m); err != nil {
			warnings = append(warnings, "metadata: "+err.Error())
		}
	}
	if a.opts.cover && !j.albumTrack {
		if _, err := a.saveImage(ctx, m.Cover.Best(), base); err != nil {
			warnings = append(warnings, "cover: "+err.Error())
		}
	}
	return warnings
}

// saveFolderExtras writes the cover and metadata of a collection.
func (a *app) saveFolderExtras(ctx context.Context, f *folder) []string {
	if !a.opts.cover && !a.opts.writeJSON {
		return nil
	}
	var warnings []string
	coverBase, jsonPath := filepath.Join(f.dir, "cover"), filepath.Join(f.dir, "info.json")
	if !f.own {
		coverBase, jsonPath = filepath.Join(f.dir, f.name), filepath.Join(f.dir, f.name+".json")
	}
	if a.opts.cover && f.col.Cover.Best() != "" {
		if _, err := a.saveImage(ctx, f.col.Cover.Best(), coverBase); err != nil {
			warnings = append(warnings, fmt.Sprintf("cover of %s: %v", f.name, err))
		}
	}
	if a.opts.writeJSON {
		// Sub-collections get files of their own.
		shallow := *f.col
		shallow.Children = nil
		if err := a.saveJSON(jsonPath, &shallow); err != nil {
			warnings = append(warnings, fmt.Sprintf("metadata of %s: %v", f.name, err))
		}
	}
	return warnings
}

func (a *app) saveJSON(path string, v any) error {
	if !a.opts.force && fileExists(path) {
		return nil
	}
	data, err := marshalJSON(v)
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

func (a *app) saveLyrics(m *rj.Media, base string) error {
	switch {
	case len(m.SyncedLyrics) > 0:
		if p := base + ".lrc"; a.opts.force || !fileExists(p) {
			return writeFile(p, []byte(lrc(m)))
		}
	case m.Lyrics != "":
		if p := base + ".txt"; a.opts.force || !fileExists(p) {
			return writeFile(p, []byte(m.Lyrics+"\n"))
		}
	}
	return nil
}

// saveImage downloads an image to base plus the image's extension and
// returns the path. An existing file is kept unless --force is set.
func (a *app) saveImage(ctx context.Context, url, base string) (string, error) {
	if url == "" {
		return "", errors.New("Radio Javan has no image for this item")
	}
	ext := strings.ToLower(path.Ext(strings.SplitN(url, "?", 2)[0]))
	switch ext {
	case ".jpeg", "":
		ext = ".jpg"
	}
	dst := base + ext
	if !a.opts.force && fileExists(dst) {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	_, err := a.dl.File(ctx, url, dst, nil)
	return dst, err
}
