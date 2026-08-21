// Package casks handles fetching the Homebrew cask index, persisting the
// set of known casks, and recording newly discovered ones.
package casks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultAPIURL is the Homebrew formulae API endpoint for all casks.
// Homebrew regenerates this file roughly every 15 minutes.
const DefaultAPIURL = "https://formulae.brew.sh/api/cask.json"

// Cask is the subset of the Homebrew cask API entry we care about.
// Fields we don't list are ignored by encoding/json.
type Cask struct {
	Token     string   `json:"token"`
	FullToken string   `json:"full_token"`
	Names     []string `json:"name"`
	Desc      string   `json:"desc"`
	Homepage  string   `json:"homepage"`
	Version   string   `json:"version"`
}

// DisplayName returns the first non-empty human-readable name,
// falling back to the token.
func (c Cask) DisplayName() string {
	for _, n := range c.Names {
		if strings.TrimSpace(n) != "" {
			return strings.TrimSpace(n)
		}
	}
	return c.Token
}

// Fetch downloads and decodes the cask index from url.
// Entries without a token are dropped rather than treated as errors.
func Fetch(ctx context.Context, url string) ([]Cask, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: unexpected status %s", url, resp.Status)
	}
	return Decode(resp.Body)
}

// FetchFile decodes the cask index from a local JSON file. Useful for
// local testing without hitting the network.
func FetchFile(path string) ([]Cask, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	return Decode(f)
}

// Decode parses the cask index JSON and filters out malformed entries.
func Decode(r io.Reader) ([]Cask, error) {
	var all []Cask
	if err := json.NewDecoder(r).Decode(&all); err != nil {
		return nil, fmt.Errorf("decoding cask JSON: %w", err)
	}

	valid := all[:0]
	for _, c := range all {
		// A cask without a token can't be tracked; skip it gracefully.
		if strings.TrimSpace(c.Token) == "" {
			continue
		}
		valid = append(valid, c)
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("cask JSON contained no valid entries")
	}
	return valid, nil
}
