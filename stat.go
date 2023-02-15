package dget

import (
	"os"
	"path/filepath"

	"github.com/boltdb/bolt"
)

var cache *bolt.DB

func initCache() error {
	db, err := bolt.Open(filepath.Join(os.TempDir(), "fb-runner.db"), 0600, nil)
	if err != nil {
		return err
	}
	cache = db
	return nil
}
