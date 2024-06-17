package filesystem

import (
	"archive/zip"
	"dependency-filter/internal/utils"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type File struct {
	Name         string    `json:"name"`
	RelativePath string    `json:"relative_path"`
	Size         uint64    `json:"size"`
	IsDir        bool      `json:"isdir"`
	LastModified time.Time `json:"last_modified"`
}
type FileSystem struct {
	root           string
	newFiles       []File
	oldFiles       []File
	latestModified time.Time
}

func NewFileSystem(root string) *FileSystem {
	filesystem := &FileSystem{
		root:           root,
		newFiles:       make([]File, 0),
		oldFiles:       make([]File, 0),
		latestModified: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	filesystem.load()
	return filesystem
}

// List returns all traversed files
func (system *FileSystem) List() ([]File, error) {
	err := filepath.WalkDir(system.root, func(path string, d fs.DirEntry, err error) error {
		if path == system.root {
			return nil
		}
		if err != nil {
			logrus.Warnf("filed to walk folder %q: %v", path, err)
			return filepath.SkipDir
		}

		info, err := d.Info()

		// we only traverse non directory files
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(system.root, filepath.Dir(path))
		if err != nil {
			return err
		}
		if err != nil {
			logrus.Warnf("filed to get info for %q: %v", rel, err)
			return filepath.SkipDir
		}
		// ignore these files, the time of these files will affect subsequent filtering.
		if info.Name() == ".dependency-filter.json" || info.Name() == ".dependency-filter-tmp.json" || info.Name() == "dependency-filter.zip" || info.Name() == os.Args[0] {
			return nil
		}

		system.newFiles = append(system.newFiles, File{
			Name:         info.Name(),
			RelativePath: filepath.ToSlash(rel),
			Size:         uint64(info.Size()),
			IsDir:        info.IsDir(),
			LastModified: info.ModTime(),
		})

		lastModified := info.ModTime()
		if system.latestModified.Before(lastModified) {
			system.latestModified = lastModified
		}
		return nil
	})

	// save all files information to a temporary file
	var file *os.File
	path := filepath.Join(system.root, ".dependency-filter-tmp.json")
	file, _ = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	defer file.Close()
	bytes, _ := json.Marshal(system.newFiles)
	_, _ = file.Write(bytes)

	return system.newFiles, err
}

// load .dependency-filter.json if it exists, the file contains the last dependencies' information
func (system *FileSystem) load() {
	file, ok := utils.Exists(filepath.Join(system.root, ".dependency-filter.json"))
	if !ok {
		logrus.Warnf("no .dependency-filter files found in %q", system.root)
		return
	}
	infos, err := os.ReadFile(file)
	if err != nil {
		logrus.Warnf("failed to read .dependency-filter.json file %q: %v", file, err)
		return
	}
	err = json.Unmarshal(infos, &system.oldFiles)
	if err != nil {
		logrus.Warnf("failed to unmarshal .dependency-filter.json file %q: %v", file, err)
		return
	}
}

// Filter returns the files with differences, which are the latest dependencies
func (system *FileSystem) Filter(mode string, interval int) ([]File, error) {
	if len(system.oldFiles) == 0 {
		mode = "latest"
	}

	diffFiles := make([]File, 0)
	newFiles, err := system.List()
	if err != nil {
		return nil, err
	}

	if mode == "latest" {
		// in latest mode, filter the files by their last modified times.
		for _, file := range newFiles {
			if duration := system.latestModified.Sub(file.LastModified); duration < time.Duration(interval)*time.Minute {
				diffFiles = append(diffFiles, file)
			}
		}
	} else if mode == "compare" {
		// in compare mode, only the files that are different from each other are filtered out.
		var path string
		visited := make(map[string]bool)
		newFilesMap := make(map[string]File)
		oldFilesMap := make(map[string]File)
		for _, file := range newFiles {
			path = filepath.Join(file.RelativePath, file.Name)
			visited[filepath.Join(file.RelativePath, file.Name)] = false
			newFilesMap[path] = file
		}

		for _, file := range system.oldFiles {
			path = filepath.Join(file.RelativePath, file.Name)
			oldFilesMap[path] = file
			// 如果存在设为true，反之如果不存在则设为false
			if _, ok := visited[path]; ok {
				visited[path] = true
			} else {
				visited[path] = false
			}
		}

		for key, value := range visited {
			if !value {
				if _, ok := newFilesMap[key]; !ok {
					diffFiles = append(diffFiles, oldFilesMap[key])
				} else {
					diffFiles = append(diffFiles, newFilesMap[key])
				}
			}
		}
	}

	return diffFiles, nil
}
func (system *FileSystem) Compress(files []File, writer io.Writer) {
	if len(files) == 0 {
		logrus.Info("no files to compress")
		return
	}
	logrus.Infof("finding %d different/latest files", len(files))
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()
	for _, file := range files {
		path := filepath.Join(system.root, file.RelativePath, file.Name)
		info, err := os.Open(path)
		if err != nil {
			logrus.Warn("Error opening file: " + path)
			return
		}
		header := &zip.FileHeader{
			Name:               filepath.FromSlash(filepath.Join(file.RelativePath, file.Name)),
			Modified:           file.LastModified,
			UncompressedSize64: file.Size,
			Method:             zip.Deflate,
		}
		writerToZip, err := zipWriter.CreateHeader(header)
		if err != nil {
			return
		}
		_, err = io.Copy(writerToZip, info)
		if err != nil {
			return
		}
		_ = info.Close()
	}
}

func (system *FileSystem) Flush() {
	oldpath := filepath.Join(system.root, ".dependency-filter-tmp.json")
	newpath := filepath.Join(system.root, ".dependency-filter.json")
	_ = os.Rename(oldpath, newpath)
}
