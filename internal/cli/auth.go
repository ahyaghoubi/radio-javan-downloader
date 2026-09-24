package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

// sessionEnv overrides the saved session, e.g. for scripts.
const sessionEnv = "RJDL_SESSION"

type session struct {
	Cookie   string    `json:"cookie"`
	Username string    `json:"username,omitempty"`
	SavedAt  time.Time `json:"saved_at"`
}

func sessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rjdl", "session.json"), nil
}

// loadSession returns the session from $RJDL_SESSION or the session file,
// or nil when there is none.
func loadSession() (*session, error) {
	if v := os.Getenv(sessionEnv); v != "" {
		return &session{Cookie: rj.SessionHeader(v)}, nil
	}
	p, err := sessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s session
	if err := json.Unmarshal(data, &s); err != nil || s.Cookie == "" {
		return nil, fmt.Errorf("%s is damaged; run rjdl login again", p)
	}
	return &s, nil
}

func saveSession(s *session) (string, error) {
	p, err := sessionPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

func (a *app) loginCmd() *cobra.Command {
	var email, cookie string
	var passwordStdin bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Radio Javan",
		Long: `Log in to Radio Javan. This is optional: everything public works without an
account. A session is needed for:

  rjdl liked          your liked songs
  rjdl library        your library
  rjdl my-playlists   your playlists (also private ones)
  RJ Premium content  (with a Premium subscription)

By default you are asked for your email and password. Accounts without a
password (signed up with an email code, Google or Apple) can use a session
from the browser instead: log in on play.radiojavan.com, open the developer
tools (F12) > Application/Storage > Cookies > https://play.radiojavan.com and
copy the value of the "_rj_web" cookie, then run:

  rjdl login --cookie <value>

The session is saved in your user config directory, readable only by you.
The RJDL_SESSION environment variable can be used instead of a saved session.`,
		Example: `  rjdl login
  rjdl login --email you@example.com
  rjdl login --cookie 'eyJ...'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := withInterrupt(cmd.Context())
			defer stop()
			var header string
			if cookie != "" {
				header = rj.SessionHeader(cookie)
			} else {
				in := bufio.NewReader(a.stdin)
				if email == "" {
					var err error
					if email, err = a.ask(in, "Email: "); err != nil {
						return err
					}
				}
				password, err := a.readPassword(in, passwordStdin)
				if err != nil {
					return err
				}
				if email == "" || password == "" {
					return errors.New("email and password are required")
				}
				if header, err = a.client.Login(ctx, email, password); err != nil {
					return fmt.Errorf("login failed: %w", err)
				}
			}
			a.client.Cookie = header
			prof, err := a.client.Profile(ctx)
			if err != nil {
				if errors.Is(err, rj.ErrSessionExpired) {
					return errors.New("Radio Javan did not accept this session")
				}
				return err
			}
			path, err := saveSession(&session{Cookie: header, Username: prof.Username, SavedAt: time.Now().UTC()})
			if err != nil {
				return fmt.Errorf("logged in, but the session could not be saved: %w", err)
			}
			fmt.Fprintf(a.stdout, "Logged in as %s (@%s).\n", prof.DisplayName, prof.Username)
			if prof.Premium {
				fmt.Fprintln(a.stdout, "RJ Premium: yes")
			}
			fmt.Fprintf(a.stdout, "Session saved to %s\n", path)
			if os.Getenv(sessionEnv) != "" {
				a.warnf("%s is set and takes precedence over the saved session", sessionEnv)
			}
			return nil
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&email, "email", "e", "", "email address of your account")
	fs.StringVar(&cookie, "cookie", "", "use a session copied from the browser (value of the _rj_web cookie)")
	fs.BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input")
	return cmd
}

func (a *app) ask(in *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(a.stderr, prompt)
	line, err := in.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a *app) readPassword(in *bufio.Reader, fromStdin bool) (string, error) {
	if f, ok := a.stdin.(*os.File); ok && !fromStdin && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(a.stderr, "Password: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.stderr)
		return string(b), err
	}
	line, err := in.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (a *app) logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out and delete the saved session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := sessionPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
				fmt.Fprintln(a.stdout, "Not logged in.")
				return nil
			}
			ctx, stop := withInterrupt(cmd.Context())
			defer stop()
			if s, _ := loadSession(); s != nil && os.Getenv(sessionEnv) == "" {
				a.client.Cookie = s.Cookie
				_ = a.client.Logout(ctx) // best effort; the local session goes anyway
			}
			if err := os.Remove(p); err != nil {
				return err
			}
			fmt.Fprintln(a.stdout, "Logged out.")
			return nil
		},
	}
}

func (a *app) whoamiCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the logged-in account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !a.client.LoggedIn() {
				fmt.Fprintln(a.stdout, "Not logged in. Run rjdl login to log in.")
				return errSilent
			}
			ctx, stop := withInterrupt(cmd.Context())
			defer stop()
			prof, err := a.client.Profile(ctx)
			if err != nil {
				return err
			}
			if asJSON {
				data, err := marshalJSON(prof)
				if err != nil {
					return err
				}
				_, err = a.stdout.Write(data)
				return err
			}
			fmt.Fprintf(a.stdout, "Logged in as %s (@%s)\n", prof.DisplayName, prof.Username)
			if prof.Email != "" {
				fmt.Fprintf(a.stdout, "Email:      %s\n", prof.Email)
			}
			premium := "no"
			if prof.Premium {
				premium = "yes"
			}
			fmt.Fprintf(a.stdout, "RJ Premium: %s\n", premium)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the account as JSON")
	return cmd
}
