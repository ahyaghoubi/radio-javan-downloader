# rjdl

**[English](README.md)**

دانلودر خط فرمان رادیو جوان: آهنگ، آلبوم، موزیک ویدیو، پادکست، پلی‌لیست و همه آثار یک خواننده را دانلود می‌کند، بدون نیاز به حساب کاربری.

## امکانات

- پشتیبانی از همه نوع لینک: آهنگ، آلبوم، ویدیو، پادکست، شوی پادکست، پلی‌لیست، صفحه خواننده، لینک‌های کوتاه اپلیکیشن و لینک‌های سایت قدیمی
- بدون نیاز به ورود؛ حساب کاربری فقط برای لایک‌ها، کتابخانه، پلی‌لیست‌های شخصی و محتوای پریمیوم لازم است
- بهترین کیفیت موجود: MP3 با کیفیت ۲۵۶ همراه کاور و مشخصات آهنگ، و ویدیو تا کیفیت 4K
- جستجو، خروجی اطلاعات کامل به صورت JSON، دانلود کاور و متن همگام آهنگ‌ها
- دانلود همزمان چند فایل؛ اگر دانلود قطع شود، اجرای دوباره همان دستور آن را ادامه می‌دهد و فایل‌های موجود دوباره دانلود نمی‌شوند
- حالت تعاملی: برنامه را بدون ورودی اجرا کنید یا در ویندوز روی آن دوبار کلیک کنید، سپس لینک را بچسبانید یا اسم آهنگ و خواننده را بنویسید

## نصب

فایل مناسب سیستم خود را دانلود کنید و از حالت فشرده خارج کنید:

<table dir="rtl">
<tr><th>سیستم</th><th>دانلود</th></tr>
<tr><td>ویندوز</td><td><a href="https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_windows_amd64.zip">۶۴ بیتی</a> · <a href="https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_windows_arm64.zip">ARM</a></td></tr>
<tr><td>مک</td><td><a href="https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_darwin_arm64.tar.gz">Apple Silicon</a> · <a href="https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_darwin_amd64.tar.gz">Intel</a></td></tr>
<tr><td>لینوکس</td><td><a href="https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_linux_amd64.tar.gz">x86-64</a> · <a href="https://github.com/MahdiGraph/radio-javan-downloader/releases/latest/download/rjdl_linux_arm64.tar.gz">ARM64</a></td></tr>
</table>

در ویندوز با دوبار کلیک روی فایل `rjdl.exe` حالت تعاملی باز می‌شود. اگر ویندوز هشدار امنیتی نشان داد، روی **More info** و بعد **Run anyway** بزنید.

در مک اگر سیستم اجازه اجرا نداد، این دستور را یک بار در ترمینال اجرا کنید:

```bash
xattr -d com.apple.quarantine rjdl
```

اگر Go نسخه ۱.۲۴ یا جدیدتر دارید، می‌توانید مستقیم نصب کنید:

```bash
go install github.com/MahdiGraph/radio-javan-downloader/cmd/rjdl@latest
```

## استفاده

کافی است لینک را جلوی `rjdl` بنویسید.

دانلود یک آهنگ:

```bash
rjdl https://play.radiojavan.com/song/raha-oh-nana
```

دانلود آلبوم کامل همراه کاور و متن آهنگ‌ها:

```bash
rjdl https://play.radiojavan.com/album/satin-ghanune-bagha --cover --lyrics
```

دانلود موزیک ویدیو با کیفیت ۱۰۸۰:

```bash
rjdl https://play.radiojavan.com/video/amir-maghare-man-delam-tange --video-quality 1080
```

دانلود همه قسمت‌های یک پادکست:

```bash
rjdl https://play.radiojavan.com/podcast/show/dance-station
```

دانلود همه آهنگ‌ها، آلبوم‌ها و ویدیوهای یک خواننده در پوشه موزیک:

```bash
rjdl https://play.radiojavan.com/artist/raha --include videos -o ~/Music
```

جستجو:

```bash
rjdl search "siavash ghomayshi"
```

نمایش اطلاعات کامل یک آهنگ به صورت JSON:

```bash
rjdl info https://play.radiojavan.com/song/raha-oh-nana
```

دانلود کاور:

```bash
rjdl cover https://play.radiojavan.com/album/ebi-koohe-yakh
```

هر آلبوم، پلی‌لیست، پادکست و خواننده در پوشه جداگانه‌ای ذخیره می‌شود و ترک‌های آلبوم شماره‌گذاری می‌شوند. لینک‌های کوتاه اپلیکیشن و لینک‌های سایت قدیمی رادیو جوان هم پشتیبانی می‌شوند.

