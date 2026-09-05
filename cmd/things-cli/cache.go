package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// A nil snapshot means the file did not exist; an empty but non-nil snapshot
// represents an existing empty file and still needs preservation on recovery.
func readCLIStateCacheFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if data == nil {
		data = []byte{}
	}
	return data, nil
}

func checkCLIStateCacheSnapshot(path string, expected []byte) error {
	current, err := readCLIStateCacheFile(path)
	if err != nil {
		return err
	}
	if (current == nil) != (expected == nil) || !bytes.Equal(current, expected) {
		return fmt.Errorf("state cache changed during sync; retry the read")
	}
	return nil
}

func createSyncedCacheFile(dir, pattern string, data []byte) (path string, err error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	path = file.Name()
	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	if err = file.Chmod(0o600); err != nil {
		return path, err
	}
	if _, err = file.Write(data); err != nil {
		return path, err
	}
	if err = file.Sync(); err != nil {
		return path, err
	}
	err = file.Close()
	return path, err
}

// Save only the state derived from the snapshot the reader actually loaded.
// Keep obsolete cache bytes in a private backup and rename a complete new file
// into place; never truncate the usable cache while writing its replacement.
func saveCLIStateCacheSnapshot(path string, cache *cliStateCache, previous []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := checkCLIStateCacheSnapshot(path, previous); err != nil {
		return err
	}
	cache.Version = cliStateCacheVersion
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := createSyncedCacheFile(dir, "."+filepath.Base(path)+".pending-*", data)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary) }()

	if previous != nil {
		var old struct {
			Version     int    `json:"version"`
			HistoryID   string `json:"historyId"`
			ServerIndex int    `json:"serverIndex"`
		}
		if json.Unmarshal(previous, &old) != nil || old.Version != cliStateCacheVersion || old.HistoryID != cache.HistoryID || old.ServerIndex > cache.ServerIndex {
			if _, err := createSyncedCacheFile(dir, filepath.Base(path)+".before-replay-*.bak", previous); err != nil {
				return fmt.Errorf("backing up state cache: %w", err)
			}
		}
	}
	if err := checkCLIStateCacheSnapshot(path, previous); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
