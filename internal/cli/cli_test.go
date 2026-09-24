package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		`Raha - "Oh Nana"`:       `Raha - 'Oh Nana'`,
		"AC/DC: Back?":           "AC-DC - Back",
		"  lots   of\tspace  ":   "lots of space",
		"...":                    "untitled",
		"CON":                    "_CON",
		"com1.txt":               "_com1.txt",
		"قانون بقا":              "قانون بقا",
		"a\x00b\x1fc":            "abc",
		"trailing dot.":          "trailing dot",
		strings.Repeat("ش", 200): strings.Repeat("ش", 90),
	} {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSelection(t *testing.T) {
	got, err := parseSelection("3, 1-2 5 2-3 8-", 9)
	if err != nil || !reflect.DeepEqual(got, []int{1, 2, 3, 5, 8, 9}) {
		t.Errorf("got %v, %v", got, err)
	}
	for _, bad := range []string{"0", "10", "a", "3-1", "", "-2"} {
		if _, err := parseSelection(bad, 9); err == nil {
			t.Errorf("parseSelection(%q) accepted", bad)
		}
	}
}

func TestLRC(t *testing.T) {
	m := &rj.Media{Title: "T", Artist: "A", Duration: 182.5, Album: &rj.AlbumRef{Title: "Al"},
		SyncedLyrics: []rj.LyricLine{{Time: 14.06, Text: "one "}, {Time: 75.5, Text: "two"}}}
	want := "[ti:T]\n[ar:A]\n[al:Al]\n[length:03:02.50]\n[00:14.06]one\n[01:15.50]two\n"
	if got := lrc(m); got != want {
		t.Errorf("lrc:\n%s\nwant:\n%s", got, want)
	}
}

func TestHumanBytes(t *testing.T) {
	for n, want := range map[int64]string{999: "999 B", 1500: "1.5 kB", 6058431: "6.1 MB", 3 << 30: "3.2 GB"} {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %s, want %s", n, got, want)
		}
	}
}

func TestPlanLayout(t *testing.T) {
	a := &app{opts: options{output: "out"}}
	track := func(id, title string, n int) *rj.Media {
		return &rj.Media{Kind: rj.KindSong, ID: id, Title: title, Artist: "Ebi", Album: &rj.AlbumRef{Title: "Koohe Yakh", Track: n}}
	}
	artist := &rj.Collection{Kind: rj.KindArtist, Title: "Ebi",
		Items: []*rj.Media{{Kind: rj.KindSong, ID: "1", Title: "Single", Artist: "Ebi"}, {Kind: rj.KindSong, ID: "1", Title: "Single", Artist: "Ebi"}},
		Children: []*rj.Collection{
			{Kind: rj.KindAlbum, Title: "Koohe Yakh", Artist: "Ebi", Year: "1995", Items: []*rj.Media{track("2", "Gheseh Eshgh", 1), track("3", "Koohe Yakh", 2)}},
			{Kind: rj.KindGroup, Title: "Videos", Items: []*rj.Media{{Kind: rj.KindVideo, ID: "4", Title: "Single", Artist: "Ebi"}}},
		},
	}
	p := a.plan(artist)
	var got []string
	for _, j := range p.jobs {
		got = append(got, filepath.ToSlash(filepath.Join(j.dir, j.base)))
	}
	want := []string{
		"out/Ebi/Ebi - Single", // the duplicate entry is dropped
		"out/Ebi/Ebi - Koohe Yakh (1995)/01. Ebi - Gheseh Eshgh",
		"out/Ebi/Ebi - Koohe Yakh (1995)/02. Ebi - Koohe Yakh",
		"out/Ebi/Videos/Ebi - Single",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !p.jobs[1].albumTrack || p.jobs[0].albumTrack {
		t.Errorf("album tracks not marked")
	}

	if err := p.selectItems("2-3"); err != nil {
		t.Fatal(err)
	}
	if len(p.jobs) != 2 || p.jobs[0].index != 1 || len(p.folders) != 2 {
		t.Errorf("after --items: %d jobs (first index %d), %d folders", len(p.jobs), p.jobs[0].index, len(p.folders))
	}

	a.opts.flat, a.opts.farsi = true, true
	song := &rj.Media{Kind: rj.KindSong, ID: "9", Title: "Oh Nana", Artist: "Raha", TitleFa: "اووه نه نه نه", ArtistFa: "رها"}
	p = a.plan(&rj.Collection{Kind: rj.KindPlaylist, Title: "Mix", Items: []*rj.Media{song}})
	if j := p.jobs[0]; j.dir != "out" || j.base != "رها - اووه نه نه نه" {
		t.Errorf("flat/farsi job: dir %q base %q", j.dir, j.base)
	}
}
