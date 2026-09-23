package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

type section struct {
	key   string
	title string
	items []rj.SearchResult
}

func sections(res *rj.SearchResults) []section {
	return []section{
		{"artists", "Artists", res.Artists},
		{"songs", "Songs", res.Songs},
		{"albums", "Albums", res.Albums},
		{"videos", "Videos", res.Videos},
		{"podcasts", "Podcasts", res.Podcasts},
		{"shows", "Podcast shows", res.Shows},
		{"playlists", "Playlists", res.Playlists},
	}
}

// searchTypes maps accepted --type values to section keys.
var searchTypes = map[string]string{
	"all": "", "artist": "artists", "artists": "artists", "song": "songs", "songs": "songs",
	"mp3": "songs", "album": "albums", "albums": "albums", "video": "videos", "videos": "videos",
	"podcast": "podcasts", "podcasts": "podcasts", "show": "shows", "shows": "shows",
	"playlist": "playlists", "playlists": "playlists",
}

// filterSections keeps the sections of one type (all when typ is empty)
// and at most limit results in each.
func filterSections(all []section, typ string, limit int) []section {
	var out []section
	for _, s := range all {
		if typ != "" && s.key != typ {
			continue
		}
		if limit > 0 && len(s.items) > limit {
			s.items = s.items[:limit]
		}
		if len(s.items) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// printSections prints numbered results and returns them in that order.
func printSections(w io.Writer, secs []section) []rj.SearchResult {
	var flat []rj.SearchResult
	for i, s := range secs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, s.title)
		for _, r := range s.items {
			flat = append(flat, r)
			title := r.Title
			if r.Kind != rj.KindArtist && r.Artist != "" {
				title = r.Artist + " - " + r.Title
			}
			fmt.Fprintf(w, "%4d. %s\n      %s\n", len(flat), title, r.URL)
		}
	}
	return flat
}

func (a *app) searchCmd() *cobra.Command {
	var typ string
	var limit int
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"s", "find"},
		Short:   "Search Radio Javan",
		Long: `Search songs, albums, videos, podcasts, podcast shows, playlists and artists.
Every result has a link that can be passed to rjdl, rjdl info or rjdl cover.`,
		Example: `  rjdl search ebi
  rjdl search "shadmehr aghili" --type songs --limit 30
  rjdl search "dance station" --type podcasts --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, ok := searchTypes[strings.ToLower(typ)]
			if !ok {
				return fmt.Errorf("--type must be one of all, songs, albums, videos, podcasts, shows, playlists or artists")
			}
			if limit <= 0 {
				limit = 5
				if key != "" {
					limit = 20
				}
			}
			ctx, stop := withInterrupt(cmd.Context())
			defer stop()
			res, err := a.client.Search(ctx, strings.Join(args, " "))
			if err != nil {
				return err
			}
			secs := filterSections(sections(res), key, limit)
			if asJSON {
				out := map[string]any{"query": res.Query}
				for _, s := range secs {
					out[s.key] = s.items
				}
				data, err := marshalJSON(out)
				if err != nil {
					return err
				}
				_, err = a.stdout.Write(data)
				return err
			}
			if len(secs) == 0 {
				fmt.Fprintln(a.stdout, "No results.")
				return nil
			}
			printSections(a.stdout, secs)
			return nil
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&typ, "type", "t", "all", "only show one type: songs, albums, videos, podcasts, shows, playlists or artists")
	fs.IntVarP(&limit, "limit", "n", 0, "number of results per type (default 5, or 20 with --type)")
	fs.BoolVar(&asJSON, "json", false, "print results as JSON")
	return cmd
}
