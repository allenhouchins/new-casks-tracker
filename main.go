// Command new-casks-tracker fetches the Homebrew cask index, diffs it
// against the previously seen set of casks, appends newly discovered casks
// to a history log, and regenerates the static GitHub Pages site.
//
// Designed to run on a schedule (GitHub Actions, every 6 hours). It exits
// non-zero on failure and never modifies state files unless the fetch and
// parse both succeeded.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/allenhouchins/new-casks-tracker/internal/casks"
	"github.com/allenhouchins/new-casks-tracker/internal/site"
)

//go:embed templates
var templatesFS embed.FS

func main() {
	log.SetFlags(0) // timestamps come from the CI runner; keep output clean

	var (
		apiURL  = flag.String("api-url", casks.DefaultAPIURL, "cask index URL, or a local JSON file path for testing")
		dataDir = flag.String("data-dir", "data", "directory for state files (known casks + history log)")
		docsDir = flag.String("docs-dir", "docs", "output directory for the generated GitHub Pages site")
		siteURL = flag.String("site-url", "https://allenhouchins.github.io/new-casks-tracker/", "absolute URL the site is published at (used in the RSS feed)")
	)
	flag.Parse()

	if err := run(*apiURL, *dataDir, *docsDir, *siteURL); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(apiURL, dataDir, docsDir, siteURL string) error {
	knownPath := filepath.Join(dataDir, "known_casks.json")
	logPath := filepath.Join(dataDir, "new_casks_log.json")

	// 1. Fetch the current cask index. Any failure here aborts the run
	// before state files are touched, so a flaky network can never
	// corrupt or reset the tracker.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var (
		current []casks.Cask
		err     error
	)
	if strings.HasPrefix(apiURL, "http://") || strings.HasPrefix(apiURL, "https://") {
		current, err = casks.Fetch(ctx, apiURL)
	} else {
		current, err = casks.FetchFile(apiURL)
	}
	if err != nil {
		return err
	}
	log.Printf("fetched %d casks", len(current))

	// 2. Load previous state.
	known, haveBaseline, err := casks.LoadKnown(knownPath)
	if err != nil {
		return err
	}
	history, err := casks.LoadLog(logPath)
	if err != nil {
		return err
	}

	// 3. Diff — but only after a baseline exists. On the very first run
	// every cask would look "new", which would dump thousands of entries
	// into the log at once, so instead the first run just records the
	// current set as the baseline.
	now := time.Now().UTC()
	if haveBaseline {
		fresh := casks.NewSince(current, known)
		for _, c := range fresh {
			history = append(history, casks.LogEntry{
				Token:      c.Token,
				Name:       c.DisplayName(),
				Desc:       c.Desc,
				Homepage:   c.Homepage,
				Version:    c.Version,
				DetectedAt: now,
			})
			log.Printf("new cask: %s (%s)", c.Token, c.DisplayName())
		}
		if len(fresh) == 0 {
			log.Printf("no new casks since last run")
		} else {
			log.Printf("detected %d new cask(s)", len(fresh))
		}
	} else {
		log.Printf("no existing state at %s — recording baseline of %d casks; new casks will be detected from the next run onward", knownPath, len(current))
	}

	// 4. Persist the updated state (full current token set + history log).
	updated := make(map[string]bool, len(current))
	for _, c := range current {
		updated[c.Token] = true
	}
	if err := casks.SaveKnown(knownPath, updated); err != nil {
		return err
	}
	if err := casks.SaveLog(logPath, history); err != nil {
		return err
	}

	// 5. Regenerate the static site.
	renderer, err := site.New(templatesFS, siteURL)
	if err != nil {
		return err
	}
	if err := renderer.Render(docsDir, history, now); err != nil {
		return err
	}
	log.Printf("site regenerated in %s (%d entries in history)", docsDir, len(history))

	fmt.Println("ok")
	return nil
}
