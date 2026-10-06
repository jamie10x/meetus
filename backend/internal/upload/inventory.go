package upload

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

var managedName = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|webp)$`)

type Candidate struct {
	Name       string    `json:"name"`
	Bytes      int64     `json:"bytes"`
	ModifiedAt time.Time `json:"modifiedAt"`
}
type Inventory struct {
	Files             int64       `json:"files"`
	Bytes             int64       `json:"bytes"`
	ReferencedFiles   int64       `json:"referencedFiles"`
	UnreferencedFiles int64       `json:"unreferencedFiles"`
	EligibleFiles     int64       `json:"eligibleFiles"`
	EligibleBytes     int64       `json:"eligibleBytes"`
	IgnoredFiles      int64       `json:"ignoredFiles"`
	Candidates        []Candidate `json:"candidates,omitempty"`
}

// Audit is read-only. Unreferenced is a snapshot, not authorization to delete:
// an organizer can still be editing a draft that references an uploaded file.
// Only names produced by our uploader are considered; links and directories
// are never followed. Candidate details are capped to keep reports bounded.
func Audit(ctx context.Context, dir string, urls []string, olderThan time.Time, detailLimit int) (Inventory, error) {
	result := Inventory{}
	referenced := make(map[string]bool, len(urls))
	for _, raw := range urls {
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if strings.HasPrefix(parsed.Path, "/uploads/") && managedName.MatchString(path.Base(parsed.Path)) {
			referenced[path.Base(parsed.Path)] = true
		}
	}
	directory, err := os.Open(dir)
	if err != nil {
		return result, err
	}
	defer directory.Close()
	for {
		entries, readErr := directory.ReadDir(256)
		for _, entry := range entries {
			if err = ctx.Err(); err != nil {
				return result, err
			}
			if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !managedName.MatchString(entry.Name()) {
				result.IgnoredFiles++
				continue
			}
			info, err := entry.Info()
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return result, err
			}
			if !info.Mode().IsRegular() {
				result.IgnoredFiles++
				continue
			}
			result.Files++
			result.Bytes += info.Size()
			if referenced[entry.Name()] {
				result.ReferencedFiles++
				continue
			}
			result.UnreferencedFiles++
			if !info.ModTime().Before(olderThan) {
				continue
			}
			result.EligibleFiles++
			result.EligibleBytes += info.Size()
			if len(result.Candidates) < detailLimit {
				result.Candidates = append(result.Candidates, Candidate{entry.Name(), info.Size(), info.ModTime().UTC()})
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return result, readErr
		}
	}
	return result, nil
}
