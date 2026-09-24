package rj

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Kind is the type of a Radio Javan entity.
type Kind string

const (
	KindSong        Kind = "song"
	KindVideo       Kind = "video"
	KindPodcast     Kind = "podcast"
	KindAlbum       Kind = "album"
	KindPlaylist    Kind = "playlist"
	KindArtist      Kind = "artist"
	KindShow        Kind = "show"
	KindLiked       Kind = "liked"
	KindLibrary     Kind = "library"
	KindMyPlaylists Kind = "my_playlists"
	// KindGroup is a section of another collection, such as an artist's
	// videos.
	KindGroup Kind = "group"
)

// Entity is either a *Media or a *Collection.
type Entity interface{ entity() }

func (*Media) entity()      {}
func (*Collection) entity() {}

// Media is a single downloadable item: a song, a music video or a podcast
// episode.
type Media struct {
	Kind         Kind        `json:"type"`
	ID           string      `json:"id"`
	Permlink     string      `json:"permlink,omitempty"`
	URL          string      `json:"url,omitempty"`
	ShareURL     string      `json:"share_url,omitempty"`
	Title        string      `json:"title"`
	Artist       string      `json:"artist,omitempty"`
	TitleFa      string      `json:"title_fa,omitempty"`
	ArtistFa     string      `json:"artist_fa,omitempty"`
	Album        *AlbumRef   `json:"album,omitempty"`
	Show         string      `json:"show,omitempty"`
	Duration     float64     `json:"duration,omitempty"`
	Date         string      `json:"date,omitempty"`
	Explicit     bool        `json:"explicit"`
	PremiumOnly  bool        `json:"premium_only"`
	Stats        Stats       `json:"stats"`
	Cover        Cover       `json:"cover"`
	Formats      []Format    `json:"formats"`
	Lyrics       string      `json:"lyrics,omitempty"`
	SyncedLyrics []LyricLine `json:"synced_lyrics,omitempty"`
	Tracklist    string      `json:"tracklist,omitempty"`
}

// HasLyrics reports whether m carries lyrics.
func (m *Media) HasLyrics() bool { return m.Lyrics != "" || len(m.SyncedLyrics) > 0 }

// AlbumRef describes the album a song belongs to.
type AlbumRef struct {
	ID       string `json:"id,omitempty"`
	Title    string `json:"title"`
	Artist   string `json:"artist,omitempty"`
	TitleFa  string `json:"title_fa,omitempty"`
	Permlink string `json:"permlink,omitempty"`
	Track    int    `json:"track,omitempty"`
	Year     string `json:"year,omitempty"`
	URL      string `json:"url,omitempty"`
	ShareURL string `json:"share_url,omitempty"`
}

// Stats are the public counters of an item.
type Stats struct {
	Plays     int64 `json:"plays,omitempty"`
	Views     int64 `json:"views,omitempty"`
	Likes     int64 `json:"likes,omitempty"`
	Dislikes  int64 `json:"dislikes,omitempty"`
	Downloads int64 `json:"downloads,omitempty"`
}

// Cover holds the artwork URLs of an item.
type Cover struct {
	Large     string `json:"large,omitempty"`
	Player    string `json:"player,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
}

// Best returns the largest available artwork.
func (c Cover) Best() string { return firstNonEmpty(c.Large, c.Player, c.Thumbnail) }

// Format is one downloadable file of a media item.
type Format struct {
	ID      string `json:"id"`                // e.g. mp3-256, aac-128, mp4-1080p, hls
	Ext     string `json:"ext"`               // mp3, m4a, mp4 or m3u8
	Bitrate int    `json:"bitrate,omitempty"` // kbit/s, audio only
	Height  int    `json:"height,omitempty"`  // pixels, video only
	URL     string `json:"url"`
}

// IsHLS reports whether f is a stream playlist rather than a file.
func (f Format) IsHLS() bool { return f.Ext == "m3u8" }

// LyricLine is one line of time-synced lyrics.
type LyricLine struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

// Collection is a list of media: an album, a playlist, a podcast show, an
// artist's catalogue or one of the logged-in user's lists.
type Collection struct {
	Kind     Kind          `json:"type"`
	ID       string        `json:"id,omitempty"`
	Subtype  string        `json:"subtype,omitempty"` // playlists: mp3 or video
	Title    string        `json:"title"`
	Artist   string        `json:"artist,omitempty"`
	TitleFa  string        `json:"title_fa,omitempty"`
	Year     string        `json:"year,omitempty"`
	URL      string        `json:"url,omitempty"`
	ShareURL string        `json:"share_url,omitempty"`
	Cover    Cover         `json:"cover"`
	Items    []*Media      `json:"items"`
	Children []*Collection `json:"children,omitempty"`
	// Warnings lists parts that could not be loaded.
	Warnings []string `json:"warnings,omitempty"`
}

// Len returns the number of media items in c and all its children.
func (c *Collection) Len() int {
	n := len(c.Items)
	for _, ch := range c.Children {
		n += ch.Len()
	}
	return n
}

// flexString decodes JSON strings and numbers.
type flexString string

func (s *flexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = flexString(v)
		return nil
	}
	if string(b) == "null" || string(b) == "false" {
		*s = ""
		return nil
	}
	*s = flexString(b)
	return nil
}

// flexInt decodes numbers and strings such as "16,918".
type flexInt int64

func (n *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.ReplaceAll(strings.Trim(string(b), `"`), ",", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = flexInt(f)
	return nil
}

