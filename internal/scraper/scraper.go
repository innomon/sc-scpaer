package scraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Judgment represents a single row from the landmark judgments table.
type Judgment struct {
	DateOfJudgment   string `json:"judgment_date"`
	CauseTitleCaseNo string `json:"cause_title_case_no"`
	Subject          string `json:"subject"`
	JudgmentSummary  string `json:"judgment_summary"`
	PDFLink          string `json:"pdf_link"`
}

func isNumericShort(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ScrapeYear fetches the page for a given year and writes a JSON file in outDir.
func ScrapeYear(year int, outDir string) error {
	if year < 2016 || year > 2025 {
		return errors.New("year out of supported range 2016..2025")
	}
	pageURL := fmt.Sprintf("https://www.sci.gov.in/landmark-judgment-summaries/?judgment_year=%d", year)
	resp, err := http.Get(pageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fetch failed: %s - %s", resp.Status, string(body))
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	var judgments []Judgment

	// helper to resolve relative URLs
	base := resp.Request.URL
	resolve := func(href string) string {
		href = strings.TrimSpace(href)
		if href == "" {
			return ""
		}
		u, err := url.Parse(href)
		if err != nil {
			return href
		}
		if u.IsAbs() {
			return href
		}
		return base.ResolveReference(u).String()
	}

	// Try to find the landmark table first, then fall back to the first table
	sel := doc.Find(".landmark_judgment_summary table").First()
	if sel.Length() == 0 {
		sel = doc.Find("table").First()
	}

	if sel.Length() > 0 {
		// determine header mapping if present
		headerMap := map[string]int{}
		hasHeader := false
		sel.Find("tr").First().Find("th").Each(func(i int, h *goquery.Selection) {
			text := strings.TrimSpace(h.Text())
			if text != "" {
				hasHeader = true
				lower := strings.ToLower(text)
				switch {
				case strings.Contains(lower, "date"):
					headerMap["date"] = i
				case strings.Contains(lower, "cause") || strings.Contains(lower, "case") || strings.Contains(lower, "title"):
					headerMap["cause"] = i
				case strings.Contains(lower, "subject"):
					headerMap["subject"] = i
				case strings.Contains(lower, "summary"):
					headerMap["summary"] = i
				case strings.Contains(lower, "view") || strings.Contains(lower, "pdf"):
					headerMap["pdf"] = i
				}
			}
		})

		sel.Find("tr").Each(func(i int, s *goquery.Selection) {
			// skip header row if present
			if i == 0 && hasHeader {
				return
			}
			cols := s.Find("td")
			if cols.Length() < 1 {
				return
			}

			// helper to read by header mapping or fallback to positional logic
			readBy := func(key string, pos int) string {
				if idx, ok := headerMap[key]; ok && idx < cols.Length() {
					return strings.TrimSpace(cols.Eq(idx).Text())
				}
				if pos < cols.Length() {
					return strings.TrimSpace(cols.Eq(pos).Text())
				}
				return ""
			}

			// Detect and skip a leading serial column if present (numeric short value)
			shift := 0
			if cols.Length() >= 5 {
				first := strings.TrimSpace(cols.Eq(0).Text())
				if isNumericShort(first) {
					shift = 1
				}
			}

			date := readBy("date", 0+shift)
			cause := readBy("cause", 1+shift)
			subject := readBy("subject", 2+shift)
			summary := readBy("summary", 3+shift)

			// find pdf link anywhere in the row: accept explicit .pdf links or site view-pdf handlers
			pdf := ""
			s.Find("a").EachWithBreak(func(i int, a *goquery.Selection) bool {
				if href, ok := a.Attr("href"); ok {
					lh := strings.ToLower(strings.TrimSpace(href))
					if strings.HasSuffix(lh, ".pdf") || strings.Contains(lh, "view-pdf") || strings.Contains(lh, "/view-pdf/") {
						pdf = resolve(href)
						return false
					}
				}
				return true
			})

			if date != "" || cause != "" || subject != "" || summary != "" || pdf != "" {
				judgments = append(judgments, Judgment{DateOfJudgment: date, CauseTitleCaseNo: cause, Subject: subject, JudgmentSummary: summary, PDFLink: pdf})
			}
		})
	}

	if len(judgments) == 0 {
		return fmt.Errorf("no judgments found on page %s", pageURL)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	outFile := filepath.Join(outDir, fmt.Sprintf("sci_judgments_%d.json", year))
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	// preserve characters like '&' in URLs instead of escaping to \u0026
	enc.SetEscapeHTML(false)
	return enc.Encode(judgments)
}
