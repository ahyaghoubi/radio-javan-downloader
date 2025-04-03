# Radio Javan Media Downloader

A command-line tool written in Go that generates direct download links for media (songs and music videos) hosted on Radio Javan. It supports both individual song URLs and artist pages, where it fetches and processes multiple songs. The program constructs multiple candidate media URLs, validates them, and outputs the first working link.

---

## Features

- **Media URL Generation:** Constructs several potential download URLs based on the provided media identifier.
- **Validation:** Verifies media files by checking HTTP response status and file size to ensure valid downloads.
- **Artist Support:** Extracts and processes all song URLs from an artist's page using the Radio Javan API.
- **Redirect Resolution:** Automatically resolves shortened URLs (e.g., starting with `https://rj.app/`) to their full destination.
- **Progress Feedback:** Displays a progress bar when processing multiple songs from an artist.
- **Output File Generation:** Writes download links to an `output.txt` file for easy access.

---

## Precompiled Executable

For users who prefer not to build the project themselves, a precompiled executable is available on the [Release page](https://github.com/ahyaghoubi/radio-javan-downloader/releases).  

- **Windows Users:** The executable is named `radio-javan-downloader.exe`.
- **Other Operating Systems:** Download the appropriate executable for your platform.

Simply download the file, ensure it has the correct execution permissions (if necessary), and run it from your command line.

---

## Requirements

- **Go:** Version 1.18 or higher (if you wish to build from source)
- **Internet Connection:** Required for API calls and media file validation.
- **Dependencies:**
  - [progressbar/v3](https://github.com/schollz/progressbar) – Used for displaying progress during batch processing.

---

## Installation

### Using the Precompiled Executable

1. Visit the [Release page](https://github.com/ahyaghoubi/radio-javan-downloader/releases).
2. Download the executable for your operating system.
   - For Windows, the file is `radio-javan-downloader.exe`.
3. Make sure the executable has the proper permissions.
4. Run the executable from your terminal or command prompt.

### Building from Source

1. **Clone the Repository:**

   ```bash
   git clone https://github.com/ahyaghoubi/radio-javan-downloader.git
   cd radio-javan-downloader
   ```

2. **Install Dependencies:**

   Use Go modules to manage dependencies. If not already initialized, run:

   ```bash
   go mod init radio-javan-downloader
   go get github.com/schollz/progressbar/v3
   ```

3. **Build the Project:**

   ```bash
   go build -o radio-javan-downloader.exe
   ```

   > **Note:** On non-Windows systems, you may choose a different binary name or omit the `.exe` extension.

---

## Usage

1. **Run the Program:**

   If using the precompiled executable on Windows:

   ```bash
   radio-javan-downloader.exe
   ```

   On other operating systems or if built from source, run the generated binary accordingly.

2. **Input Options:**
   - **Song URL:** Provide a direct song link (e.g., `https://play.radiojavan.com/song/<permlink>`). The program will output a validated download link.
   - **Artist URL:** Provide an artist page URL (e.g., `https://play.radiojavan.com/artist/<artistName>/songs`). The tool will fetch all unique song links, validate them, and write the download links to an `output.txt` file.
   - **Short URLs:** If you enter a URL starting with `https://rj.app/`, it will automatically resolve it to the full URL.

3. **Exiting:**
   - To exit the program, simply input `0` when prompted for a link.

---

## Code Overview

### Media URL Construction

- **`getMediaUrls(mediaName string) []string`**  
  Generates a list of candidate URLs for the media by using various hosts and paths for both MP3 and MP4 formats.

### Media File Validation

- **`isValidMediaFile(url string) (bool, error)`**  
  Validates if a media file exists by performing HTTP HEAD (or GET as a fallback) requests. Checks for an HTTP 200 status and sufficient file size.

- **`checkMediaUrls(urls []string) string`**  
  Iterates through the generated URLs and returns the first URL that passes validation.

### URL Resolution

- **`resolveRedirect(link string) (string, error)`**  
  Follows HTTP redirects for shortened URLs to obtain the full URL.

### Media Processing

- **`oneSongDownloadlink(link string) (string, string, error)`**  
  Extracts the media name from a song URL, generates candidate download links, and returns the first valid one. Also provides an indicator if the song couldn’t be fetched.

- **`getMediaTypeFromURL(link string) (string, error)`**  
  Determines whether the provided URL points to a song or an artist.

- **`extractArtistName(link string) (string, error)`**  
  Extracts the artist’s name from the artist URL.

### Fetching Artist Songs

- **`fetchArtistsSongs(artistName string) ([]string, error)`**  
  Calls the Radio Javan API to retrieve song data for an artist and extracts unique song URLs.

- **`extractPermlinks(obj interface{}, songLinks map[string]bool)`**  
  Recursively searches the JSON response for "permlink" keys to construct full song URLs.

### Main Loop

- The program continuously prompts the user for a URL.
- It processes the input based on whether it's a song or artist URL.
- For artist URLs, download links are written to `output.txt` and a progress bar is displayed.
- Appropriate error handling is implemented to manage network, parsing, and file I/O errors.

---

## Contributing

Contributions are welcome! To improve or add new features:

1. Fork the repository.
2. Create a new branch for your changes.
3. Submit a pull request with a clear description of your changes.

For major changes, please open an issue first to discuss what you would like to change.

---

Feel free to reach out if you have any questions or issues with the project!
