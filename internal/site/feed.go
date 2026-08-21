package site

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/allenhouchins/new-casks-tracker/internal/casks"
)

// FeedCap is the maximum number of entries included in the RSS feed.
const FeedCap = 100

// RSS 2.0 document structure, marshaled with encoding/xml.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description,omitempty"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
}

type rssGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}

// renderFeed writes an RSS 2.0 feed of the newest entries to path.
// entries must already be sorted newest first.
func renderFeed(path, siteURL string, entries []casks.LogEntry, now time.Time) error {
	if len(entries) > FeedCap {
		entries = entries[:FeedCap]
	}

	items := make([]rssItem, 0, len(entries))
	for _, e := range entries {
		title := e.Name
		if e.Version != "" {
			title = fmt.Sprintf("%s %s", e.Name, e.Version)
		}
		desc := e.Desc
		if e.Homepage != "" {
			desc = strings.TrimSpace(desc + " — " + e.Homepage)
		}
		items = append(items, rssItem{
			Title:       title,
			Link:        "https://formulae.brew.sh/cask/" + e.Token,
			Description: desc,
			// Token + detection time keeps GUIDs unique even if a cask
			// is ever removed from Homebrew and later re-added.
			GUID: rssGUID{
				Value:       e.Token + "@" + e.DetectedAt.UTC().Format(time.RFC3339),
				IsPermaLink: "false",
			},
			PubDate: e.DetectedAt.UTC().Format(time.RFC1123Z),
		})
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:         "New Homebrew Casks",
			Link:          siteURL,
			Description:   "Newly added Homebrew casks, checked every 6 hours",
			LastBuildDate: now.UTC().Format(time.RFC1123Z),
			Items:         items,
		},
	}

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling RSS feed: %w", err)
	}
	out := append([]byte(xml.Header), data...)
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
