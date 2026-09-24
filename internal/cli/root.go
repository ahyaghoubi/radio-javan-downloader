// Package cli implements the rjdl command line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/MahdiGraph/radio-javan-downloader/internal/download"
	"github.com/MahdiGraph/radio-javan-downloader/internal/netutil"
	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

// Version is set at build time with
// -ldflags "-X github.com/MahdiGraph/radio-javan-downloader/internal/cli.Version=v2.0.0".
var Version string

func version() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

// errSilent reports a failure whose details were already printed.
var errSilent = errors.New("failed")

type options struct {
	output       string
	audioFormat  string
	videoQuality string
	jobs         int
	cover        bool
	writeJSON    bool
	lyrics       bool
	farsi        bool
	flat         bool
	items        string
	include      []string
	links        bool
	dryRun       bool
	force        bool
}

type app struct {
	opts   options
	proxy  string
	client *rj.Client
	dl     *download.Downloader
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	// tty is set when stderr is a terminal; progress bars are shown then.
	tty bool
}

// Execute runs the command line and returns the process exit code.
func Execute() int {
	a := &app{
		client: rj.NewClient(),
		dl:     download.New(rj.BrowserUserAgent),
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		tty:    term.IsTerminal(int(os.Stderr.Fd())),
	}
	if err := a.rootCmd().Execute(); err != nil {
		if !errors.Is(err, errSilent) {
			fmt.Fprintln(a.stderr, "Error:", explain(err))
		}
		return 1
	}
	return 0
}

func (a *app) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "rjdl [link...]",
		Short: "Download songs, albums, videos, podcasts and playlists from Radio Javan",
		Long: `rjdl downloads from Radio Javan (play.radiojavan.com) without an account:
songs, albums, music videos, podcast episodes and shows, playlists and whole
artist catalogues. It can also search, print metadata as JSON and save cover
art. Logging in is only needed for your liked songs, library and playlists,
and for RJ Premium content.

Run it without arguments for an interactive prompt that accepts links and
search queries.`,
		Example: `  rjdl https://play.radiojavan.com/song/raha-oh-nana
  rjdl https://rj.app/m/zE1g57yl --cover --lyrics
  rjdl https://play.radiojavan.com/album/satin-ghanune-bagha -o ~/Music
  rjdl https://play.radiojavan.com/artist/ebi --include videos
  rjdl search "siavash ghomayshi" --type albums
  rjdl info https://play.radiojavan.com/podcast/dance-station-41
  rjdl cover https://play.radiojavan.com/album/satin-ghanune-bagha`,
		Args:              cobra.ArbitraryArgs,
		Version:           version(),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return a.setup() },
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if f, ok := a.stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
					return a.interactive(cmd.Context())
				}
				return cmd.Help()
			}
			return a.runDownload(cmd.Context(), args)
		},
	}
	root.SetContext(context.Background())
	root.PersistentFlags().StringVar(&a.proxy, "proxy", "", "send all traffic through this proxy, e.g. socks5://127.0.0.1:1080 or http://127.0.0.1:8080\n(HTTPS_PROXY and HTTP_PROXY are used when not set)")
	a.addDownloadFlags(root.Flags())
	root.AddCommand(
		a.downloadCmd(),
		a.infoCmd(),
		a.searchCmd(),
		a.coverCmd(),
		a.loginCmd(),
		a.logoutCmd(),
		a.whoamiCmd(),
	)
	return root
}

func (a *app) downloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "download <link>...",
		Aliases: []string{"dl", "get"},
		Short:   "Download songs, albums, videos, podcasts, shows, playlists or artists",
		Long: `Download everything a link points to. Accepted links:

  song, album, video and podcast pages   play.radiojavan.com/song/…, /album/…, /video/…, /podcast/…
  podcast shows                          play.radiojavan.com/podcast/show/…
  playlists (audio and video)            play.radiojavan.com/playlist/mp3/…, /playlist/video/…
  artists (songs and albums)             play.radiojavan.com/artist/…
  short links                            rj.app/…
  old radiojavan.com links               radiojavan.com/mp3s/mp3/…, /videos/video/…, …
  your account (needs rjdl login)        liked, library, my-playlists

Albums, playlists, shows and artists are saved in a folder of their own.
Existing files are skipped and interrupted downloads resume, so the same
command can be run again to finish or update a download.`,
		Example: `  rjdl download https://play.radiojavan.com/album/ebi-koohe-yakh --cover
  rjdl download https://play.radiojavan.com/video/amir-maghare-man-delam-tange --video-quality 1080
  rjdl download https://play.radiojavan.com/playlist/mp3/205e3f10cd96 --items 1-10
  rjdl download liked -o ~/Music/RJ`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return a.runDownload(cmd.Context(), args) },
	}
	a.addDownloadFlags(cmd.Flags())
	return cmd
}

