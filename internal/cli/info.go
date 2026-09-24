package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

func (a *app) infoCmd() *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:     "info <link>...",
		Aliases: []string{"json", "meta"},
		Short:   "Print metadata as JSON",
		Long: `Print everything Radio Javan knows about a link as JSON: titles (also in
Persian), artist, album, track number, release date, duration, play and like
counts, cover art, lyrics and the direct link of every available file.

For albums, playlists, shows and artists the items are listed with the same
fields. --raw prints the API response unchanged instead.`,
		Example: `  rjdl info https://play.radiojavan.com/song/raha-oh-nana
  rjdl info https://rj.app/m/zE1g57yl > song.json
  rjdl info https://play.radiojavan.com/album/satin-ghanune-bagha | jq '.items[].title'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := withInterrupt(cmd.Context())
			defer stop()
			var out []any
			for _, in := range args {
				v, err := a.info(ctx, in, raw)
				if err != nil {
					return fmt.Errorf("%s: %w", in, err)
				}
				out = append(out, v)
			}
			var v any = out
			if len(out) == 1 {
				v = out[0]
			}
			data, err := marshalJSON(v)
			if err != nil {
				return err
			}
			_, err = a.stdout.Write(data)
			return err
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "print the unmodified API response")
	cmd.Flags().StringSliceVar(&a.opts.include, "include", nil, "for artists, also list videos and/or podcasts: videos,podcasts")
	return cmd
}

func (a *app) info(ctx context.Context, input string, raw bool) (any, error) {
	if !raw {
		return a.load(ctx, input)
	}
	if !rj.IsLink(input) {
		return nil, fmt.Errorf("not a Radio Javan link (to search, run: rjdl search %s)", input)
	}
	t, err := a.client.Resolve(ctx, input)
	if err != nil {
		return nil, err
	}
	data, err := a.client.FetchRaw(ctx, t)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (a *app) coverCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "cover <link>...",
		Aliases: []string{"art", "artwork"},
		Short:   "Download cover art",
		Long: `Download the cover art of a song, video, podcast, album, playlist, podcast
show or artist in the largest size Radio Javan has. With --all the covers of
every item of a collection are saved too, in a folder named after it.`,
		Example: `  rjdl cover https://play.radiojavan.com/song/raha-oh-nana
  rjdl cover https://play.radiojavan.com/playlist/mp3/205e3f10cd96 --all -o covers`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := withInterrupt(cmd.Context())
			defer stop()
			failed := false
			for _, in := range args {
				if err := a.covers(ctx, in, all); err != nil {
					fmt.Fprintf(a.stderr, "Error: %s: %v\n", in, explain(err))
					failed = true
				}
			}
			if failed {
				return errSilent
			}
			return nil
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&a.opts.output, "output", "o", ".", "directory to save images in")
	fs.BoolVar(&all, "all", false, "also save the cover of every item of an album, playlist, show or artist")
	fs.BoolVar(&a.opts.farsi, "farsi", false, "use Persian titles for file names when available")
	fs.BoolVarP(&a.opts.force, "force", "f", false, "overwrite images that already exist")
	return cmd
}

func (a *app) covers(ctx context.Context, input string, all bool) error {
	ent, err := a.load(ctx, input)
	if err != nil {
		return err
	}
	save := func(url, base string) error {
		p, err := a.saveImage(ctx, url, base)
		if err != nil {
			return err
		}
		fmt.Fprintln(a.stdout, displayPath(p))
		return nil
	}
	switch e := ent.(type) {
	case *rj.Media:
		return save(e.Cover.Best(), filepath.Join(a.opts.output, sanitize(a.mediaName(e))))
	case *rj.Collection:
		name := a.collectionName(e)
		if url := e.Cover.Best(); url != "" {
			if err := save(url, filepath.Join(a.opts.output, name)); err != nil {
				return err
			}
		} else if !all {
			return errors.New("Radio Javan has no cover for this; use --all for the covers of its items")
		}
		if !all {
			return nil
		}
		dir := filepath.Join(a.opts.output, name)
		seen := map[string]bool{e.Cover.Best(): true} // album tracks share one cover
		var walk func(c *rj.Collection)
		walk = func(c *rj.Collection) {
			for _, m := range c.Items {
				url := m.Cover.Best()
				if url == "" || seen[url] {
					continue
				}
				seen[url] = true
				if err := save(url, filepath.Join(dir, sanitize(a.mediaName(m)))); err != nil {
					a.warnf("%s: %v", a.mediaName(m), err)
				}
			}
			for _, ch := range c.Children {
				walk(ch)
			}
		}
		walk(e)
	}
	return nil
}