// flexFloat decodes numbers and numeric strings.
type flexFloat float64

func (n *flexFloat) UnmarshalJSON(b []byte) error {
	f, err := strconv.ParseFloat(strings.Trim(string(b), `"`), 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = flexFloat(f)
	return nil
}

// rawItem is a song, video or podcast as returned by the API.
type rawItem struct {
	ID            flexString `json:"id"`
	Type          string     `json:"type"`
	Title         string     `json:"title"`
	Artist        string     `json:"artist"`
	Song          string     `json:"song"`
	PodcastArtist string     `json:"podcast_artist"`
	ArtistFarsi   string     `json:"artist_farsi"`
	SongFarsi     string     `json:"song_farsi"`
	TitleFarsi    string     `json:"title_farsi"`
	Permlink      string     `json:"permlink"`
	ShareLink     string     `json:"share_link"`
	CreatedAt     string     `json:"created_at"`
	Duration      flexFloat  `json:"duration"`
	Plays         flexInt    `json:"plays"`
	Views         flexInt    `json:"views"`
	Likes         flexInt    `json:"likes"`
	Dislikes      flexInt    `json:"dislikes"`
	Downloads     flexInt    `json:"downloads"`
	Explicit      bool       `json:"explicit"`
	PremiumOnly   bool       `json:"premium_only"`

	Link    string `json:"link"`
	HQLink  string `json:"hq_link"`
	LQLink  string `json:"lq_link"`
	HLSLink string `json:"hls_link"`
	HQHLS   string `json:"hq_hls"`
	HLS     string `json:"hls"`
	HLSHEVC string `json:"hls_hevc"`

	Photo       string `json:"photo"`
	PhotoLarge  string `json:"photo_large"`
	PhotoPlayer string `json:"photo_player"`
	Thumbnail   string `json:"thumbnail"`

	Album          json.RawMessage `json:"album"` // object in lists, album name in details
	AlbumAlbum     string          `json:"album_album"`
	AlbumArtist    string          `json:"album_artist"`
	AlbumFarsi     string          `json:"album_farsi"`
	AlbumShareLink string          `json:"album_share_link"`
	ReleaseYear    flexString      `json:"release_year"`
	AlbumTracks    []rawItem       `json:"album_tracks"`

	Lyric        string      `json:"lyric"`
	LyricSynced  []LyricLine `json:"lyric_synced"`
	Tracklist    string      `json:"tracklist"`
	ShowPermlink string      `json:"show_permlink"`
}

type rawAlbumRef struct {
	ID        flexString `json:"id"`
	Album     string     `json:"album"`
	Artist    string     `json:"artist"`
	Track     flexInt    `json:"track"`
	ShareLink string     `json:"share_link"`
	Permlink  string     `json:"permlink"`
}

func (r *rawItem) kind() Kind {
	switch r.Type {
	case "video":
		return KindVideo
	case "podcast":
		return KindPodcast
	default:
		return KindSong
	}
}

func (r *rawItem) media() *Media {
	m := &Media{
		Kind:        r.kind(),
		ID:          string(r.ID),
		Permlink:    r.Permlink,
		ShareURL:    r.ShareLink,
		Duration:    float64(r.Duration),
		Date:        r.CreatedAt,
		Explicit:    r.Explicit,
		PremiumOnly: r.PremiumOnly,
		Stats: Stats{
			Plays:     int64(r.Plays),
			Views:     int64(r.Views),
			Likes:     int64(r.Likes),
			Dislikes:  int64(r.Dislikes),
			Downloads: int64(r.Downloads),
		},
		Cover: Cover{
			Large:     firstNonEmpty(r.PhotoLarge, r.Photo),
			Player:    r.PhotoPlayer,
			Thumbnail: r.Thumbnail,
		},
		Formats:      r.formats(),
		Lyrics:       strings.TrimSpace(r.Lyric),
		SyncedLyrics: r.LyricSynced,
	}
	if m.Kind == KindPodcast {
		m.Title = r.Title
		m.Artist = r.PodcastArtist
		m.TitleFa = r.TitleFarsi
		m.ArtistFa = r.ArtistFarsi
		m.Show = r.ShowPermlink
		m.Tracklist = strings.TrimSpace(r.Tracklist)
	} else {
		m.Title = firstNonEmpty(r.Song, r.Title)
		m.Artist = r.Artist
		m.TitleFa = r.SongFarsi
		m.ArtistFa = r.ArtistFarsi
		m.Album = r.albumRef()
	}
	m.URL = PageURL(m.Kind, m.Permlink)
	return m
}

// albumRef merges the two shapes the API uses for album information.
func (r *rawItem) albumRef() *AlbumRef {
	var ref AlbumRef
	if len(r.Album) > 0 {
		switch r.Album[0] {
		case '{':
			var a rawAlbumRef
			if json.Unmarshal(r.Album, &a) == nil {
				ref = AlbumRef{
					ID:       string(a.ID),
					Title:    a.Album,
					Artist:   a.Artist,
					Track:    int(a.Track),
					ShareURL: a.ShareLink,
					Permlink: a.Permlink,
				}
			}
		case '"':
			_ = json.Unmarshal(r.Album, &ref.Title)
		}
	}
	// Song details only name the album; its track list has the rest.
	if ref.Permlink == "" {
		for i := range r.AlbumTracks {
			if r.AlbumTracks[i].ID == r.ID {
				if t := r.AlbumTracks[i].albumRef(); t != nil {
					ref.ID, ref.Permlink, ref.Track = t.ID, t.Permlink, t.Track
				}
				break
			}
		}
	}
	ref.Title = firstNonEmpty(r.AlbumAlbum, ref.Title)
	ref.Artist = firstNonEmpty(r.AlbumArtist, ref.Artist)
	ref.TitleFa = r.AlbumFarsi
	ref.ShareURL = firstNonEmpty(ref.ShareURL, r.AlbumShareLink)
	ref.Year = string(r.ReleaseYear)
	if ref.Title == "" && ref.Permlink == "" {
		return nil
	}
	ref.URL = PageURL(KindAlbum, ref.Permlink)
	return &ref
}

var (
	reAudioPath  = regexp.MustCompile(`/(mp3|aac)-(\d+)/`)
	reVideoPath  = regexp.MustCompile(`/music_video/(4k|hd|hq|lq)/`)
	videoHeights = map[string]int{"4k": 2160, "hd": 1080, "hq": 720, "lq": 480}
	videoDirs    = map[int]string{2160: "4k", 1080: "hd", 720: "hq", 480: "lq"}
)

func (r *rawItem) formats() []Format {
	var out []Format
	seen := map[string]bool{}
	add := func(u, id string) {
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		f := classify(u)
		if id != "" {
			f.ID = id
		}
		out = append(out, f)
	}
	add(r.Link, "")
	add(r.HQLink, "")
	add(r.LQLink, "")
	add(firstNonEmpty(r.HLSLink, r.HQHLS, r.HLS), "hls")
	add(r.HLSHEVC, "hls-hevc")
	return out
}

// classify describes a media URL from its path, e.g.
// .../media/mp3/aac-256/x.m4a or .../media/music_video/hd/x.mp4.
func classify(u string) Format {
	f := Format{URL: u, Ext: extOf(u)}
	if m := reAudioPath.FindStringSubmatch(u); m != nil {
		f.Bitrate, _ = strconv.Atoi(m[2])
		f.ID = m[1] + "-" + m[2]
		return f
	}
	if m := reVideoPath.FindStringSubmatch(u); m != nil {
		f.Height = videoHeights[m[1]]
		f.ID = fmt.Sprintf("mp4-%dp", f.Height)
		return f
	}
	f.ID = f.Ext
	if f.IsHLS() {
		f.ID = "hls"
	}
	return f
}

func extOf(u string) string {
	p := u
	if pu, err := url.Parse(u); err == nil {
		p = pu.Path
	}
	return strings.ToLower(strings.TrimPrefix(path.Ext(p), "."))
}

// Preference chooses between the files of a media item.
type Preference struct {
	// Audio is "mp3" (default), "m4a" (AAC, best bitrate) or "m4a-low"
	// (AAC, smallest).
	Audio string
	// Video is "best" (default) or a maximum height: "2160", "1080", "720"
	// or "480".
	Video string
}

// Candidate is a file to try for a media item.
type Candidate struct {
	Format
	// Derived is set when the URL was not listed by the API but guessed
	// from another format; it must be checked before use.
	Derived bool
}

// Candidates returns the files of m in order of preference.
func (m *Media) Candidates(p Preference) []Candidate {
	if m.Kind == KindVideo {
		return videoCandidates(m.Formats, p.Video)
	}
	return audioCandidates(m.Formats, p.Audio)
}

func audioCandidates(formats []Format, pref string) []Candidate {
	var direct, aac, hls []Format
	for _, f := range formats {
		switch {
		case f.IsHLS():
			hls = append(hls, f)
		case f.Ext == "m4a":
			aac = append(aac, f)
		default:
			direct = append(direct, f)
		}
	}
	sort.SliceStable(aac, func(i, j int) bool { return aac[i].Bitrate > aac[j].Bitrate })
	var order []Format
	switch pref {
	case "m4a":
		order = append(append(order, aac...), direct...)
	case "m4a-low":
		for i := len(aac) - 1; i >= 0; i-- {
			order = append(order, aac[i])
		}
		order = append(order, direct...)
	default:
		order = append(append(order, direct...), aac...)
	}
	return wrap(append(order, hls...))
}

func videoCandidates(formats []Format, pref string) []Candidate {
	var mp4s, hls []Format
	byHeight := map[int]Format{}
	for _, f := range formats {
		switch {
		case f.IsHLS():
			hls = append(hls, f)
		case f.Height > 0:
			if _, ok := byHeight[f.Height]; !ok {
				byHeight[f.Height] = f
				mp4s = append(mp4s, f)
			}
		default:
			mp4s = append(mp4s, f)
		}
	}
	sort.SliceStable(mp4s, func(i, j int) bool { return mp4s[i].Height > mp4s[j].Height })

	want, _ := strconv.Atoi(strings.TrimSuffix(strings.ToLower(pref), "p"))
	if strings.EqualFold(pref, "4k") {
		want = 2160
	}
	if _, known := videoDirs[want]; !known || len(byHeight) == 0 {
		return wrap(append(mp4s, hls...))
	}

	// The wanted height, then smaller ones, then larger ones.
	heights := []int{2160, 1080, 720, 480}
	order := []int{want}
	for _, h := range heights {
		if h < want {
			order = append(order, h)
		}
	}
	for i := len(heights) - 1; i >= 0; i-- {
		if heights[i] > want {
			order = append(order, heights[i])
		}
	}
	var base string
	for _, f := range mp4s {
		if f.Height > 0 {
			base = f.URL
			break
		}
	}
	var out []Candidate
	for _, h := range order {
		if f, ok := byHeight[h]; ok {
			out = append(out, Candidate{Format: f})
			continue
		}
		u := reVideoPath.ReplaceAllString(base, "/music_video/"+videoDirs[h]+"/")
		out = append(out, Candidate{
			Format:  Format{ID: fmt.Sprintf("mp4-%dp", h), Ext: "mp4", Height: h, URL: u},
			Derived: true,
		})
	}
	return append(out, wrap(hls)...)
}

func wrap(fs []Format) []Candidate {
	out := make([]Candidate, len(fs))
	for i, f := range fs {
		out[i] = Candidate{Format: f}
	}
	return out
}

// PageURL returns the play.radiojavan.com page of an entity.
func PageURL(k Kind, id string) string {
	if id == "" {
		return ""
	}
	switch k {
	case KindSong:
		return SiteURL + "/song/" + strings.ToLower(id)
	case KindVideo:
		return SiteURL + "/video/" + strings.ToLower(id)
	case KindPodcast:
		return SiteURL + "/podcast/" + strings.ToLower(id)
	case KindAlbum:
		return SiteURL + "/album/" + strings.ToLower(id)
	case KindShow:
		return SiteURL + "/podcast/show/" + strings.ToLower(id)
	case KindArtist:
		return SiteURL + "/artist/" + artistSlug(id)
	}
	return ""
}

// PlaylistURL returns the play.radiojavan.com page of a playlist.
func PlaylistURL(subtype, id string) string {
	if subtype == "" {
		subtype = "mp3"
	}
	return SiteURL + "/playlist/" + subtype + "/" + id
}

// artistSlug encodes an artist name the way the web player does:
// lower case with "+" for spaces.
func artistSlug(name string) string {
	s := url.PathEscape(strings.ToLower(strings.TrimSpace(name)))
	s = strings.ReplaceAll(s, "%20", "+")
	return strings.ReplaceAll(s, "&", "%26")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
