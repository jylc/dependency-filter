package filesystem

import (
	"archive/zip"
	"bufio"
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

// List returns all traversed files and path
func (system *FileSystem) List() ([]File, error) {
	err := filepath.WalkDir(system.root, func(path string, d fs.DirEntry, err error) error {
		if path == system.root {
			return nil
		}
		if err != nil {
			logrus.Warnf("filed to walk folder %q: %v", path, err)
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(system.root, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			logrus.Warnf("filed to get info for %q: %v", rel, err)
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == ".dependency-filter.json" || info.Name() == ".dependency-filter-tmp.json" || info.Name() == "dependency-filter.zip" {
			return filepath.SkipDir
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

	var file *os.File
	path, _ := utils.Exists(system.root + "/.dependency-filter-tmp.json")
	file, _ = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	bytes, _ := json.Marshal(system.newFiles)
	writer := bufio.NewWriter(file)
	_, _ = writer.Write(bytes)

	return system.newFiles, err
}

func (system *FileSystem) load() {
	file, ok := utils.Exists(system.root + "/.dependency-filter.json")
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
func (system *FileSystem) Filter(mode string) ([]File, error) {
	if len(system.oldFiles) == 0 {
		mode = "latest"
	}
	visited := make(map[string]bool)
	oldFilesMap := make(map[string]File)
	diffFiles := make([]File, 0)
	newFiles, err := system.List()
	if err != nil {
		return nil, err
	}

	if mode == "latest" {
		for _, file := range newFiles {
			if system.latestModified.Compare(file.LastModified) == 0 {
				diffFiles = append(diffFiles, file)
			}
		}
	} else if mode == "compare" {
		var path string
		for _, file := range system.oldFiles {
			if file.IsDir {
				continue
			}
			path = filepath.Join(file.RelativePath, file.Name)
			visited[filepath.Join(file.RelativePath, file.Name)] = false
			oldFilesMap[path] = file
		}

		for _, file := range newFiles {
			path = filepath.Join(file.RelativePath, file.Name)
			if ok := visited[path]; ok {
				visited[path] = true
			} else {
				visited[path] = false
			}
		}

		for key, value := range visited {
			if !value {
				diffFiles = append(diffFiles, oldFilesMap[key])
			}
		}
	}

	return diffFiles, nil
}
func (system *FileSystem) Compress(files []File, writer io.Writer) {
	if len(files) == 0 {
		return
	}
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()
	for _, file := range files {
		path := filepath.Join(file.RelativePath, file.Name)
		info, err := os.Open(path)
		if err != nil {
			logrus.Warn("Error opening file: " + path)
			return
		}
		header := &zip.FileHeader{
			Name:               filepath.FromSlash(path),
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
