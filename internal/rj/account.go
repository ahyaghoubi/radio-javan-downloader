package rj

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ErrSessionExpired is returned when the saved session is not accepted.
var ErrSessionExpired = errors.New("the saved Radio Javan session is no longer valid: run `rjdl login` again")

// Profile is the logged-in user.
type Profile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	Premium     bool   `json:"premium"`
}

type rawProfile struct {
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Subscription bool   `json:"subscription"`
}

// Profile returns the logged-in user. It fails with ErrLoginRequired when
// the client has no session and ErrSessionExpired when the session is not
// accepted anymore.
func (c *Client) Profile(ctx context.Context) (*Profile, error) {
	if !c.LoggedIn() {
		return nil, ErrLoginRequired
	}
	var r rawProfile
	if err := c.getJSON(ctx, "user_profile", nil, &r); err != nil {
		return nil, err
	}
	if r.Username == "" {
		return nil, ErrSessionExpired
	}
	return &Profile{
		Username:    r.Username,
		DisplayName: firstNonEmpty(r.DisplayName, r.Name, r.Username),
		Email:       r.Email,
		Premium:     r.Subscription,
	}, nil
}

// Login signs in with an email address and password and returns the
// session as a Cookie header value (see SessionHeader).
func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	anon := *c
	anon.Cookie = ""
	resp, _, err := anon.call(ctx, http.MethodPost, "login", nil, map[string]string{
		"login_email":    email,
		"login_password": password,
	})
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusOK {
			return "", errors.New(apiErr.Message)
		}
		return "", err
	}
	var parts []string
	for _, ck := range resp.Cookies() {
		if ck.Value != "" && ck.MaxAge >= 0 {
			parts = append(parts, ck.Name+"="+ck.Value)
		}
	}
	if len(parts) == 0 {
		return "", errors.New("login was accepted but Radio Javan did not return a session cookie")
	}
	return strings.Join(parts, "; "), nil
}

// Logout ends the session on the server.
func (c *Client) Logout(ctx context.Context) error {
	if !c.LoggedIn() {
		return nil
	}
	_, _, err := c.call(ctx, http.MethodGet, "logout", nil, nil)
	return err
}

// Liked returns the logged-in user's liked songs.
func (c *Client) Liked(ctx context.Context) (*Collection, error) {
	if _, err := c.Profile(ctx); err != nil {
		return nil, err
	}
	var items []rawItem
	if err := c.getJSON(ctx, "mp3s_liked", nil, &items); err != nil {
		return nil, err
	}
	col := &Collection{Kind: KindLiked, Title: "Liked Songs", URL: SiteURL + "/library/liked/songs"}
	for i := range items {
		col.Items = append(col.Items, items[i].media())
	}
	return col, nil
}

type rawLibrary struct {
	Items []struct {
		ItemType string  `json:"item_type"`
		Item     rawItem `json:"item"`
	} `json:"items"`
}

func (c *Client) libraryRaw(ctx context.Context) (json.RawMessage, error) {
	_, data, err := c.call(ctx, http.MethodPost, "library", url.Values{"full": {"1"}}, nil)
	return json.RawMessage(data), err
}

// Library returns the songs, videos and podcasts saved in the logged-in
// user's library.
func (c *Client) Library(ctx context.Context) (*Collection, error) {
	if _, err := c.Profile(ctx); err != nil {
		return nil, err
	}
	data, err := c.libraryRaw(ctx)
	if err != nil {
		return nil, err
	}
	var r rawLibrary
	if err := decode("library", data, &r); err != nil {
		return nil, err
	}
	col := &Collection{Kind: KindLibrary, Title: "Library", URL: SiteURL + "/library"}
	skipped := 0
	for i := range r.Items {
		switch r.Items[i].ItemType {
		case "mp3", "video", "podcast":
			col.Items = append(col.Items, r.Items[i].Item.media())
		default:
			skipped++
		}
	}
	if skipped > 0 {
		col.Warnings = append(col.Warnings, fmt.Sprintf("skipped %d library entries that are not songs, videos or podcasts", skipped))
	}
	return col, nil
}

type rawPlaylistsDash struct {
	MP3s struct {
		MyPlaylists []rawPlaylist `json:"myplaylists"`
	} `json:"mp3s"`
	Videos struct {
		MyPlaylists []rawPlaylist `json:"myplaylists"`
	} `json:"videos"`
}

// MyPlaylists returns the playlists the logged-in user created, each with
// its items.
func (c *Client) MyPlaylists(ctx context.Context) (*Collection, error) {
	if _, err := c.Profile(ctx); err != nil {
		return nil, err
	}
	var r rawPlaylistsDash
	if err := c.getJSON(ctx, "playlists_dash", nil, &r); err != nil {
		return nil, err
	}
	col := &Collection{Kind: KindMyPlaylists, Title: "My Playlists"}
	load := func(subtype string, lists []rawPlaylist) {
		for _, p := range lists {
			if p.ID == "" {
				continue
			}
			pl, err := c.Playlist(ctx, subtype, string(p.ID))
			if err != nil {
				col.Warnings = append(col.Warnings, fmt.Sprintf("playlist %q: %v", p.Title, err))
				continue
			}
			col.Children = append(col.Children, pl)
		}
	}
	load("mp3", r.MP3s.MyPlaylists)
	load("video", r.Videos.MyPlaylists)
	return col, nil
}
