package cache

import (
	"os"
	"sync"
	"time"
)

const stagedDownloadTTL = 10 * time.Minute

type stagedDownload struct {
	Path      string
	FileName  string
	Size      int64
	expiresAt time.Time
}

var (
	stagingMu    sync.Mutex
	stagingFiles = map[string]stagedDownload{}
	sweepOnce    sync.Once
)

// StageDownload records a temp file on disk under the given token, ready to be
// fetched once (or cleaned up if it never is). Starts the background sweeper for
// abandoned downloads on first use.
func StageDownload(token, path, fileName string, size int64) {
	sweepOnce.Do(startDownloadSweeper)

	stagingMu.Lock()
	defer stagingMu.Unlock()
	stagingFiles[token] = stagedDownload{
		Path:      path,
		FileName:  fileName,
		Size:      size,
		expiresAt: time.Now().Add(stagedDownloadTTL),
	}
}

// GetStagedDownload returns the staged file for a token, if it exists and hasn't expired.
func GetStagedDownload(token string) (stagedDownload, bool) {
	stagingMu.Lock()
	defer stagingMu.Unlock()
	d, ok := stagingFiles[token]
	if !ok || time.Now().After(d.expiresAt) {
		return stagedDownload{}, false
	}
	return d, true
}

// ConsumeStagedDownload removes a token from the staging map without touching the
// file on disk - the caller is expected to have already read (or is about to delete) it.
func ConsumeStagedDownload(token string) {
	stagingMu.Lock()
	delete(stagingFiles, token)
	stagingMu.Unlock()
}

// startDownloadSweeper periodically deletes staged files that were never fetched
// (the user closed the confirm dialog, navigated away, etc.) so they don't accumulate
// on disk indefinitely.
func startDownloadSweeper() {
	go func() {
		ticker := time.NewTicker(stagedDownloadTTL / 2)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			stagingMu.Lock()
			for token, d := range stagingFiles {
				if now.After(d.expiresAt) {
					os.Remove(d.Path) //nolint:errcheck
					delete(stagingFiles, token)
				}
			}
			stagingMu.Unlock()
		}
	}()
}
