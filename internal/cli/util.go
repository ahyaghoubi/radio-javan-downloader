package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MahdiGraph/radio-javan-downloader/internal/rj"
)

var unsafeChars = strings.NewReplacer(
	"/", "-", "\\", "-", ":", " -", "|", "-",
	"*", "", "?", "", "<", "", ">", "",
	`"`, "'",
)

// sanitize makes name safe to use as a file name on every common OS.
func sanitize(name string) string {
	name = unsafeChars.Replace(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case r < 32 || r == 127:
			return -1
		}
		return r
	}, name)
	name = strings.Join(strings.Fields(name), " ")
	// 255 bytes is the usual limit; keep room for suffixes and extensions.
	name = strings.Trim(truncateBytes(strings.Trim(name, " ."), 180), " .")
	if name == "" {
		return "untitled"
	}
	if isReservedName(name) {
		name = "_" + name
	}
	return name
}

// truncateBytes shortens s to at most n bytes without splitting a
// character.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// isReservedName reports names Windows does not allow for files.
func isReservedName(name string) bool {
	base := strings.ToUpper(strings.TrimSpace(strings.SplitN(name, ".", 2)[0]))
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

// shorten cuts s to at most n characters for display.
func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// parseSelection parses a list like "1-3,5 8" of 1-based positions up to
// max. The result is sorted and has no duplicates.
func parseSelection(spec string, max int) ([]int, error) {
	seen := map[int]bool{}
	for _, part := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' }) {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q", part)
		}
		b := a
		if isRange {
			if strings.TrimSpace(hi) == "" {
				b = max
			} else if b, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil {
				return nil, fmt.Errorf("invalid selection %q", part)
			}
		}
		if a < 1 || b > max || a > b {
			return nil, fmt.Errorf("selection %q is out of range 1-%d", part, max)
		}
		for i := a; i <= b; i++ {
			seen[i] = true
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("empty selection")
	}
	out := make([]int, 0, len(seen))
	for i := range seen {
		out = append(out, i)
	}
	sort.Ints(out)
	return out, nil
}

// humanBytes formats a size like "6.1 MB".
func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

// lrc renders synced lyrics in the LRC format.
func lrc(m *rj.Media) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[ti:%s]\n[ar:%s]\n", m.Title, m.Artist)
	if m.Album != nil && m.Album.Title != "" {
		fmt.Fprintf(&b, "[al:%s]\n", m.Album.Title)
	}
	if m.Duration > 0 {
		fmt.Fprintf(&b, "[length:%s]\n", lrcTime(m.Duration))
	}
	for _, l := range m.SyncedLyrics {
		fmt.Fprintf(&b, "[%s]%s\n", lrcTime(l.Time), strings.TrimSpace(l.Text))
	}
	return b.String()
}

func lrcTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	min := int(sec / 60)
	return fmt.Sprintf("%02d:%05.2f", min, sec-float64(min*60))
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Size() > 0
}

// writeFile writes data to path through a temporary file.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func marshalJSON(v any) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// displayPath shows path relative to the working directory when that is
// shorter.
func displayPath(path string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "." + string(filepath.Separator)
			}
			return rel
		}
	}
	return path
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
