package rj

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

var mediaEndpoints = map[Kind]string{
	KindSong:    "mp3",
	KindVideo:   "video",
	KindPodcast: "podcast",
}

// Media fetches a song, music video or podcast episode by permlink or
// numeric id.
func (c *Client) Media(ctx context.Context, kind Kind, id string) (*Media, error) {
	endpoint, ok := mediaEndpoints[kind]
	if !ok {
		return nil, fmt.Errorf("%s is not a media type", kind)
	}
	var r rawItem
	if err := c.getJSON(ctx, endpoint, url.Values{"id": {id}}, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, fmt.Errorf("%s %q: %w", kind, id, ErrNotFound)
	}
	return r.media(), nil
}

// Album fetches an album by its permlink or numeric id.
func (c *Client) Album(ctx context.Context, id string) (*Collection, error) {
	var r rawItem
	if err := c.getJSON(ctx, "mp3", url.Values{"id": {id}, "album": {"1"}}, &r); err != nil {
		return nil, err
	}
	if len(r.AlbumTracks) == 0 {
		return nil, fmt.Errorf("album %q: %w", id, ErrNotFound)
	}
	col := &Collection{
		Kind:     KindAlbum,
		Title:    r.AlbumAlbum,
		Artist:   r.AlbumArtist,
		TitleFa:  r.AlbumFarsi,
		Year:     string(r.ReleaseYear),
		ShareURL: r.AlbumShareLink,
		Cover: Cover{
			Large:     firstNonEmpty(r.PhotoLarge, r.Photo),
			Player:    r.PhotoPlayer,
			Thumbnail: r.Thumbnail,
		},
	}
	for i := range r.AlbumTracks {
		m := r.AlbumTracks[i].media()
		if m.Album == nil {
			m.Album = &AlbumRef{}
		}
		a := m.Album
		a.Title = firstNonEmpty(a.Title, col.Title)
		a.Artist = firstNonEmpty(a.Artist, col.Artist)
		a.TitleFa = firstNonEmpty(a.TitleFa, col.TitleFa)
		a.Year = firstNonEmpty(a.Year, col.Year)
		a.ShareURL = firstNonEmpty(a.ShareURL, col.ShareURL)
		col.ID = firstNonEmpty(col.ID, a.ID)
		col.URL = firstNonEmpty(col.URL, a.URL)
		col.Title = firstNonEmpty(col.Title, a.Title)
		col.Artist = firstNonEmpty(col.Artist, a.Artist)
		col.Items = append(col.Items, m)
	}
	return col, nil
}

