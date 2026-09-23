package rj

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		in   string
		want Target
	}{
		{"https://play.radiojavan.com/song/raha-oh-nana", Target{Kind: KindSong, ID: "raha-oh-nana"}},
		{"play.radiojavan.com/song/Raha-Oh-Nana?utm=x", Target{Kind: KindSong, ID: "Raha-Oh-Nana"}},
		{"https://play.radiojavan.com/song/ebi-hamin-khoobe-(ft-shadmehr-aghili)", Target{Kind: KindSong, ID: "ebi-hamin-khoobe-(ft-shadmehr-aghili)"}},
		{"https://play.radiojavan.com/song/raha-oh-nana/stories/abc", Target{Kind: KindSong, ID: "raha-oh-nana"}},
		{"https://play.radiojavan.com/video/amir-maghare-man-delam-tange", Target{Kind: KindVideo, ID: "amir-maghare-man-delam-tange"}},
		{"https://play.radiojavan.com/podcast/dance-station-41", Target{Kind: KindPodcast, ID: "dance-station-41"}},
		{"https://play.radiojavan.com/podcast/show/dance-station", Target{Kind: KindShow, ID: "dance-station"}},
		{"https://play.radiojavan.com/album/satin-ghanune-bagha", Target{Kind: KindAlbum, ID: "satin-ghanune-bagha"}},
		{"https://play.radiojavan.com/playlist/mp3/205e3f10cd96", Target{Kind: KindPlaylist, Subtype: "mp3", ID: "205e3f10cd96"}},
		{"https://play.radiojavan.com/playlist/video/0a278cbb50eb", Target{Kind: KindPlaylist, Subtype: "video", ID: "0a278cbb50eb"}},
		{"https://play.radiojavan.com/artist/siavash+ghomayshi", Target{Kind: KindArtist, ID: "siavash ghomayshi"}},
		{"https://play.radiojavan.com/artist/behin+%26+samin/songs", Target{Kind: KindArtist, ID: "behin & samin"}},
		{"https://play.radiojavan.com/artist/a%2Bb", Target{Kind: KindArtist, ID: "a+b"}},
		{"https://play.radiojavan.com/library/liked/songs", Target{Kind: KindLiked}},
		{"https://play.radiojavan.com/library", Target{Kind: KindLibrary}},
		{"https://www.radiojavan.com/mp3s/mp3/Raha-Oh-Nana", Target{Kind: KindSong, ID: "Raha-Oh-Nana"}},
		{"https://www.radiojavan.com/mp3s/album/Ebi-Koohe-Yakh", Target{Kind: KindAlbum, ID: "Ebi-Koohe-Yakh"}},
		{"radiojavan.com/videos/video/amir-maghare-man-delam-tange", Target{Kind: KindVideo, ID: "amir-maghare-man-delam-tange"}},
		{"https://www.radiojavan.com/podcasts/podcast/Dance-Station-41", Target{Kind: KindPodcast, ID: "Dance-Station-41"}},
		{"https://www.radiojavan.com/playlists/playlist/mp3/205e3f10cd96", Target{Kind: KindPlaylist, Subtype: "mp3", ID: "205e3f10cd96"}},
		{"radiojavan://mp3/159372", Target{Kind: KindSong, ID: "159372"}},
		{"radiojavan://playlist/video/0a278cbb50eb", Target{Kind: KindPlaylist, Subtype: "video", ID: "0a278cbb50eb"}},
		{"radiojavan://show/Dance-Station", Target{Kind: KindShow, ID: "Dance-Station"}},
		{"Liked", Target{Kind: KindLiked}},
		{"library", Target{Kind: KindLibrary}},
		{"my-playlists", Target{Kind: KindMyPlaylists}},
	}
	for _, tt := range tests {
		got, err := ParseTarget(tt.in)
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseTarget(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseTargetNeedsAPIOrFails(t *testing.T) {
	for _, in := range []string{"https://rj.app/m/zE1g57yl", "rj.app/pm/Mg6B7rMg", "https://play.radiojavan.com/events"} {
		if _, err := ParseTarget(in); err != errNeedsAPI {
			t.Errorf("ParseTarget(%q) error = %v, want errNeedsAPI", in, err)
		}
		if !IsLink(in) {
			t.Errorf("IsLink(%q) = false", in)
		}
	}
	for _, in := range []string{"ebi", "ebi koohe yakh", "https://example.com/song/x", "", "https://notradiojavan.com/song/x"} {
		if _, err := ParseTarget(in); err != ErrNotLink {
			t.Errorf("ParseTarget(%q) error = %v, want ErrNotLink", in, err)
		}
		if IsLink(in) {
			t.Errorf("IsLink(%q) = true", in)
		}
	}
}

func TestFlexibleDecoding(t *testing.T) {
	// Lists describe the album as an object, details as a string, and the
	// same field can be a number or a string.
	data := `{
		"id": 159375, "type": "mp3", "artist": "Satin", "song": "Ghanune Bagha",
		"permlink": "Satin-Ghanune-Bagha", "plays": "3,449", "likes": 13, "duration": "196.9",
		"explicit": "not a bool",
		"link": "https://host2.media-rj.com/media/mp3/mp3-256/159375-x.mp3",
		"hq_link": "https://host2.media-rj.com/media/mp3/aac-256/159375-x.m4a",
		"lq_link": "https://host2.media-rj.com/media/mp3/aac-128/159375-x.m4a",
		"hls_link": "https://host2.media-rj.com/media/mp3/mp3-hls/159375-x/playlist.m3u8",
		"album": "Ghanune Bagha", "album_album": "Ghanune Bagha", "album_artist": "Satin",
		"release_year": 2026,
		"album_tracks": [
			{"id": 159375, "type": "mp3", "song": "Ghanune Bagha",
			 "album": {"id": 4587, "album": "Ghanune Bagha", "artist": "Satin", "track": 1, "permlink": "Satin-Ghanune-Bagha"}}
		],
		"lyric_synced": [{"time": 14.06, "text": "line"}]
	}`
	var r rawItem
	if err := decode("mp3", []byte(data), &r); err != nil {
		t.Fatal(err)
	}
	m := r.media()
	if m.ID != "159375" || m.Kind != KindSong || m.Title != "Ghanune Bagha" || m.Artist != "Satin" {
		t.Errorf("basic fields: %+v", m)
	}
	if m.Stats.Plays != 3449 || m.Stats.Likes != 13 || m.Duration != 196.9 {
		t.Errorf("numbers: %+v duration %v", m.Stats, m.Duration)
	}
	if m.Album == nil || m.Album.Title != "Ghanune Bagha" || m.Album.Track != 1 || m.Album.Permlink != "Satin-Ghanune-Bagha" || m.Album.Year != "2026" {
		t.Errorf("album: %+v", m.Album)
	}
	if m.Album.URL != "https://play.radiojavan.com/album/satin-ghanune-bagha" {
		t.Errorf("album url: %s", m.Album.URL)
	}
	ids := []string{}
	for _, f := range m.Formats {
		ids = append(ids, f.ID)
	}
	if got := strings.Join(ids, ","); got != "mp3-256,aac-256,aac-128,hls" {
		t.Errorf("formats = %s", got)
	}
	if len(m.SyncedLyrics) != 1 || m.SyncedLyrics[0].Time != 14.06 {
		t.Errorf("synced lyrics: %+v", m.SyncedLyrics)
	}
}

func TestPodcastFields(t *testing.T) {
	var r rawItem
	data := `{"id":4461,"type":"podcast","title":"Dance Station 41","podcast_artist":"Hosein Aerial",
		"date":"Hosein Aerial","show_permlink":"Dance-Station","permlink":"Dance-Station-41","premium_only":true}`
	if err := decode("podcast", []byte(data), &r); err != nil {
		t.Fatal(err)
	}
	m := r.media()
	if m.Kind != KindPodcast || m.Title != "Dance Station 41" || m.Artist != "Hosein Aerial" || m.Show != "Dance-Station" || !m.PremiumOnly {
		t.Errorf("podcast: %+v", m)
	}
	if m.URL != "https://play.radiojavan.com/podcast/dance-station-41" {
		t.Errorf("url: %s", m.URL)
	}
	if len(m.Candidates(Preference{})) != 0 {
		t.Errorf("premium item without links should have no candidates")
	}
}

func formatIDs(cs []Candidate) string {
	var ids []string
	for _, c := range cs {
		id := c.ID
		if c.Derived {
			id += "*"
		}
		ids = append(ids, id)
	}
	return strings.Join(ids, ",")
}

func TestAudioCandidates(t *testing.T) {
	m := &Media{Kind: KindSong, Formats: []Format{
		classify("https://h/media/mp3/mp3-256/1-a.mp3"),
		classify("https://h/media/mp3/aac-256/1-a.m4a"),
		classify("https://h/media/mp3/aac-128/1-a.m4a"),
		{ID: "hls", Ext: "m3u8", URL: "https://h/x.m3u8"},
	}}
	for pref, want := range map[string]string{
		"":        "mp3-256,aac-256,aac-128,hls",
		"mp3":     "mp3-256,aac-256,aac-128,hls",
		"m4a":     "aac-256,aac-128,mp3-256,hls",
		"m4a-low": "aac-128,aac-256,mp3-256,hls",
	} {
		if got := formatIDs(m.Candidates(Preference{Audio: pref})); got != want {
			t.Errorf("audio %q: got %s, want %s", pref, got, want)
		}
	}
}

func TestVideoCandidates(t *testing.T) {
	m := &Media{Kind: KindVideo, Formats: []Format{
		classify("https://h/media/music_video/hq/v.mp4"),
		classify("https://h/media/music_video/4k/v.mp4"),
		{ID: "hls", Ext: "m3u8", URL: "https://h/v.m3u8"},
	}}
	for pref, want := range map[string]string{
		"best": "mp4-2160p,mp4-720p,hls",
		"2160": "mp4-2160p,mp4-1080p*,mp4-720p,mp4-480p*,hls",
		"1080": "mp4-1080p*,mp4-720p,mp4-480p*,mp4-2160p,hls",
		"720":  "mp4-720p,mp4-480p*,mp4-1080p*,mp4-2160p,hls",
		"480p": "mp4-480p*,mp4-720p,mp4-1080p*,mp4-2160p,hls",
		"4K":   "mp4-2160p,mp4-1080p*,mp4-720p,mp4-480p*,hls",
	} {
		if got := formatIDs(m.Candidates(Preference{Video: pref})); got != want {
			t.Errorf("video %q: got %s, want %s", pref, got, want)
		}
	}
	for _, c := range m.Candidates(Preference{Video: "1080"}) {
		if c.ID == "mp4-1080p" && c.URL != "https://h/media/music_video/hd/v.mp4" {
			t.Errorf("derived 1080p url = %s", c.URL)
		}
	}
}

func TestPageURLs(t *testing.T) {
	if got := PageURL(KindArtist, "Behin & Samin"); got != "https://play.radiojavan.com/artist/behin+%26+samin" {
		t.Errorf("artist url = %s", got)
	}
	// The URL must parse back to the same artist.
	tgt, err := ParseTarget(PageURL(KindArtist, "Behin & Samin"))
	if err != nil || tgt.ID != "behin & samin" {
		t.Errorf("round trip: %+v %v", tgt, err)
	}
}

func TestSessionHeader(t *testing.T) {
	for in, want := range map[string]string{
		"abc":                    "_rj_web=abc",
		" _rj_web=abc ":          "_rj_web=abc",
		"Cookie: a=1; _rj_web=b": "a=1; _rj_web=b",
		"":                       "",
	} {
		if got := SessionHeader(in); got != want {
			t.Errorf("SessionHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFailureMessage(t *testing.T) {
	for body, want := range map[string]bool{
		`{"success":false}`:                   true,
		`{"success":false,"msg":"bad login"}`: true,
		// The subscription endpoint says success:false for free accounts.
		`{"success":false,"display_ads":true}`: false,
		`{"success":true}`:                     false,
		`[]`:                                   false,
		`{}`:                                   false,
	} {
		if _, got := failureMessage([]byte(body)); got != want {
			t.Errorf("failureMessage(%s) = %v, want %v", body, got, want)
		}
	}
}

type recorded struct {
	*http.Request
	body string
}

// fakeAPI serves canned API responses and records the requests.
func fakeAPI(t *testing.T, routes map[string]string) (*Client, *[]recorded) {
	t.Helper()
	var reqs []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reqs = append(reqs, recorded{r, string(b)})
		body, ok := routes[strings.TrimPrefix(r.URL.Path, apiPath)]
		if !ok {
			body = "{}"
		}
		if r.URL.Path == apiPath+"login" {
			http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "s3cr3t"})
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.BaseURL = srv.URL
	return c, &reqs
}

func TestClientHeadersAndNotFound(t *testing.T) {
	c, reqs := fakeAPI(t, nil)
	c.Cookie = "_rj_web=abc"
	_, err := c.Media(context.Background(), KindSong, "nope")
	if err == nil || !strings.Contains(err.Error(), ErrNotFound.Error()) {
		t.Fatalf("err = %v, want not found", err)
	}
	r := (*reqs)[0]
	if r.URL.Query().Get("id") != "nope" || r.Header.Get("x-api-key") == "" || r.Header.Get("x-rj-user-agent") == "" || r.Header.Get("Cookie") != "_rj_web=abc" {
		t.Errorf("request: %s %v", r.URL, r.Header)
	}
}

func TestArtistGroupsAlbums(t *testing.T) {
	c, _ := fakeAPI(t, map[string]string{
		"artist": `{"artist":"Satin","mp3s":[
			{"id":1,"type":"mp3","song":"Single","artist":"Satin","link":"https://h/media/mp3/mp3-256/1.mp3"},
			{"id":2,"type":"mp3","song":"On Album","artist":"Satin","link":"https://h/media/mp3/mp3-256/2.mp3"}],
			"albums":[{"id":2,"type":"mp3","album":{"id":9,"album":"LP","permlink":"Satin-LP"}}],
			"videos":[{"id":5,"type":"video","song":"Clip","link":"https://h/media/music_video/hq/c.mp4"}]}`,
		"mp3": `{"id":2,"album_album":"LP","album_artist":"Satin","release_year":"2020","album_tracks":[
			{"id":2,"type":"mp3","song":"On Album","album":{"id":9,"album":"LP","track":1,"permlink":"Satin-LP"}},
			{"id":3,"type":"mp3","song":"Only On Album","album":{"id":9,"album":"LP","track":2,"permlink":"Satin-LP"}}]}`,
	})
	col, err := c.Artist(context.Background(), "satin", ArtistOptions{Videos: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Items) != 1 || col.Items[0].ID != "1" {
		t.Errorf("singles = %+v", col.Items)
	}
	if len(col.Children) != 2 || col.Children[0].Title != "LP" || len(col.Children[0].Items) != 2 || col.Children[1].Title != "Videos" {
		b, _ := json.Marshal(col.Children)
		t.Errorf("children = %s", b)
	}
	if col.Len() != 4 {
		t.Errorf("Len = %d, want 4", col.Len())
	}
}

func TestLogin(t *testing.T) {
	c, reqs := fakeAPI(t, map[string]string{"login": `{"success":true}`})
	cookie, err := c.Login(context.Background(), "a@b.c", "pw")
	if err != nil || cookie != "_rj_web=s3cr3t" {
		t.Fatalf("Login = %q, %v", cookie, err)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte((*reqs)[0].body), &body); err != nil || body["login_email"] != "a@b.c" || body["login_password"] != "pw" {
		t.Errorf("login body = %q (%v)", (*reqs)[0].body, err)
	}
	if (*reqs)[0].Method != http.MethodPost {
		t.Errorf("method = %s", (*reqs)[0].Method)
	}

	c, _ = fakeAPI(t, map[string]string{"login": `{"success":false,"msg":"The email or password doesn't look right."}`})
	if _, err := c.Login(context.Background(), "a@b.c", "bad"); err == nil || !strings.Contains(err.Error(), "doesn't look right") {
		t.Errorf("bad login error = %v", err)
	}
}

func TestAccountLists(t *testing.T) {
	c, _ := fakeAPI(t, map[string]string{
		"user_profile": `{"username":"me","display_name":"Me","subscription":false}`,
		"mp3s_liked":   `[{"id":1,"type":"mp3","song":"Liked","link":"https://h/media/mp3/mp3-256/1.mp3"}]`,
		"library": `{"success":true,"items":[
			{"item_id":"1","item_type":"mp3","item":{"id":1,"type":"mp3","song":"A","link":"https://h/media/mp3/mp3-256/1.mp3"}},
			{"item_id":"2","item_type":"podcast","item":{"id":2,"type":"podcast","title":"P","link":"https://h/media/podcast/mp3-192/2.mp3"}},
			{"item_id":"x","item_type":"artist","item":{"name":"Someone"}}]}`,
		"playlists_dash":          `{"mp3s":{"myplaylists":[{"id":"abc","title":"Mine"}]},"videos":{"myplaylists":[]}}`,
		"mp3_playlist_with_items": `{"id":"abc","title":"Mine","subtype":"mp3","items":[{"id":3,"type":"mp3","song":"S"}]}`,
	})
	ctx := context.Background()
	if _, err := c.Liked(ctx); err != ErrLoginRequired {
		t.Fatalf("anonymous Liked: %v", err)
	}
	c.Cookie = "_rj_web=x"
	liked, err := c.Liked(ctx)
	if err != nil || len(liked.Items) != 1 || liked.Items[0].Title != "Liked" {
		t.Fatalf("Liked = %+v, %v", liked, err)
	}
	lib, err := c.Library(ctx)
	if err != nil || len(lib.Items) != 2 || lib.Items[1].Kind != KindPodcast || len(lib.Warnings) != 1 {
		t.Fatalf("Library = %+v, %v", lib, err)
	}
	mine, err := c.MyPlaylists(ctx)
	if err != nil || len(mine.Children) != 1 || mine.Children[0].Title != "Mine" || len(mine.Children[0].Items) != 1 {
		t.Fatalf("MyPlaylists = %+v, %v", mine, err)
	}

	c, _ = fakeAPI(t, map[string]string{"user_profile": `{}`})
	c.Cookie = "_rj_web=expired"
	if _, err := c.Library(ctx); err != ErrSessionExpired {
		t.Errorf("expired session: %v", err)
	}
}
