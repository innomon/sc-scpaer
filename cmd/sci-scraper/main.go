package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/local/sci-scraper/internal/scraper"
)

func main() {
	year := flag.Int("year", 0, "Single year to scrape (overrides from/to)")
	from := flag.Int("from", 2017, "Start year to scrape (inclusive)")
	to := flag.Int("to", 2018, "End year to scrape (inclusive)")
	out := flag.String("out", "./output", "Output directory for JSON files")
	concurrency := flag.Int("concurrency", 1, "Number of concurrent workers to run")
	retries := flag.Int("retries", 0, "Number of times to retry a failed year")
	retryDelay := flag.Int("retry-delay", 2, "Delay in seconds between retries")
	flag.Parse()

	years := []int{}
	if *year != 0 {
		years = append(years, *year)
	} else {
		for y := *from; y <= *to; y++ {
			years = append(years, y)
		}
	}

	// If concurrency is 1, just run sequentially (simple path)
	if *concurrency <= 1 {
		for _, y := range years {
			fmt.Printf("Scraping year %d -> output dir %s\n", y, *out)
			if err := scraper.ScrapeYear(y, filepath.Clean(*out)); err != nil {
				log.Printf("scrape failed for %d: %v", y, err)
			} else {
				fmt.Printf("Done year %d\n", y)
			}
		}
		return
	}

	// Worker pool for concurrent scraping
	type job struct{ year int }
	jobs := make(chan job)
	var wg sync.WaitGroup

	worker := func(id int) {
		defer wg.Done()
		for j := range jobs {
			attempt := 0
			for {
				attempt++
				fmt.Printf("worker %d: scraping %d (attempt %d)\n", id, j.year, attempt)
				err := scraper.ScrapeYear(j.year, filepath.Clean(*out))
				if err == nil {
					fmt.Printf("worker %d: done %d\n", id, j.year)
					break
				}
				log.Printf("worker %d: error scraping %d: %v", id, j.year, err)
				if attempt > *retries {
					log.Printf("worker %d: giving up on %d after %d attempts", id, j.year, attempt)
					break
				}
				time.Sleep(time.Duration(*retryDelay) * time.Second)
			}
		}
	}

	// start workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i + 1)
	}

	// send jobs
	go func() {
		for _, y := range years {
			fmt.Printf("queueing year %d\n", y)
			jobs <- job{year: y}
		}
		close(jobs)
	}()

	wg.Wait()
}
