package casks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// LogEntry is one newly discovered cask in the append-only history log.
type LogEntry struct {
	Token      string    `json:"token"`
	Name       string    `json:"name"`
	Desc       string    `json:"desc,omitempty"`
	Homepage   string    `json:"homepage,omitempty"`
	Version    string    `json:"version,omitempty"`
	DetectedAt time.Time `json:"detected_at"`
}

// LoadKnown reads the set of known cask tokens from path. The second return
// value reports whether the file existed; a missing file (first run) is not
// an error.
func LoadKnown(path string) (map[string]bool, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", path, err)
	}

	var tokens []string
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, false, fmt.Errorf("parsing %s: %w", path, err)
	}
	known := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		known[t] = true
	}
	return known, true, nil
}

// SaveKnown writes the full token set to path as a sorted JSON array,
// which keeps git diffs of the state file small and stable.
func SaveKnown(path string, known map[string]bool) error {
	tokens := make([]string, 0, len(known))
	for t := range known {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	return writeJSONAtomic(path, tokens)
}

// LoadLog reads the history of newly discovered casks. A missing file
// yields an empty log.
func LoadLog(path string) ([]LogEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var entries []LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return entries, nil
}

// SaveLog writes the full history log to path.
func SaveLog(path string, entries []LogEntry) error {
	if entries == nil {
		entries = []LogEntry{} // serialize an empty log as [], not null
	}
	return writeJSONAtomic(path, entries)
}

// NewSince returns the casks from current whose tokens are not in known,
// sorted by token for deterministic output.
func NewSince(current []Cask, known map[string]bool) []Cask {
	var fresh []Cask
	for _, c := range current {
		if !known[c.Token] {
			fresh = append(fresh, c)
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].Token < fresh[j].Token })
	return fresh
}

// writeJSONAtomic marshals v and writes it to path via a temp file + rename,
// so a crash or full disk never leaves a truncated state file behind.
func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", path, err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
