// Command rjdl downloads songs, albums, videos, podcasts and playlists from
// Radio Javan.
package main

import (
	"os"

	"github.com/ahyaghoubi/radio-javan-downloader/v2/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
