package rj

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Target identifies something that can be fetched from Radio Javan.
type Target struct {
	Kind    Kind   `json:"type"`
	ID      string `json:"id,omitempty"`
	Subtype string `json:"subtype,omitempty"` // playlists: mp3 or video
}

func (t Target) String() string {
	switch {
	case t.ID == "":
		return string(t.Kind)
	case t.Subtype != "":
		return fmt.Sprintf("%s %s/%s", t.Kind, t.Subtype, t.ID)
	default:
		return fmt.Sprintf("%s %s", t.Kind, t.ID)
	}
}

// ErrNotLink is returned for input that is not a Radio Javan link.
var ErrNotLink = errors.New("not a Radio Javan link")

// errNeedsAPI marks links that only the API can resolve, such as rj.app
// short links.
var errNeedsAPI = errors.New("link has to be resolved by the API")

// Keywords accepted in place of a link for the logged-in user's lists.
var keywords = map[string]Kind{
	"liked":        KindLiked,
	"likes":        KindLiked,
	"liked-songs":  KindLiked,
	"library":      KindLibrary,
	"my-playlists": KindMyPlaylists,
	"myplaylists":  KindMyPlaylists,
}

// ParseTarget recognises Radio Javan links without network access:
// play.radiojavan.com pages, old radiojavan.com pages, radiojavan:// app
// links and the keywords liked, library and my-playlists. rj.app short links
// return an error; use Client.Resolve for those.
func ParseTarget(input string) (Target, error) {
	s := strings.TrimSpace(input)
	if k, ok := keywords[strings.ToLower(s)]; ok {
		return Target{Kind: k}, nil
	}
	if len(s) > 13 && strings.EqualFold(s[:13], "radiojavan://") {
		if t, ok := parsePath(splitPath(s[13:])); ok {
			return t, nil
		}
		return Target{}, errNeedsAPI
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return Target{}, ErrNotLink
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "rj.app" || strings.HasSuffix(host, ".rj.app"):
		return Target{}, errNeedsAPI
	case host == "radiojavan.com" || strings.HasSuffix(host, ".radiojavan.com"):
		if t, ok := parsePath(splitPath(u.EscapedPath())); ok {
			return t, nil
		}
		return Target{}, errNeedsAPI
	}
	return Target{}, ErrNotLink
}

// IsLink reports whether s is something Resolve can handle.
func IsLink(s string) bool {
	_, err := ParseTarget(s)
	return err == nil || errors.Is(err, errNeedsAPI)
}

// splitPath splits an escaped URL path into its escaped segments.
func splitPath(p string) []string {
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	var segs []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func parsePath(segs []string) (Target, bool) {
	at := func(i int) string {
		if i >= len(segs) {
			return ""
		}
		s, err := url.PathUnescape(segs[i])
		if err != nil {
			return segs[i]
		}
		return s
	}
	lc := func(i int) string { return strings.ToLower(at(i)) }
	media := func(k Kind, i int) (Target, bool) {
		if at(i) == "" {
			return Target{}, false
		}
		return Target{Kind: k, ID: at(i)}, true
	}
	playlist := func(i int) (Target, bool) {
		if sub := lc(i); (sub == "mp3" || sub == "video") && at(i+1) != "" {
			return Target{Kind: KindPlaylist, Subtype: sub, ID: at(i + 1)}, true
		}
		return Target{}, false
	}

	switch lc(0) {
	case "song", "mp3":
		return media(KindSong, 1)
	case "video":
		return media(KindVideo, 1)
	case "podcast":
		if lc(1) == "show" {
			return media(KindShow, 2)
		}
		return media(KindPodcast, 1)
	case "show":
		return media(KindShow, 1)
	case "album", "mp3_album":
		return media(KindAlbum, 1)
	case "playlist":
		return playlist(1)
	case "artist":
		if len(segs) < 2 {
			return Target{}, false
		}
		// The web player writes spaces as "+" in artist links.
		name, err := url.PathUnescape(strings.ReplaceAll(segs[1], "+", " "))
		if err != nil || strings.TrimSpace(name) == "" {
			return Target{}, false
		}
		return Target{Kind: KindArtist, ID: strings.TrimSpace(name)}, true
	case "library":
		if lc(1) == "liked" {
			return Target{Kind: KindLiked}, true
		}
		return Target{Kind: KindLibrary}, true
	// Old radiojavan.com pages.
	case "mp3s":
		switch lc(1) {
		case "mp3":
			return media(KindSong, 2)
		case "album":
			return media(KindAlbum, 2)
		}
	case "videos":
		if lc(1) == "video" {
			return media(KindVideo, 2)
		}
	case "podcasts":
		if lc(1) == "podcast" {
			return media(KindPodcast, 2)
		}
	case "playlists":
		if lc(1) == "playlist" {
			return playlist(2)
		}
	}
	return Target{}, false
}

// Resolve turns a link into a Target. Links that cannot be parsed locally,
// such as rj.app short links, are resolved through the API.
func (c *Client) Resolve(ctx context.Context, input string) (Target, error) {
	t, err := ParseTarget(input)
	if !errors.Is(err, errNeedsAPI) {
		return t, err
	}
	link := strings.TrimSpace(input)
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	return c.Deeplink(ctx, link)
}

type rawDeeplink struct {
	Type     string     `json:"type"`
	Subtype  string     `json:"subtype"`
	ID       flexString `json:"id"`
	Permlink string     `json:"permlink"`
}

// Deeplink asks the API what a Radio Javan link points to. It understands
// rj.app short links as well as site and app links.
func (c *Client) Deeplink(ctx context.Context, link string) (Target, error) {
	var r rawDeeplink
	err := c.getJSON(ctx, "deeplink", url.Values{"url": {link}}, &r)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusOK {
		return Target{}, fmt.Errorf("%s: %w", link, ErrNotFound)
	}
	if err != nil {
		return Target{}, err
	}
	id := firstNonEmpty(r.Permlink, string(r.ID))
	switch r.Type {
	case "mp3":
		return Target{Kind: KindSong, ID: id}, nil
	case "video":
		return Target{Kind: KindVideo, ID: id}, nil
	case "podcast":
		return Target{Kind: KindPodcast, ID: id}, nil
	case "mp3_album", "album":
		return Target{Kind: KindAlbum, ID: id}, nil
	case "playlist":
		return Target{Kind: KindPlaylist, Subtype: firstNonEmpty(r.Subtype, "mp3"), ID: string(r.ID)}, nil
	case "artist":
		return Target{Kind: KindArtist, ID: string(r.ID)}, nil
	case "show":
		return Target{Kind: KindShow, ID: id}, nil
	case "":
		return Target{}, fmt.Errorf("%s: %w", link, ErrNotFound)
	}
	return Target{}, fmt.Errorf("%s: %q links are not supported", link, r.Type)
}
