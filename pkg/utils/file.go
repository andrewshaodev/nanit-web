package utils

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data, readable only by its owner. A
// crash mid-write leaves the old file, never a truncated one.
func WriteFileAtomic(path string, data []byte) error {
	// CreateTemp makes the file 0600, and a unique name per write
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
