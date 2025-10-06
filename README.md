sci-scraper

A small Go CLI to scrape Supreme Court of India landmark judgment summaries and save them as JSON.

Usage:

Build:

```bash
go build -o bin/sci-scraper ./cmd/sci-scraper
```

Run (scrape year 2016):

```bash
./bin/sci-scraper -year 2016 -out ./output
```

This will create `sci_judgments_2016.json` in `./output`.
