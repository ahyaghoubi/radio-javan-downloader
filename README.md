# rjdl

[![Build](https://github.com/MahdiGraph/radio-javan-downloader/actions/workflows/build.yml/badge.svg)](https://github.com/MahdiGraph/radio-javan-downloader/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/MahdiGraph/radio-javan-downloader)](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest)

A command-line downloader for [Radio Javan](https://play.radiojavan.com):
songs, albums, music videos, podcasts, playlists and whole artists. No
account needed.

**[راهنمای فارسی](README.fa.md)**

```text
$ rjdl https://play.radiojavan.com/album/raha-revival
Fetching album raha-revival…
Album: Raha - Revival (2025), 8 tracks
Saving to Raha - Revival (2025)
[1/8] ✓ 01. Raha - Raha.mp3 (6.5 MB)
[2/8] ✓ 02. Raha - Jaded (Ft Parsa).mp3 (5.5 MB)
…
Finished: 8 downloaded (47.1 MB), 0 already there, 0 failed.
```

## Features

- **Any link:** songs, albums, music videos, podcast episodes and shows,
  playlists, artists, `rj.app` short links and old `radiojavan.com` links.
- **No account needed.** Log in only for your likes, library and playlists,
  or for RJ Premium content.
- **Best quality:** MP3 256 kbps with tags and cover art, videos up to 4K.
  AAC and lower resolutions on request.
- **Search**, **metadata as JSON**, **cover art** and **time-synced lyrics**.
- **Safe to re-run:** parallel downloads, interrupted downloads resume, and
  files you already have are skipped.
- **Interactive mode:** run `rjdl` without arguments, or double-click it on
  Windows.

## Install

Download the build for your system and unpack it:

| System | Download |
| --- | --- |
| Windows | [64-bit](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_windows_amd64.zip) · [ARM](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_windows_arm64.zip) |
| macOS | [Apple Silicon](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_darwin_arm64.tar.gz) · [Intel](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_darwin_amd64.tar.gz) |
| Linux | [x86-64](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_linux_amd64.tar.gz) · [ARM64](https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_linux_arm64.tar.gz) |

- **Windows:** double-click `rjdl.exe` for the interactive mode, or run it
  in a terminal. If SmartScreen warns about an unrecognized app, choose
  *More info* → *Run anyway*.
- **macOS and Linux:** move `rjdl` to a folder in your `PATH`. If macOS
  refuses to open it, run `xattr -d com.apple.quarantine rjdl` once.

Or, with Go 1.24 or newer:

```bash
go install github.com/MahdiGraph/radio-javan-downloader/cmd/rjdl@latest
```

## Usage

```text
rjdl <link>...           download
rjdl search <query>      search
rjdl info <link>         print metadata as JSON
rjdl cover <link>        download cover art
rjdl login               log in (optional)
rjdl                     interactive mode
```

```bash
rjdl https://play.radiojavan.com/song/raha-oh-nana
rjdl https://play.radiojavan.com/album/satin-ghanune-bagha --cover --lyrics
rjdl https://play.radiojavan.com/video/amir-maghare-man-delam-tange --video-quality 1080
rjdl https://play.radiojavan.com/podcast/show/dance-station
rjdl https://play.radiojavan.com/artist/raha --include videos -o ~/Music
rjdl https://rj.app/m/zE1g57yl
```

Albums, playlists, shows and artists get a folder of their own:

```text
Raha/
├── Raha - Oh Nana.mp3
├── Raha - Revival (2025)/
│   ├── 01. Raha - Raha.mp3
│   └── 02. Raha - Jaded (Ft Parsa).mp3
└── Videos/
    └── Wantons - Haminim Ke Hastim.mp4
```

Running the same command again finishes an interrupted download and only
fetches what is new.

### Options

| Option | Description |
| --- | --- |
| `-o, --output DIR` | Where to save files (default: the current folder) |
| `--audio-format F` | `mp3` 256 kbps (default), `m4a` AAC 256 kbps or `m4a-low` AAC 128 kbps; podcasts are 192 kbps |
| `--video-quality Q` | `best` (default, up to 4K), `2160`, `1080`, `720` or `480` |
| `--cover` | Also save cover art |
| `--lyrics` | Also save lyrics, as `.lrc` when time-synced |
| `--write-json` | Also save metadata as `.json` |
| `--farsi` | Name files with Persian titles |
| `--include videos,podcasts` | For artists, also get their videos and podcasts |
| `--dry-run` | List the numbered files without downloading |
| `--items 1-5,8` | Download only these items of the list |
| `--links` | Print direct download links instead |
| `-j, --jobs N` | Files downloaded at once (default 3) |
| `--flat` | Don't create album and playlist folders |
| `-f, --force` | Download again and overwrite existing files |
| `--proxy URL` | Use a proxy, e.g. `socks5://127.0.0.1:1080` |

### Supported links

| Type | Example |
| --- | --- |
| Song | `play.radiojavan.com/song/raha-oh-nana` |
| Album | `play.radiojavan.com/album/satin-ghanune-bagha` |
| Music video | `play.radiojavan.com/video/amir-maghare-man-delam-tange` |
| Podcast | `play.radiojavan.com/podcast/dance-station-41` |
| Podcast show | `play.radiojavan.com/podcast/show/dance-station` |
| Playlist | `play.radiojavan.com/playlist/mp3/205e3f10cd96`, `…/playlist/video/…` |
| Artist | `play.radiojavan.com/artist/siavash+ghomayshi` |
| Short link | `rj.app/m/zE1g57yl` |
| Old website | `radiojavan.com/mp3s/mp3/Raha-Oh-Nana` |
| Your account | `liked`, `library`, `my-playlists` (after `rjdl login`) |

## Search and metadata

```bash
rjdl search "siavash ghomayshi"
rjdl search ebi --type albums --limit 10
rjdl info https://play.radiojavan.com/song/raha-oh-nana
```

`rjdl info` prints English and Persian titles, artist, album and track
number, release date, duration, play counts, cover art, lyrics and the direct
link of every available file; `--raw` prints Radio Javan's own response.
It works well with [jq](https://jqlang.org):

```bash
rjdl info https://play.radiojavan.com/album/satin-ghanune-bagha | jq -r '.items[] | "\(.album.track). \(.title)"'
```

## Account

Nothing public needs an account. Log in to download your liked songs,
library and playlists, or RJ Premium content if you have a subscription:

```bash
rjdl login
rjdl whoami
rjdl liked
rjdl library
rjdl my-playlists
rjdl logout
```

For an account without a password (email code, Google or Apple sign-in), log
in on play.radiojavan.com, copy the value of the `_rj_web` cookie from the
browser's developer tools (Application → Cookies), then run
`rjdl login --cookie VALUE`.

The session is saved in your user config folder, readable only by you. Your
password is never saved.

## Blocked network

If you see `your network blocks Radio Javan`, turn on a VPN or use a proxy.
`HTTPS_PROXY` and `HTTP_PROXY` are respected as well.

```bash
rjdl --proxy socks5://127.0.0.1:1080 https://play.radiojavan.com/song/raha-oh-nana
```

## Building

```bash
go build -o rjdl ./cmd/rjdl   # this computer
scripts/build.sh              # every platform, into dist/
go test ./...
```

GitHub Actions tests and builds every push. Pushing a tag such as `v1.2.0`
publishes a release with the binaries.

## Credits

Started as a fork of
[ahyaghoubi/radio-javan-downloader](https://github.com/ahyaghoubi/radio-javan-downloader).
Not affiliated with Radio Javan. For personal use; please respect the rights
of the artists and Radio Javan's terms.
