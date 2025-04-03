package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

func getMediaUrls(mediaName string) []string {
	return []string{
		fmt.Sprintf("https://host2.rj-mw1.com/media/mp3/mp3-320/%s.mp3", mediaName),
		fmt.Sprintf("https://host1.rj-mw1.com/media/mp3/mp3-320/%s.mp3", mediaName),
		fmt.Sprintf("https://host2.rj-mw1.com/media/podcast/mp3-320/%s.mp3", mediaName),
		fmt.Sprintf("https://host1.rj-mw1.com/media/podcast/mp3-320/%s.mp3", mediaName),
		fmt.Sprintf("https://host2.rj-mw1.com/media/music_video/hd/%s.mp4", mediaName),
		fmt.Sprintf("https://host1.rj-mw1.com/media/music_video/hd/%s.mp4", mediaName),
	}
}

func isValidMediaFile(url string) (bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		resp, err = client.Get(url)
		if err != nil {
			return false, err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	if resp.ContentLength < 100 {
		return false, fmt.Errorf("file too small, Content-Length: %d", resp.ContentLength)
	}
	return true, nil
}

func checkMediaUrls(urls []string) string {
	for _, u := range urls {
		valid, _ := isValidMediaFile(u)
		if valid {
			return u
		}
	}
	return ""
}

func resolveRedirect(link string) (string, error) {
	resp, err := http.Get(link)
	if err != nil {
		return "", fmt.Errorf("error during GET request: %w", err)
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}

func oneSongDownloadlink(link string) (string, string, error) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return "", "", fmt.Errorf("Error parsing URL: %v\n", err)
	}

	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", "", fmt.Errorf("Invalid URL format. Expected path like /mediaType/mediaName")
	}
	mediaName := pathParts[1]

	mediaUrls := getMediaUrls(mediaName)
	if len(mediaUrls) == 0 {
		return "", "", fmt.Errorf("Unsupported media type or invalid media name!")
	}

	songDownloadlink := checkMediaUrls(mediaUrls)
	if songDownloadlink == "" {
		return "", mediaName, nil
	}
	return songDownloadlink, "", nil
}

func getMediaTypeFromURL(link string) (string, error) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL: %v\n", err)
	}

	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", fmt.Errorf("Invalid URL format. Expected path like /mediaType/mediaName")
	}
	return pathParts[0], nil
}

func extractArtistName(link string) (string, error) {
	parsedUrl, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL: %v\n", err)
	}

	segments := strings.Split(strings.Trim(parsedUrl.Path, "/"), "/")
	if len(segments) < 2 {
		return "", fmt.Errorf("unexpected URL format: %s\n", parsedUrl.Path)
	}

	return segments[len(segments)-1], nil
}

func fetchArtistsSongs(artistName string) ([]string, error) {
	apiURL := fmt.Sprintf("https://play.radiojavan.com/api/p/artist?query=%s&v=2", artistName)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return []string{}, fmt.Errorf("Error creating request: %v\n", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://play.radiojavan.com/artist/%s/songs", artistName))
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("x-api-key", "40e87948bd4ef75efe61205ac5f468a9fd2b970511acf58c49706ecb984f1d67")
	req.Header.Set("x-rj-user-agent", "Radio Javan/4.0.2/f6173917bde5c0102c894b5d2e478693c9d750b7 com.radioJavan.rj.web")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []string{}, fmt.Errorf("Error fetching data from the API: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []string{}, fmt.Errorf("Non-OK HTTP status: %v\n", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}, fmt.Errorf("Error reading response body: %v\n", err)
	}

	var data interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return []string{}, fmt.Errorf("Error parsing JSON: %v\n", err)
	}

	songLinks := make(map[string]bool)
	extractPermlinks(data, songLinks)

	seen := make(map[string]bool)
	uniqueLinks := []string{}
	for item := range songLinks {
		lowerCaseItem := strings.ToLower(item)
		if !seen[lowerCaseItem] {
			seen[lowerCaseItem] = true
			uniqueLinks = append(uniqueLinks, item)
		}
	}

	fmt.Printf("Number of Unique Songs Found: %d\n", len(uniqueLinks))
	return uniqueLinks, nil
}

// extractPermlinks recursively searches for "permlink" keys in the data.
func extractPermlinks(obj interface{}, songLinks map[string]bool) {
	switch val := obj.(type) {
	case map[string]interface{}:
		for key, value := range val {
			if key == "permlink" {
				permlink, ok := value.(string)
				if ok {
					link := fmt.Sprintf("https://play.radiojavan.com/song/%s", permlink)
					songLinks[link] = true
				}
			} else {
				extractPermlinks(value, songLinks)
			}
		}
	case []interface{}:
		for _, item := range val {
			extractPermlinks(item, songLinks)
		}
	}
}

func main() {
	for {
		var link string
		fmt.Print("Enter your link (or 0 to exit): ")
		fmt.Scanln(&link)

		if link == "0" {
			fmt.Println("Exiting the program.")
			break
		}

		if strings.HasPrefix(link, "https://rj.app/") {
			resolved, err := resolveRedirect(link)
			if err != nil {
				fmt.Printf("\nError: %v", err)
				os.Exit(1)
			}
			link = resolved
			fmt.Printf("Resolved URL: %s\n", link)
		}

		mediaType, err := getMediaTypeFromURL(link)
		if err != nil {
			fmt.Printf("\nError: %v", err)
			os.Exit(1)
		}

		if mediaType == "artist" {
			artist, err := extractArtistName(link)
			if err != nil {
				fmt.Printf("\nError: %v", err)
				os.Exit(1)
			}

			file, err := os.Create("output.txt")
			if err != nil {
				fmt.Printf("\nError: %v", err)
				os.Exit(1)
			}
			defer file.Close()

			songs, err := fetchArtistsSongs(artist)
			if err != nil {
				fmt.Printf("\nError: %v", err)
				os.Exit(1)
			}

			var notFetchedSongs []string
			bar := progressbar.Default(int64(len(songs)))
			for _, song := range songs {
				dlLink, notFetchedSong, err := oneSongDownloadlink(song)
				if err != nil {
					fmt.Printf("\nError: %v", err)
					continue
				}

				if notFetchedSong != "" {
					notFetchedSongs = append(notFetchedSongs, notFetchedSong)
					continue
				}

				_, err = fmt.Fprintln(file, dlLink)
				if err != nil {
					log.Fatal("Failed to write to file:", err)
				}
				bar.Add(1)
			}

			fmt.Printf("\n\nData written to output.txt successfully\n\n")

			if len(notFetchedSongs) != 0 {
				fmt.Println("These songs could not be fetched:")
				for _, item := range notFetchedSongs {
					fmt.Println(item)
				}
			}
		} else if mediaType == "song" {
			dlLink, notFetchedSong, err := oneSongDownloadlink(link)
			if err != nil {
				fmt.Printf("\nError: %v", err)
				os.Exit(1)
			}

			if notFetchedSong != "" {
				fmt.Printf("\nError: %v Could not be fetched!", err)
				os.Exit(1)
			}

			fmt.Printf("\nDownload link: %v\n\n\n", dlLink)
		} else {
			fmt.Println("Unsupported media type!")
		}
	}
}
