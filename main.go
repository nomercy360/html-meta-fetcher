package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"image/png"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	timeout = 120 * time.Second
)

// fetchWithChromedp fetches the HTML content and captures a screenshot
func fetchWithChromedp(url string) (html string, screenshot []byte, err error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	var buf []byte
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("meta[property]", chromedp.ByQueryAll),
		chromedp.InnerHTML("html", &html),
		chromedp.CaptureScreenshot(&buf),
	)
	if err != nil {
		return "", nil, fmt.Errorf("chromedp error: %w", err)
	}

	_, err = png.Decode(bytes.NewReader(buf))
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	return html, buf, nil
}

// fetchProductMeta extracts metadata from the HTML
func fetchProductMeta(data string) (map[string]string, error) {
	reader := strings.NewReader(data)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	meta := map[string]string{}
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if property, exists := s.Attr("property"); exists {
			if content, exists := s.Attr("content"); exists {
				meta[property] = content
			}
		}
		if name, exists := s.Attr("name"); exists {
			if content, exists := s.Attr("content"); exists {
				meta[name] = content
			}
		}
	})

	return meta, nil
}

// handleScreenshot handles the /screenshot API endpoint
func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	html, _, err := fetchWithChromedp(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch page: %v", err), http.StatusInternalServerError)
		return
	}

	meta, err := fetchProductMeta(html)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to extract metadata: %v", err), http.StatusInternalServerError)
		return
	}
	//
	//// Save the screenshot temporarily
	//tempFile, err := os.CreateTemp("", "screenshot-*.png")
	//if err != nil {
	//	http.Error(w, "failed to create temp file", http.StatusInternalServerError)
	//	return
	//}
	//defer tempFile.Close()
	//
	//_, err = tempFile.Write(screenshot)
	//if err != nil {
	//	http.Error(w, "failed to write screenshot to file", http.StatusInternalServerError)
	//	return
	//}

	// Build the response
	response := map[string]interface{}{
		"metadata": meta,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/meta-fetcher", handleScreenshot)

	port := "8080"
	fmt.Printf("Server is running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