type rawPlaylist struct {
	ID           flexString `json:"id"`
	Title        string     `json:"title"`
	Subtype      string     `json:"subtype"`
	ShareLink    string     `json:"share_link"`
	Photo        string     `json:"photo"`
	PhotoPlayer  string     `json:"photo_player"`
	Thumbnail    string     `json:"thumbnail"`
	CreatedTitle string     `json:"created_title"`
	Owner        struct {
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	Items []rawItem `json:"items"`
}

func (r *rawPlaylist) collection() *Collection {
	sub := firstNonEmpty(r.Subtype, "mp3")
	col := &Collection{
		Kind:     KindPlaylist,
		ID:       string(r.ID),
		Subtype:  sub,
		Title:    r.Title,
		Artist:   firstNonEmpty(r.Owner.DisplayName, r.CreatedTitle),
		URL:      PlaylistURL(sub, string(r.ID)),
		ShareURL: r.ShareLink,
		Cover:    Cover{Large: r.Photo, Player: r.PhotoPlayer, Thumbnail: r.Thumbnail},
	}
	for i := range r.Items {
		col.Items = append(col.Items, r.Items[i].media())
	}
	return col
}

// Playlist fetches a playlist. subtype is "mp3" or "video".
func (c *Client) Playlist(ctx context.Context, subtype, id string) (*Collection, error) {
	endpoint := "mp3_playlist_with_items"
	if subtype == "video" {
		endpoint = "video_playlist_with_items"
	}
	var r rawPlaylist
	if err := c.getJSON(ctx, endpoint, url.Values{"id": {id}}, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, fmt.Errorf("playlist %q: %w", id, ErrNotFound)
	}
	if r.Subtype == "" {
		r.Subtype = subtype
	}
	return r.collection(), nil
}

type rawArtist struct {
	Artist      string    `json:"artist"`
	ArtistFarsi string    `json:"artist_farsi"`
	ShareLink   string    `json:"share_link"`
	Photo       string    `json:"photo"`
	PhotoPlayer string    `json:"photo_player"`
	PhotoThumb  string    `json:"photo_thumb"`
	MP3s        []rawItem `json:"mp3s"`
	Albums      []rawItem `json:"albums"`
	Videos      []rawItem `json:"videos"`
	Podcasts    []rawItem `json:"podcasts"`
}

func (r *rawArtist) empty() bool {
	return r.Artist == "" && len(r.MP3s)+len(r.Albums)+len(r.Videos)+len(r.Podcasts) == 0
}

// ArtistOptions selects what Artist loads besides songs and albums.
type ArtistOptions struct {
	Videos   bool
	Podcasts bool
}

func (c *Client) artist(ctx context.Context, name string) (*rawArtist, error) {
	var r rawArtist
	err := c.getJSON(ctx, "artist", url.Values{"query": {name}, "v": {"2"}}, &r)
	return &r, err
}

// Artist fetches an artist's catalogue. Album tracks are grouped into one
// child collection per album; songs that are not on any album are Items.
func (c *Client) Artist(ctx context.Context, name string, opt ArtistOptions) (*Collection, error) {
	r, err := c.artist(ctx, name)
	if err != nil {
		return nil, err
	}
	if r.empty() && strings.Contains(name, "-") {
		// Old radiojavan.com links use dashes where names have spaces.
		if r2, err := c.artist(ctx, strings.ReplaceAll(name, "-", " ")); err == nil && !r2.empty() {
			r = r2
		}
	}
	if r.empty() {
		return nil, fmt.Errorf("artist %q: %w", name, ErrNotFound)
	}
	artist := firstNonEmpty(r.Artist, name)
	col := &Collection{
		Kind:     KindArtist,
		ID:       artist,
		Title:    artist,
		TitleFa:  r.ArtistFarsi,
		URL:      PageURL(KindArtist, artist),
		ShareURL: r.ShareLink,
		Cover:    Cover{Large: r.Photo, Player: r.PhotoPlayer, Thumbnail: r.PhotoThumb},
	}
	// The song list misses some album tracks, so every album is loaded.
	col.Children = c.albums(ctx, r.Albums, col)
	inAlbum := map[string]bool{}
	for _, a := range col.Children {
		for _, m := range a.Items {
			inAlbum[m.ID] = true
		}
	}
	for i := range r.MP3s {
		if m := r.MP3s[i].media(); !inAlbum[m.ID] {
			col.Items = append(col.Items, m)
		}
	}
	if opt.Videos && len(r.Videos) > 0 {
		col.Children = append(col.Children, group("Videos", r.Videos))
	}
	if opt.Podcasts && len(r.Podcasts) > 0 {
		col.Children = append(col.Children, group("Podcasts", r.Podcasts))
	}
	return col, nil
}

// albums loads the albums referenced by refs, four at a time, keeping their
// order. Failures are recorded as warnings on parent.
func (c *Client) albums(ctx context.Context, refs []rawItem, parent *Collection) []*Collection {
	type result struct {
		album *Collection
		err   error
		name  string
	}
	results := make([]result, len(refs))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := range refs {
		ref := refs[i].albumRef()
		if ref == nil {
			continue
		}
		id := firstNonEmpty(ref.Permlink, ref.ID)
		if id == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			a, err := c.Album(ctx, id)
			results[i] = result{a, err, ref.Title}
		}()
	}
	wg.Wait()
	var out []*Collection
	seen := map[string]bool{}
	for _, res := range results {
		switch {
		case res.err != nil:
			parent.Warnings = append(parent.Warnings, fmt.Sprintf("album %q: %v", res.name, res.err))
		case res.album != nil && !seen[res.album.ID]:
			seen[res.album.ID] = true
			out = append(out, res.album)
		}
	}
	return out
}

