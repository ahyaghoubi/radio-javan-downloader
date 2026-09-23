package cli

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

const interactiveHelp = `Paste one or more Radio Javan links to download them: songs, albums, videos,
podcasts, podcast shows, playlists, artists or rj.app short links. Anything
else is searched for; pick results by number to download them.

  liked, library, my-playlists   download from your account (needs rjdl login)
  help                           show this help
  0 or q                         quit`

// interactive is the prompt shown when rjdl runs without arguments.
func (a *app) interactive(ctx context.Context) error {
	in := bufio.NewReader(a.stdin)
	out, _ := filepath.Abs(a.opts.output)
	fmt.Fprintf(a.stdout, "Radio Javan Downloader %s\n", version())
	fmt.Fprintln(a.stdout, "Paste a link to download it, or type something to search. Type help for help, 0 to quit.")
	fmt.Fprintf(a.stdout, "Files are saved in %s\n", out)
	for {
		fmt.Fprint(a.stdout, "\n> ")
		line, err := in.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil && line == "" {
			fmt.Fprintln(a.stdout)
			return nil
		}
		switch strings.ToLower(line) {
		case "":
			continue
		case "0", "q", "quit", "exit":
			return nil
		case "help", "?", "h":
			fmt.Fprintln(a.stdout, interactiveHelp)
			continue
		}
		if fields := strings.Fields(line); allLinks(fields) {
			a.interactiveDownload(ctx, fields)
			continue
		}
		a.interactiveSearch(ctx, in, line)
	}
}

func allLinks(fields []string) bool {
	for _, f := range fields {
		if !rj.IsLink(f) {
			return false
		}
	}
	return len(fields) > 0
}

func (a *app) interactiveDownload(ctx context.Context, inputs []string) {
	// runDownload reports its own errors.
	_ = a.runDownload(ctx, inputs)
}

func (a *app) interactiveSearch(ctx context.Context, in *bufio.Reader, query string) {
	sctx, stop := withInterrupt(ctx)
	res, err := a.client.Search(sctx, query)
	stop()
	if err != nil {
		fmt.Fprintln(a.stderr, "Error:", explain(err))
		return
	}
	results := printSections(a.stdout, filterSections(sections(res), "", 5))
	if len(results) == 0 {
		fmt.Fprintln(a.stdout, "No results.")
		return
	}
	for {
		fmt.Fprint(a.stdout, "\nDownload which? (numbers like 1 3-5, Enter to skip): ")
		line, _ := in.ReadString('\n')
		switch line = strings.TrimSpace(line); strings.ToLower(line) {
		case "", "0", "q":
			return
		}
		picked, err := parseSelection(line, len(results))
		if err != nil {
			fmt.Fprintln(a.stderr, "Error:", err)
			continue
		}
		links := make([]string, len(picked))
		for i, n := range picked {
			links[i] = results[n-1].URL
		}
		a.interactiveDownload(ctx, links)
		return
	}
}
