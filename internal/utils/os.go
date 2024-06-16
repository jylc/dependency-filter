package utils

import (
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

// Exists checks if the path exists, and if it exists and is a relative path, returns an absolute path
func Exists(path string) (string, bool) {
	filepath.Clean(path)
	var (
		err error
	)
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(filepath.Dir(os.Args[0]) + "/" + path)
		if err != nil {
			log.Warn(err)
			return "", false
		} else {
			return path, true
		}
	}

	if _, err = os.Stat(path); err != nil {
		log.Warn(err)
		return "", false
	}
	return path, true
}