func group(title string, items []rawItem) *Collection {
	g := &Collection{Kind: KindGroup, Title: title}
	for i := range items {
		g.Items = append(g.Items, items[i].media())
	}
	return g
}

type rawShow struct {
	Title      string    `json:"title"`
	Permlink   string    `json:"permlink"`
	ShareLink  string    `json:"share_link"`
	Photo      string    `json:"photo"`
	PhotoLarge string    `json:"photo_large"`
	Thumb      string    `json:"thumb"`
	Podcasts   []rawItem `json:"podcasts"`
}

// Show fetches a podcast show with its episodes.
func (c *Client) Show(ctx context.Context, id string) (*Collection, error) {
	var r rawShow
	if err := c.getJSON(ctx, "podcast_show", url.Values{"id": {id}}, &r); err != nil {
		return nil, err
	}
	if r.Title == "" && len(r.Podcasts) == 0 {
		return nil, fmt.Errorf("podcast show %q: %w", id, ErrNotFound)
	}
	permlink := firstNonEmpty(r.Permlink, id)
	col := &Collection{
		Kind:     KindShow,
		ID:       permlink,
		Title:    r.Title,
		URL:      PageURL(KindShow, permlink),
		ShareURL: r.ShareLink,
		Cover:    Cover{Large: firstNonEmpty(r.PhotoLarge, r.Photo), Thumbnail: r.Thumb},
	}
	for i := range r.Podcasts {
		m := r.Podcasts[i].media()
		col.Artist = firstNonEmpty(col.Artist, m.Artist)
		col.Items = append(col.Items, m)
	}
	return col, nil
}

// SearchResult is one search hit.
type SearchResult struct {
	Kind   Kind   `json:"type"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist,omitempty"`
	URL    string `json:"url"`
}

// SearchResults are search hits grouped by type, most relevant first.
type SearchResults struct {
	Query     string         `json:"query"`
	Artists   []SearchResult `json:"artists"`
	Songs     []SearchResult `json:"songs"`
	Albums    []SearchResult `json:"albums"`
	Videos    []SearchResult `json:"videos"`
	Podcasts  []SearchResult `json:"podcasts"`
	Shows     []SearchResult `json:"shows"`
	Playlists []SearchResult `json:"playlists"`
}

type rawSearch struct {
	AllArtists []rawArtistHit `json:"all_artists"`
	Artists    []rawArtistHit `json:"artists"`
	MP3s       []rawItem      `json:"mp3s"`
	Albums     []rawItem      `json:"albums"`
	Videos     []rawItem      `json:"videos"`
	Podcasts   []rawItem      `json:"podcasts"`
	Shows      []rawShow      `json:"shows"`
	Playlists  []struct {
		Playlist rawPlaylist `json:"playlist"`
	} `json:"playlists"`
}

type rawArtistHit struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