func (a *app) addDownloadFlags(fs *pflag.FlagSet) {
	o := &a.opts
	fs.StringVarP(&o.output, "output", "o", ".", "directory to save files in")
	fs.StringVar(&o.audioFormat, "audio-format", "mp3", "audio file to get: mp3 (256k, tagged, with cover), m4a (AAC 256k) or m4a-low (AAC 128k)")
	fs.StringVar(&o.videoQuality, "video-quality", "best", "video resolution: best, 2160, 1080, 720 or 480")
	fs.IntVarP(&o.jobs, "jobs", "j", 3, "number of files to download at the same time (1-10)")
	fs.BoolVar(&o.cover, "cover", false, "also save cover art as .jpg files")
	fs.BoolVar(&o.writeJSON, "write-json", false, "also save metadata as .json files")
	fs.BoolVar(&o.lyrics, "lyrics", false, "also save lyrics of songs (.lrc when time-synced, .txt otherwise)")
	fs.BoolVar(&o.farsi, "farsi", false, "use Persian titles for file and folder names when available")
	fs.BoolVar(&o.flat, "flat", false, "save everything in the output directory, without album/playlist folders")
	fs.StringVar(&o.items, "items", "", "download only these items of a collection, e.g. 1-5,8")
	fs.StringSliceVar(&o.include, "include", nil, "also download an artist's videos and/or podcasts: videos,podcasts")
	fs.BoolVar(&o.links, "links", false, "print direct download links instead of downloading")
	fs.BoolVar(&o.dryRun, "dry-run", false, "list the numbered files that would be downloaded (see --items) and stop")
	fs.BoolVarP(&o.force, "force", "f", false, "download again and overwrite files that already exist")
}

// setup validates options and loads the saved session.
func (a *app) setup() error {
	o := &a.opts
	switch o.audioFormat {
	case "mp3", "m4a", "m4a-low":
	default:
		return fmt.Errorf("--audio-format must be mp3, m4a or m4a-low, not %q", o.audioFormat)
	}
	switch strings.ToLower(strings.TrimSuffix(o.videoQuality, "p")) {
	case "best", "4k", "2160", "1080", "720", "480":
	default:
		return fmt.Errorf("--video-quality must be best, 2160, 1080, 720 or 480, not %q", o.videoQuality)
	}
	if o.jobs < 1 || o.jobs > 10 {
		return fmt.Errorf("--jobs must be between 1 and 10")
	}
	for _, v := range o.include {
		if v != "videos" && v != "podcasts" {
			return fmt.Errorf("--include accepts videos and podcasts, not %q", v)
		}
	}
	if err := a.setupNetwork(); err != nil {
		return err
	}
	s, err := loadSession()
	if err != nil {
		a.warnf("could not read the saved session: %v", err)
	}
	if s != nil {
		a.client.Cookie = s.Cookie
	}
	return nil
}

// setupNetwork gives the API client and the downloader one transport that
// honours --proxy, or the standard proxy environment variables.
func (a *app) setupNetwork() error {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	tr.ResponseHeaderTimeout = 30 * time.Second
	if a.proxy != "" {
		u, err := url.Parse(a.proxy)
		if err != nil || u.Host == "" {
			return fmt.Errorf("--proxy %q is not a valid URL like socks5://127.0.0.1:1080", a.proxy)
		}
		switch u.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("--proxy must be an http, https, socks5 or socks5h URL, not %q", a.proxy)
		}
		tr.Proxy = http.ProxyURL(u)
	}
	a.client.HTTP.Transport = tr
	a.dl.Client.Transport = tr
	return nil
}

// explain adds advice to errors that users can act on.
func explain(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return errors.New("interrupted")
	case netutil.Filtered(err):
		return fmt.Errorf("%w (your network blocks Radio Javan: use a VPN or --proxy)", err)
	}
	return err
}

func (a *app) preference() rj.Preference {
	return rj.Preference{Audio: a.opts.audioFormat, Video: strings.TrimSuffix(a.opts.videoQuality, "p")}
}

func (a *app) fetchOptions() rj.FetchOptions {
	var opt rj.FetchOptions
	for _, v := range a.opts.include {
		switch v {
		case "videos":
			opt.Artist.Videos = true
		case "podcasts":
			opt.Artist.Podcasts = true
		}
	}
	return opt
}

// withInterrupt returns a context that is cancelled by Ctrl+C.
func withInterrupt(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

func (a *app) statusf(format string, args ...any) {
	fmt.Fprintf(a.stderr, format+"\n", args...)
}

func (a *app) warnf(format string, args ...any) {
	fmt.Fprintf(a.stderr, "Warning: "+format+"\n", args...)
}
