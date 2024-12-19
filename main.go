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
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	timeout = 60 * time.Second
	out     = "screenshot.png"
	debug   = parseDebugEnv()
)

func parseDebugEnv() bool {
	val, exists := os.LookupEnv("DEBUG")
	if !exists {
		return false
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		log.Printf("Invalid DEBUG value: %s, defaulting to false", val)
		return false
	}

	return parsed
}

func fetchWithChromedp(url string) (html string, screenshot []byte, err error) {
	allocatorCtx, cancel := chromedp.NewExecAllocator(context.Background(),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-automation", true),
	)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocatorCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	var buf []byte
	var htmlContent string

	tasks := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(2 * time.Second),
		chromedp.InnerHTML("html", &htmlContent),
	}

	if debug {
		tasks = append(tasks, chromedp.CaptureScreenshot(&buf))
	}

	err = chromedp.Run(ctx, tasks...)
	if err != nil {
		return "", nil, fmt.Errorf("chromedp error: %w", err)
	}

	if debug {
		return htmlContent, buf, nil
	}

	return htmlContent, nil, nil
}

// fetchProductMeta extracts metadata from the HTML
func fetchProductMeta(data string) (map[string]interface{}, error) {
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

	var response = make(map[string]interface{})
	doc.Find("title").Each(func(i int, s *goquery.Selection) {
		response["title"] = s.Text()
	})

	response["meta"] = meta

	return response, nil
}

func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	html, buf, err := fetchWithChromedp(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch page: %v", err), http.StatusInternalServerError)
		return
	}

	if debug {
		if _, err := png.Decode(bytes.NewReader(buf)); err != nil {
			http.Error(w, fmt.Sprintf("failed to decode screenshot: %v", err), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(out, buf, 0o644); err != nil {
			http.Error(w, fmt.Sprintf("failed to write screenshot to file: %v", err), http.StatusInternalServerError)
			return
		}
	}

	resp, err := fetchProductMeta(html)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to extract metadata: %v", err), http.StatusInternalServerError)
		return
	}

	if debug {
		resp["screenshot"] = out
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
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
