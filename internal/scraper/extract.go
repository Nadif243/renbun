package scraper

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

// Compile the regex ONCE when the engine boots, not every time a URL is scraped.
var hiraganaRegex = regexp.MustCompile(`\p{Hiragana}`)

// ExtractSentences takes a URL, downloads the HTML, and isolates Japanese sentences.
func ExtractSentences(url string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja,en-US;q=0.7,en;q=0.3")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network timeout or failure: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("website rejected request: %d %s", res.StatusCode, res.Status)
	}

	// The Universal Translator
	// This automatically detects Shift_JIS or EUC-JP and streams it as clean UTF-8
	utf8Body, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to convert character encoding: %w", err)
	}

	// Load the sanitized UTF-8 stream into goquery
	doc, err := goquery.NewDocumentFromReader(utf8Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML structure: %w", err)
	}

	// Destroy all furigana reading tags to keep the kanji clean
	doc.Find("rt, rp").Remove()

	var sentences []string

	doc.Find("p, .main_text").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			parts := strings.Split(text, "。")
			for _, part := range parts {
				// Clean up internal newlines caused by Aozora's <br> tags
				cleanPart := strings.ReplaceAll(part, "\n", "")
				cleanPart = strings.ReplaceAll(cleanPart, "\r", "")

				// Explicitly target the Japanese full-width space " " as well as standard spaces
				cleanPart = strings.ReplaceAll(cleanPart, " ", "")
				cleanPart = strings.ReplaceAll(cleanPart, "　", "")

				cleanPart = strings.TrimSpace(cleanPart)

				if len([]rune(cleanPart)) > 5 && hiraganaRegex.MatchString(cleanPart) {
					sentences = append(sentences, cleanPart+"。")
				}
			}
		}
	})

	return sentences, nil
}
