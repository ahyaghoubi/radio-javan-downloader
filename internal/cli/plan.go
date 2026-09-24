package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

// job is one media file to download.
type job struct {
	media *rj.Media
	dir   string
	base  string // file name without extension
	index int    // 1-based position in the plan
	// albumTrack is set for album tracks, whose cover art is saved once
	// for the album folder instead of next to every track.
	albumTrack bool
}

// folder is a collection that gets its own cover and metadata files.
type folder struct {
	col  *rj.Collection
	dir  string
	name string
	own  bool // dir belongs to the collection (not --flat)
}

type plan struct {
	jobs    []*job
	folders []*folder
}

// load resolves input and fetches what it points to.
func (a *app) load(ctx context.Context, input string) (rj.Entity, error) {
	if !rj.IsLink(input) {
		return nil, fmt.Errorf("not a Radio Javan link (to search, run: rjdl search %s)", input)
	}
	t, err := a.client.Resolve(ctx, input)
	if err != nil {
		return nil, err
	}
	a.statusf("Fetching %s…", t)
	ent, err := a.client.Fetch(ctx, t, a.fetchOptions())
	if err != nil {
		return nil, err
	}
	if c, ok := ent.(*rj.Collection); ok {
		a.printWarnings(c)
	}
	return ent, nil
}

func (a *app) printWarnings(c *rj.Collection) {
	for _, w := range c.Warnings {
		a.warnf("%s", w)
	}
	for _, ch := range c.Children {
		a.printWarnings(ch)
	}
}

// describe summarises an entity for the user.
func describe(ent rj.Entity) string {
	switch e := ent.(type) {
	case *rj.Media:
		return fmt.Sprintf("%s: %s", kindLabel(e.Kind), joinArtistTitle(e.Artist, e.Title))
	case *rj.Collection:
		name := e.Title
		if e.Kind == rj.KindAlbum {
			name = joinArtistTitle(e.Artist, e.Title)
			if e.Year != "" {
				name += " (" + e.Year + ")"
			}
		}
		unit := "item"
		switch e.Kind {
		case rj.KindAlbum:
			unit = "track"
		case rj.KindShow:
			unit = "episode"
		case rj.KindLiked:
			unit = "song"
		case rj.KindArtist:
			unit = "song"
			for _, ch := range e.Children {
				if ch.Kind == rj.KindGroup { // videos or podcasts
					unit = "item"
				}
			}
		}
		n := e.Len()
		if n != 1 {
			unit += "s"
		}
		return fmt.Sprintf("%s: %s, %d %s", kindLabel(e.Kind), name, n, unit)
	}
	return ""
}

var kindLabels = map[rj.Kind]string{
	rj.KindSong:        "Song",
	rj.KindVideo:       "Video",
	rj.KindPodcast:     "Podcast",
	rj.KindAlbum:       "Album",
	rj.KindPlaylist:    "Playlist",
	rj.KindArtist:      "Artist",
	rj.KindShow:        "Podcast show",
	rj.KindLiked:       "Liked songs",
	rj.KindLibrary:     "Library",
	rj.KindMyPlaylists: "Your playlists",
}

func kindLabel(k rj.Kind) string {
	if l, ok := kindLabels[k]; ok {
		return l
	}
	return string(k)
}

func joinArtistTitle(artist, title string) string {
	switch {
	case artist == "":
		return title
	case title == "":
		return artist
	}
	return artist + " - " + title
}

// plan decides where every file of ent goes.
func (a *app) plan(ent rj.Entity) *plan {
	p := &plan{}
	pl := planner{a: a, p: p, names: map[string]bool{}, seen: map[string]bool{}}
	switch e := ent.(type) {
	case *rj.Media:
		pl.add(e, a.opts.output, 0, false)
	case *rj.Collection:
		pl.collection(e, a.opts.output)
	}
	for i, j := range p.jobs {
		j.index = i + 1
	}
	return p
}

type planner struct {
	a     *app
	p     *plan
	names map[string]bool // file paths in use, lower case
	seen  map[string]bool // media already planned per folder
}

func (pl *planner) collection(c *rj.Collection, parent string) {
	name := pl.a.collectionName(c)
	dir := parent
	if !pl.a.opts.flat {
		dir = filepath.Join(parent, name)
	}
	pl.p.folders = append(pl.p.folders, &folder{col: c, dir: dir, name: name, own: !pl.a.opts.flat})
	for i, m := range c.Items {
		track := 0
		if c.Kind == rj.KindAlbum {
			track = i + 1
			if m.Album != nil && m.Album.Track > 0 {
				track = m.Album.Track
			}
		}
		pl.add(m, dir, track, c.Kind == rj.KindAlbum)
	}
	for _, ch := range c.Children {
		pl.collection(ch, dir)
	}
}

func (pl *planner) add(m *rj.Media, dir string, track int, albumTrack bool) {
	key := strings.ToLower(dir) + "\x00" + string(m.Kind) + "\x00" + m.ID
	if pl.seen[key] {
		return
	}
	pl.seen[key] = true
	name := pl.a.mediaName(m)
	if track > 0 {
		name = fmt.Sprintf("%02d. %s", track, name)
	}
	base := sanitize(name)
	if pl.names[strings.ToLower(filepath.Join(dir, base))] {
		base = sanitize(fmt.Sprintf("%s (%s)", name, m.ID))
	}
	pl.names[strings.ToLower(filepath.Join(dir, base))] = true
	pl.p.jobs = append(pl.p.jobs, &job{media: m, dir: dir, base: base, albumTrack: albumTrack})
}

// mediaName is "Artist - Title", in Persian with --farsi when available.
func (a *app) mediaName(m *rj.Media) string {
	title, artist := m.Title, m.Artist
	if a.opts.farsi {
		title = firstNonEmpty(m.TitleFa, title)
		artist = firstNonEmpty(m.ArtistFa, artist)
	}
	return firstNonEmpty(joinArtistTitle(artist, title), m.Permlink, m.ID)
}

func (a *app) collectionName(c *rj.Collection) string {
	title, artist := c.Title, c.Artist
	if a.opts.farsi {
		title = firstNonEmpty(c.TitleFa, title)
		if c.Kind == rj.KindAlbum && len(c.Items) > 0 {
			artist = firstNonEmpty(c.Items[0].ArtistFa, artist)
		}
	}
	name := title
	if c.Kind == rj.KindAlbum {
		name = joinArtistTitle(artist, title)
		if c.Year != "" {
			name += " (" + c.Year + ")"
		}
	}
	return sanitize(firstNonEmpty(name, string(c.Kind)+" "+c.ID))
}

// selectItems keeps only the jobs at the given 1-based positions.
func (p *plan) selectItems(spec string) error {
	if spec == "" {
		return nil
	}
	if len(p.jobs) == 0 {
		return errors.New("--items: there is nothing to select from")
	}
	picked, err := parseSelection(spec, len(p.jobs))
	if err != nil {
		return fmt.Errorf("--items: %w", err)
	}
	jobs := make([]*job, 0, len(picked))
	for n, i := range picked {
		j := p.jobs[i-1]
		j.index = n + 1
		jobs = append(jobs, j)
	}
	p.jobs = jobs
	// Drop folders that no longer receive any file.
	folders := p.folders[:0]
	for _, f := range p.folders {
		for _, j := range jobs {
			if j.dir == f.dir || strings.HasPrefix(j.dir, f.dir+string(os.PathSeparator)) {
				folders = append(folders, f)
				break
			}
		}
	}
	p.folders = folders
	return nil
}
