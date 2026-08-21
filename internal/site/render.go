// Package site renders the static GitHub Pages site from the history log.
package site

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/allenhouchins/new-casks-tracker/internal/casks"
)

// IndexCap is the maximum number of entries shown on the main page;
// the full log is always available on history.html.
const IndexCap = 200

// PageData is passed to the HTML template.
type PageData struct {
	Title       string
	Entries     []casks.LogEntry
	TotalNew    int  // total entries across the whole history log
	Capped      bool // true when Entries is a truncated view of the log
	IsHistory   bool // true on the full-history page
	GeneratedAt time.Time
}

// Renderer generates the static site into an output directory.
type Renderer struct {
	tmpl    *template.Template
	assets  fs.FS
	siteURL string // absolute URL of the published site, used by the RSS feed
}

// New parses the page template from templatesFS and remembers the FS so
// static assets (style.css) can be copied into the output directory.
// siteURL is the absolute URL the site will be published at.
func New(templatesFS fs.FS, siteURL string) (*Renderer, error) {
	funcs := template.FuncMap{
		"fmtTime": func(t time.Time) string {
			return t.UTC().Format("2006-01-02 15:04 UTC")
		},
	}
	tmpl, err := template.New("index.html.tmpl").Funcs(funcs).ParseFS(templatesFS, "templates/index.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing page template: %w", err)
	}
	return &Renderer{tmpl: tmpl, assets: templatesFS, siteURL: siteURL}, nil
}

// Render writes index.html, history.html, feed.xml, and style.css into
// outDir. Log entries are shown newest first everywhere.
func (r *Renderer) Render(outDir string, log []casks.LogEntry, now time.Time) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", outDir, err)
	}

	// Newest first; ties broken by token so output is deterministic.
	sorted := make([]casks.LogEntry, len(log))
	copy(sorted, log)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].DetectedAt.Equal(sorted[j].DetectedAt) {
			return sorted[i].DetectedAt.After(sorted[j].DetectedAt)
		}
		return sorted[i].Token < sorted[j].Token
	})

	indexEntries := sorted
	capped := false
	if len(indexEntries) > IndexCap {
		indexEntries = indexEntries[:IndexCap]
		capped = true
	}

	pages := []struct {
		file string
		data PageData
	}{
		{"index.html", PageData{
			Title:       "New Homebrew Casks",
			Entries:     indexEntries,
			TotalNew:    len(sorted),
			Capped:      capped,
			GeneratedAt: now,
		}},
		{"history.html", PageData{
			Title:       "New Homebrew Casks — Full History",
			Entries:     sorted,
			TotalNew:    len(sorted),
			IsHistory:   true,
			GeneratedAt: now,
		}},
	}
	for _, p := range pages {
		if err := r.renderPage(filepath.Join(outDir, p.file), p.data); err != nil {
			return err
		}
	}

	if err := renderFeed(filepath.Join(outDir, "feed.xml"), r.siteURL, sorted, now); err != nil {
		return err
	}

	// Ship the stylesheet alongside the generated pages so docs/ is
	// fully self-contained for GitHub Pages.
	css, err := fs.ReadFile(r.assets, "templates/style.css")
	if err != nil {
		return fmt.Errorf("reading embedded stylesheet: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "style.css"), css, 0o644); err != nil {
		return fmt.Errorf("writing style.css: %w", err)
	}
	return nil
}

func (r *Renderer) renderPage(path string, data PageData) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	if err := r.tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("rendering %s: %w", path, err)
	}
	return f.Close()
}