// Search searches Radio Javan. It does not need an account.
func (c *Client) Search(ctx context.Context, query string) (*SearchResults, error) {
	var r rawSearch
	if err := c.getJSON(ctx, "search", url.Values{"query": {query}}, &r); err != nil {
		return nil, err
	}
	res := &SearchResults{Query: query}
	artists := r.AllArtists
	if len(artists) == 0 {
		artists = r.Artists
	}
	for _, a := range artists {
		if name := firstNonEmpty(a.Name, a.Query); name != "" {
			res.Artists = append(res.Artists, SearchResult{Kind: KindArtist, ID: name, Title: name, URL: PageURL(KindArtist, name)})
		}
	}
	for i := range r.MP3s {
		res.Songs = append(res.Songs, mediaHit(r.MP3s[i].media()))
	}
	for i := range r.Videos {
		res.Videos = append(res.Videos, mediaHit(r.Videos[i].media()))
	}
	for i := range r.Podcasts {
		res.Podcasts = append(res.Podcasts, mediaHit(r.Podcasts[i].media()))
	}
	seen := map[string]bool{}
	for i := range r.Albums {
		ref := r.Albums[i].albumRef()
		if ref == nil || ref.Permlink == "" || seen[strings.ToLower(ref.Permlink)] {
			continue
		}
		seen[strings.ToLower(ref.Permlink)] = true
		res.Albums = append(res.Albums, SearchResult{Kind: KindAlbum, ID: ref.Permlink, Title: ref.Title, Artist: ref.Artist, URL: ref.URL})
	}
	for _, s := range r.Shows {
		if s.Permlink != "" {
			res.Shows = append(res.Shows, SearchResult{Kind: KindShow, ID: s.Permlink, Title: s.Title, URL: PageURL(KindShow, s.Permlink)})
		}
	}
	for _, p := range r.Playlists {
		if pl := p.Playlist; pl.ID != "" {
			sub := firstNonEmpty(pl.Subtype, "mp3")
			res.Playlists = append(res.Playlists, SearchResult{
				Kind:   KindPlaylist,
				ID:     string(pl.ID),
				Title:  pl.Title,
				Artist: firstNonEmpty(pl.Owner.DisplayName, pl.CreatedTitle),
				URL:    PlaylistURL(sub, string(pl.ID)),
			})
		}
	}
	return res, nil
}

func mediaHit(m *Media) SearchResult {
	return SearchResult{Kind: m.Kind, ID: firstNonEmpty(m.Permlink, m.ID), Title: m.Title, Artist: m.Artist, URL: m.URL}
}

// FetchOptions tunes Fetch.
type FetchOptions struct {
	Artist ArtistOptions
}

// Fetch loads the entity a Target points to.
func (c *Client) Fetch(ctx context.Context, t Target, opt FetchOptions) (Entity, error) {
	switch t.Kind {
	case KindSong, KindVideo, KindPodcast:
		return c.Media(ctx, t.Kind, t.ID)
	case KindAlbum:
		return c.Album(ctx, t.ID)
	case KindPlaylist:
		return c.Playlist(ctx, t.Subtype, t.ID)
	case KindArtist:
		return c.Artist(ctx, t.ID, opt.Artist)
	case KindShow:
		return c.Show(ctx, t.ID)
	case KindLiked:
		return c.Liked(ctx)
	case KindLibrary:
		return c.Library(ctx)
	case KindMyPlaylists:
		return c.MyPlaylists(ctx)
	}
	return nil, fmt.Errorf("cannot fetch %s", t.Kind)
}

// FetchRaw returns the unprocessed API response behind a Target.
func (c *Client) FetchRaw(ctx context.Context, t Target) (json.RawMessage, error) {
	switch t.Kind {
	case KindSong, KindVideo, KindPodcast:
		return c.getRaw(ctx, mediaEndpoints[t.Kind], url.Values{"id": {t.ID}})
	case KindAlbum:
		return c.getRaw(ctx, "mp3", url.Values{"id": {t.ID}, "album": {"1"}})
	case KindPlaylist:
		endpoint := "mp3_playlist_with_items"
		if t.Subtype == "video" {
			endpoint = "video_playlist_with_items"
		}
		return c.getRaw(ctx, endpoint, url.Values{"id": {t.ID}})
	case KindArtist:
		return c.getRaw(ctx, "artist", url.Values{"query": {t.ID}, "v": {"2"}})
	case KindShow:
		return c.getRaw(ctx, "podcast_show", url.Values{"id": {t.ID}})
	case KindLiked, KindLibrary, KindMyPlaylists:
		if _, err := c.Profile(ctx); err != nil {
			return nil, err
		}
		switch t.Kind {
		case KindLiked:
			return c.getRaw(ctx, "mp3s_liked", nil)
		case KindLibrary:
			return c.libraryRaw(ctx)
		default:
			return c.getRaw(ctx, "playlists_dash", nil)
		}
	}
	return nil, fmt.Errorf("cannot fetch %s", t.Kind)
}