## گزینه‌ها

<table dir="rtl">
<tr><th>گزینه</th><th>کاربرد</th></tr>
<tr><td><code dir="ltr">-o DIR</code></td><td>پوشه ذخیره فایل‌ها؛ پیش‌فرض پوشه فعلی است</td></tr>
<tr><td><code dir="ltr">--audio-format m4a</code></td><td>صدا با فرمت AAC به جای MP3 که پیش‌فرض است</td></tr>
<tr><td><code dir="ltr">--audio-format m4a-low</code></td><td>صدا با کمترین حجم</td></tr>
<tr><td><code dir="ltr">--video-quality 1080</code></td><td>کیفیت ویدیو: 2160 یا 1080 یا 720 یا 480؛ پیش‌فرض بهترین کیفیت موجود است</td></tr>
<tr><td><code dir="ltr">--cover</code></td><td>ذخیره کاور کنار فایل‌ها</td></tr>
<tr><td><code dir="ltr">--lyrics</code></td><td>ذخیره متن آهنگ؛ متن‌های همگام با فرمت LRC</td></tr>
<tr><td><code dir="ltr">--write-json</code></td><td>ذخیره اطلاعات کامل هر آهنگ در فایل JSON</td></tr>
<tr><td><code dir="ltr">--farsi</code></td><td>نام‌گذاری فایل‌ها با عنوان فارسی</td></tr>
<tr><td><code dir="ltr">--include videos,podcasts</code></td><td>دانلود ویدیوها و پادکست‌های خواننده همراه آهنگ‌ها</td></tr>
<tr><td><code dir="ltr">--dry-run</code></td><td>نمایش فهرست شماره‌دار فایل‌ها بدون دانلود</td></tr>
<tr><td><code dir="ltr">--items 1-5,8</code></td><td>دانلود فقط موارد انتخابی از همان فهرست</td></tr>
<tr><td><code dir="ltr">--links</code></td><td>نمایش لینک مستقیم فایل‌ها به جای دانلود</td></tr>
<tr><td><code dir="ltr">-j 5</code></td><td>تعداد دانلود همزمان؛ پیش‌فرض ۳ است</td></tr>
<tr><td><code dir="ltr">--flat</code></td><td>ذخیره همه فایل‌ها در یک پوشه، بدون پوشه آلبوم و پلی‌لیست</td></tr>
<tr><td><code dir="ltr">-f</code></td><td>دانلود دوباره و جایگزینی فایل‌های موجود</td></tr>
<tr><td><code dir="ltr">--proxy URL</code></td><td>استفاده از پروکسی</td></tr>
</table>

فهرست کامل گزینه‌ها با این دستور نمایش داده می‌شود:

```bash
rjdl --help
```

## حساب کاربری

برای دانلودهای عمومی نیازی به ورود نیست. برای دانلود آهنگ‌های لایک‌شده، کتابخانه، پلی‌لیست‌های شخصی یا محتوای پریمیوم، یک بار وارد شوید:

```bash
rjdl login
```

بعد از ورود این دستورها کار می‌کنند:

```bash
rjdl whoami
rjdl liked
rjdl library
rjdl my-playlists
```

اگر حساب شما رمز عبور ندارد و با کد ایمیلی یا گوگل وارد می‌شوید، در سایت رادیو جوان وارد شوید، از بخش ابزار توسعه‌دهنده مرورگر مقدار کوکی <code dir="ltr">_rj_web</code> را کپی کنید و آن را این‌طور به برنامه بدهید:

```bash
rjdl login --cookie COOKIE_VALUE
```

اطلاعات ورود فقط روی کامپیوتر خودتان ذخیره می‌شود و رمز عبور هیچ‌جا ذخیره نمی‌شود.

## اینترنت فیلترشده

اگر پیام `your network blocks Radio Javan` را دیدید، VPN را روشن کنید یا از پروکسی استفاده کنید:

```bash
rjdl --proxy socks5://127.0.0.1:1080 https://play.radiojavan.com/song/raha-oh-nana
```

## ساخت از سورس

```bash
go build -o rjdl ./cmd/rjdl
```

---

این پروژه در ابتدا از [ahyaghoubi/radio-javan-downloader](https://github.com/ahyaghoubi/radio-javan-downloader) فورک شده و ارتباطی با رادیو جوان ندارد. لطفاً فقط برای استفاده شخصی از آن استفاده کنید و به حقوق هنرمندان احترام بگذارید.
