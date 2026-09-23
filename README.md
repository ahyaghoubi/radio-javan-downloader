# rjdl — Radio Javan Downloader

Download songs, albums, music videos, podcasts, playlists and whole artist
catalogues from [Radio Javan](https://play.radiojavan.com), **without an
account**. Search the catalogue, save cover art and lyrics, and export clean
JSON metadata.

```text
$ rjdl https://play.radiojavan.com/album/satin-ghanune-bagha --cover --lyrics
Fetching album satin-ghanune-bagha…
Album: Satin - Ghanune Bagha (2026), 14 tracks
Saving to Satin - Ghanune Bagha (2026)
[1/14] ✓ 01. Satin - Ghanune Bagha.mp3 (6.1 MB)
[2/14] ✓ 02. Satin - Ghaza Bala.mp3 (6.7 MB)
…
[14/14] ✓ 14. Satin - Baramgardoun.mp3 (5.7 MB)
Finished: 14 downloaded (88.8 MB), 0 already there, 0 failed.
```

**[راهنمای فارسی ↓](#راهنمای-فارسی)**

## Features

- **Every kind of link:** songs, albums, music videos, podcast episodes and
  shows, audio and video playlists, artists, `rj.app` short links and old
  `radiojavan.com` links.
- **No login** for anything public. An account is only needed for your liked
  songs, library and playlists, and for RJ Premium content.
- **Best quality by default:** MP3 256 kbps with ID3 tags and 1000×1000 cover
  art built in, and videos up to 4K. AAC files and lower resolutions are one
  flag away.
- **Search** songs, albums, videos, podcasts, shows, playlists and artists.
- **JSON metadata:** English and Persian titles, album, track number, release
  date, duration, play counts, cover art, lyrics and every download link.
- **Cover art and lyrics:** the largest artwork available, and time-synced
  lyrics as `.lrc` files.
- **Reliable downloads:** several files at once, automatic retries, resuming
  after interruptions, and files you already have are skipped, so running the
  same command again finishes or updates a download.
- **Interactive mode:** run `rjdl` without arguments, then paste links or
  type a search.
- One self-contained binary for Windows, macOS and Linux.

## Install

With Go 1.24 or newer:

```bash
go install github.com/MahdiGraph/radio-javan-downloader/cmd/rjdl@latest
```

This puts `rjdl` in `$(go env GOPATH)/bin`; make sure that directory is in
your `PATH`.

Or build it from source:

```bash
git clone https://github.com/MahdiGraph/radio-javan-downloader.git
cd radio-javan-downloader
go build -o rjdl ./cmd/rjdl
```

To build a Windows executable on any system, run
`GOOS=windows GOARCH=amd64 go build -o rjdl.exe ./cmd/rjdl`. Double-clicking
`rjdl.exe` opens the interactive mode.

[ffmpeg](https://ffmpeg.org) is optional. It is only used for the rare item
that Radio Javan offers as a stream and not as a file.

## Quick start

```bash
# A song, an album, a music video, a podcast episode
rjdl https://play.radiojavan.com/song/raha-oh-nana
rjdl https://play.radiojavan.com/album/satin-ghanune-bagha
rjdl https://play.radiojavan.com/video/amir-maghare-man-delam-tange
rjdl https://play.radiojavan.com/podcast/dance-station-41

# Everything by an artist: songs, albums, and their videos too
rjdl https://play.radiojavan.com/artist/raha --include videos

# A playlist into ~/Music, with cover art and lyrics
rjdl https://play.radiojavan.com/playlist/mp3/205e3f10cd96 -o ~/Music --cover --lyrics

# Short links from the app work too
rjdl https://rj.app/m/zE1g57yl

# Search, then download a result by its link
rjdl search "shadmehr aghili"
```

## Supported links

| What | Example |
| --- | --- |
| Song | `https://play.radiojavan.com/song/raha-oh-nana` |
| Album | `https://play.radiojavan.com/album/satin-ghanune-bagha` |
| Music video | `https://play.radiojavan.com/video/amir-maghare-man-delam-tange` |
| Podcast episode | `https://play.radiojavan.com/podcast/dance-station-41` |
| Podcast show, all episodes | `https://play.radiojavan.com/podcast/show/dance-station` |
| Audio or video playlist | `https://play.radiojavan.com/playlist/mp3/205e3f10cd96`<br>`https://play.radiojavan.com/playlist/video/0a278cbb50eb` |
| Artist: songs and albums | `https://play.radiojavan.com/artist/siavash+ghomayshi` |
| Short link | `https://rj.app/m/zE1g57yl`, `https://rj.app/ma/DwrLAvp8`, … |
| Old radiojavan.com link | `https://www.radiojavan.com/mp3s/mp3/Raha-Oh-Nana` |
| Your account (needs `rjdl login`) | `liked`, `library`, `my-playlists` |

## Downloading

`rjdl <link>...` and `rjdl download <link>...` do the same thing. Several
links can be given at once.

| Flag | Default | What it does |
| --- | --- | --- |
| `-o, --output DIR` | `.` | Directory to save files in |
| `--audio-format` | `mp3` | `mp3`, `m4a` or `m4a-low`, see [Quality](#quality) |
| `--video-quality` | `best` | `best`, `2160`, `1080`, `720` or `480` |
| `-j, --jobs N` | `3` | Files downloaded at the same time (1–10) |
| `--cover` | off | Also save cover art as `.jpg` |
| `--lyrics` | off | Also save lyrics: `.lrc` when time-synced, `.txt` otherwise |
| `--write-json` | off | Also save metadata as `.json` next to each file |
| `--farsi` | off | Name files and folders with Persian titles when available |
| `--flat` | off | Put everything directly in the output directory |
| `--include` | none | For artists, also get `videos` and/or `podcasts` |
| `--dry-run` | off | List the numbered files that would be downloaded, then stop |
| `--items LIST` | all | Only download these items, e.g. `1-5,8` |
| `--links` | off | Print direct download links instead of downloading |
| `-f, --force` | off | Download again and overwrite existing files |
| `--proxy URL` | none | Send all traffic through a proxy, see [Network](#network-vpn-and-proxies) |

### Quality

Audio, with `--audio-format`:

| Value | Songs | Podcasts | Notes |
| --- | --- | --- | --- |
| `mp3` (default) | MP3 256 kbps | MP3 192 kbps | ID3 tags and cover art built in |
| `m4a` | AAC 256 kbps | AAC 192 kbps | Tagged, no built-in cover |
| `m4a-low` | AAC 128 kbps | AAC 128 kbps | Smallest files |

Video, with `--video-quality`: `best` picks 4K when a video has it and 1080p
otherwise. `2160`, `1080`, `720` and `480` pick that resolution, or the
closest lower one when it is missing.

### Where files go

Single items are saved as `Artist - Title.ext`. Albums, playlists, shows and
artists get a folder of their own:

```text
Raha/                                  ← rjdl …/artist/raha --include videos --cover
├── cover.jpg
├── Raha - Oh Nana.mp3
├── Raha - Oh Nana.jpg
├── …
├── Raha - Revival (2025)/
│   ├── cover.jpg
│   ├── 01. Raha - Raha.mp3
│   ├── 02. Raha - Jaded (Ft Parsa).mp3
│   └── …
└── Videos/
    ├── Wantons - Haminim Ke Hastim.mp4
    └── …
```

With `--write-json`, every file gets a `.json` twin and every folder an
`info.json`. `--flat` drops the folders, and `--farsi` uses Persian names such
as `رها - اووه نه نه نه.mp3`.

### Picking items from a big collection

```bash
rjdl https://play.radiojavan.com/artist/ebi --dry-run      # numbered list, nothing is downloaded
rjdl https://play.radiojavan.com/artist/ebi --items 1-10,25
```

### Direct links only

`--links` prints one direct link per file instead of downloading, for use in
another download manager:

```bash
rjdl https://play.radiojavan.com/playlist/mp3/205e3f10cd96 --links > links.txt
```

## Search

```text
$ rjdl search ebi --limit 2
Artists
   1. Ebi
      https://play.radiojavan.com/artist/ebi
   2. Eti
      https://play.radiojavan.com/artist/eti

Songs
   3. Ebi - Gheseh Eshgh
      https://play.radiojavan.com/song/ebi-gheseh-eshgh
…
```

`--type songs|albums|videos|podcasts|shows|playlists|artists` limits the
results to one type (20 by default, change it with `--limit`), and `--json`
prints them as JSON. Every result comes with a link for `rjdl`, `rjdl info`
or `rjdl cover`.

## Metadata as JSON

`rjdl info <link>` prints what Radio Javan knows about an item or a
collection:

```text
$ rjdl info https://play.radiojavan.com/song/raha-oh-nana
```

```json
{
  "type": "song",
  "id": "159372",
  "permlink": "Raha-Oh-Nana",
  "url": "https://play.radiojavan.com/song/raha-oh-nana",
  "share_url": "https://rj.app/m/zE1g57yl",
  "title": "Oh Nana",
  "artist": "Raha",
  "title_fa": "اووه نه نه نه",
  "artist_fa": "رها",
  "duration": 182.518,
  "date": "2026-09-22T12:30:00-04:00",
  "explicit": false,
  "premium_only": false,
  "stats": { "plays": 17155, "likes": 135, "downloads": 17155 },
  "cover": {
    "large": "https://assets.rjassets.com/static/mp3/raha-oh-nana/d138bfe179ff6ac.jpg",
    "player": "https://assets.rjassets.com/static/mp3/raha-oh-nana/8854d320cec6865-player.jpg",
    "thumbnail": "https://assets.rjassets.com/static/mp3/raha-oh-nana/d138bfe179ff6ac-thumb.jpg"
  },
  "formats": [
    { "id": "mp3-256", "ext": "mp3", "bitrate": 256, "url": "https://host2.media-rj.com/media/mp3/mp3-256/159372-a39efd65192438e.mp3" },
    { "id": "aac-256", "ext": "m4a", "bitrate": 256, "url": "https://host2.media-rj.com/media/mp3/aac-256/159372-a39efd65192438e.m4a" },
    { "id": "aac-128", "ext": "m4a", "bitrate": 128, "url": "https://host2.media-rj.com/media/mp3/aac-128/159372-a39efd65192438e.m4a" },
    { "id": "hls", "ext": "m3u8", "url": "https://host2.media-rj.com/media/mp3/mp3-hls/159372-a39efd65192438e/playlist.m3u8" }
  ],
  "lyrics": "…",
  "synced_lyrics": [{ "time": 17.57, "text": "…" }]
}
```

Songs on an album also have an `album` object (title, artist, track number,
year, link), podcasts have `show` and `tracklist`, and videos list MP4 files
by resolution (`mp4-2160p`, `mp4-1080p`, `mp4-720p`, …). Albums, playlists,
shows and artists are printed with their `items`, and artists with their
albums as `children`. Some examples with [jq](https://jqlang.org):

```bash
rjdl info https://play.radiojavan.com/album/satin-ghanune-bagha | jq -r '.items[] | "\(.album.track). \(.title)"'
rjdl info https://rj.app/p/d840nN8p | jq -r .tracklist
rjdl info https://play.radiojavan.com/video/puzzle-harfe-daregooshi | jq -r '.formats[].id'
```

`--raw` prints the API response exactly as Radio Javan sends it.

## Cover art

```bash
rjdl cover https://play.radiojavan.com/song/raha-oh-nana            # Raha - Oh Nana.jpg
rjdl cover https://play.radiojavan.com/playlist/mp3/205e3f10cd96 --all -o covers
```

Covers are saved in the largest size Radio Javan has (1000×1000 for songs and
podcasts). With `--all`, the covers of every item of a collection are saved
too. When downloading, `--cover` does the same next to the files.

## Your account (optional)

Everything public works without logging in. A session is only needed for:

| | |
| --- | --- |
| `rjdl liked` | Your liked songs |
| `rjdl library` | The songs, videos and podcasts in your library |
| `rjdl my-playlists` | The playlists you made, including private ones |
| RJ Premium content | Items marked Premium, with a Premium subscription |

```bash
rjdl login                 # asks for your email and password
rjdl whoami                # shows the account and whether it has RJ Premium
rjdl liked -o ~/Music/RJ   # downloads your liked songs
rjdl logout
```

**Account without a password?** If you sign in with an email code, Google or
Apple, log in on [play.radiojavan.com](https://play.radiojavan.com), open the
browser's developer tools (F12) → Application (Storage in Firefox) → Cookies →
`https://play.radiojavan.com`, copy the value of the `_rj_web` cookie, and
run:

```bash
rjdl login --cookie 'PASTE-THE-VALUE-HERE'
```

The session is saved with permissions only you can read, in
`~/.config/rjdl/session.json` on Linux, `~/Library/Application
Support/rjdl/session.json` on macOS and `%AppData%\rjdl\session.json` on
Windows. For scripts, the `RJDL_SESSION` environment variable can hold the
cookie value instead. Your password is sent only to Radio Javan and is never
stored.

## Network, VPN and proxies

rjdl needs to reach `play.radiojavan.com` and Radio Javan's download servers.
Where Radio Javan is blocked (you will see
`your network blocks Radio Javan: use a VPN or --proxy`), use a VPN or point
rjdl at a proxy:

```bash
rjdl --proxy socks5://127.0.0.1:1080 https://play.radiojavan.com/song/raha-oh-nana
rjdl --proxy http://127.0.0.1:8080 search ebi
```

`--proxy` works with every command and accepts `http`, `https`, `socks5` and
`socks5h` URLs. Without it, the usual `HTTPS_PROXY` and `HTTP_PROXY`
environment variables are used.

## Interactive mode

Running `rjdl` without arguments (or double-clicking `rjdl.exe`) starts a
prompt. Paste one or more links to download them, or type anything else to
search and then pick results by number. Type `help` for help and `0` to quit.
Flags still apply, e.g. `rjdl -o ~/Music --cover`.

```text
> satin
Artists
   1. Satin
      https://play.radiojavan.com/artist/satin
Songs
   3. Satin - Zibaye Mani
      https://play.radiojavan.com/song/satin-zibaye-mani
…
Download which? (numbers like 1 3-5, Enter to skip): 3
```

## How it works

rjdl talks to the same JSON API that the play.radiojavan.com web player uses,
so it sees exactly what the site shows: song, video and podcast details come
with direct links to Radio Javan's download servers, and albums, playlists,
shows and artists come with their track lists. Short links are resolved by
Radio Javan itself. Files are downloaded straight from Radio Javan's servers
into a `.part` file that is resumed if the connection drops and renamed when
complete.

## Development

```text
cmd/rjdl/          entry point
internal/rj/       Radio Javan API client: links, media, collections, search, account
internal/download/ HTTP downloads with resume and retries, ffmpeg for streams
internal/cli/      commands, file naming, progress display, interactive mode
```

```bash
go test ./...
go vet ./...
```

## Credits and disclaimer

This project started as a fork of
[ahyaghoubi/radio-javan-downloader](https://github.com/ahyaghoubi/radio-javan-downloader).
It is not affiliated with Radio Javan. Use it for personal listening, respect
the rights of artists and Radio Javan's terms, and consider supporting the
artists you love and [RJ Premium](https://play.radiojavan.com).

---

<div dir="rtl">

## راهنمای فارسی

**rjdl** ابزاری خط‌فرمانی برای دانلود از رادیو جوان است: آهنگ، آلبوم، موزیک‌ویدیو، پادکست، شوی پادکست، پلی‌لیست و همه آثار یک خواننده؛ **بدون نیاز به حساب کاربری**. جستجو، دانلود کاور، متن آهنگ (فایل LRC همگام) و خروجی JSON از اطلاعات آهنگ هم دارد.

### نصب

با Go نسخه ۱.۲۴ یا بالاتر:

</div>

```bash
go install github.com/MahdiGraph/radio-javan-downloader/cmd/rjdl@latest
```

<div dir="rtl">

یا از سورس بسازید: `go build -o rjdl ./cmd/rjdl` (برای ویندوز: `rjdl.exe`). اگر `rjdl.exe` را با دوبار کلیک باز کنید، حالت تعاملی اجرا می‌شود: لینک را بچسبانید یا اسم آهنگ/خواننده را بنویسید تا جستجو شود.

### نمونه‌ها

</div>

```bash
rjdl https://play.radiojavan.com/song/raha-oh-nana                  # یک آهنگ
rjdl https://play.radiojavan.com/album/satin-ghanune-bagha --cover  # آلبوم کامل با کاور
rjdl https://play.radiojavan.com/artist/raha --include videos       # همه آهنگ‌ها، آلبوم‌ها و ویدیوهای خواننده
rjdl https://play.radiojavan.com/podcast/show/dance-station         # همه قسمت‌های یک پادکست
rjdl https://rj.app/m/zE1g57yl --lyrics                             # لینک کوتاه + متن آهنگ
rjdl search "siavash ghomayshi" --type albums                       # جستجو
rjdl info https://play.radiojavan.com/song/raha-oh-nana             # اطلاعات کامل به صورت JSON
rjdl cover https://play.radiojavan.com/song/raha-oh-nana            # فقط کاور
rjdl https://play.radiojavan.com/video/amir-maghare-man-delam-tange --video-quality 720
rjdl https://play.radiojavan.com/song/raha-oh-nana --farsi          # نام فایل فارسی
```

<div dir="rtl">

### ورود به حساب (اختیاری)

فقط برای لایک‌ها (`rjdl liked`)، کتابخانه (`rjdl library`)، پلی‌لیست‌های خودتان (`rjdl my-playlists`) و محتوای پریمیوم لازم است. با `rjdl login` ایمیل و رمز را وارد کنید. اگر حسابتان رمز ندارد (ورود با کد ایمیلی یا گوگل)، مقدار کوکی `_rj_web` را از مرورگر بردارید و `rjdl login --cookie '...'` را اجرا کنید (توضیح کامل در بخش انگلیسی بالا).

### فیلترینگ و پروکسی

اگر پیام `your network blocks Radio Javan` را دیدید، VPN را روشن کنید یا از پروکسی استفاده کنید:

</div>

```bash
rjdl --proxy socks5://127.0.0.1:1080 https://play.radiojavan.com/song/raha-oh-nana
```
