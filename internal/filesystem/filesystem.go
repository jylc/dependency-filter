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
	root     string
	newFiles []File
	oldFiles []File
}

func NewFileSystem(root string) *FileSystem {
	filesystem := &FileSystem{
		root:     root,
		newFiles: make([]File, 0),
		oldFiles: make([]File, 0),
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
		system.newFiles = append(system.newFiles, File{
			Name:         info.Name(),
			RelativePath: filepath.ToSlash(rel),
			Size:         uint64(info.Size()),
			IsDir:        info.IsDir(),
			LastModified: info.ModTime(),
		})
		return nil
	})
	return system.newFiles, err
}

func (system *FileSystem) load() {
	file, ok := utils.Exists(system.root + "/.dependency-filter")
	if !ok {
		logrus.Warnf("no .dependency-filter files found in %q", system.root)
		return
	}
	infos, err := os.ReadFile(file)
	if err != nil {
		logrus.Warnf("failed to read .dependency-filter file %q: %v", file, err)
		return
	}
	err = json.Unmarshal(infos, &system.oldFiles)
	if err != nil {
		logrus.Warnf("failed to unmarshal .dependency-filter file %q: %v", file, err)
		return
	}
}

// Filter returns the files with differences, which are the latest dependencies
func (system *FileSystem) Filter() ([]File, error) {
	if len(system.newFiles) == 0 {
		return nil, nil
	}
	visited := make(map[string]bool)
	oldFilesMap := make(map[string]File)
	diffFiles := make([]File, 0)
	newFiles, err := system.List()
	if err != nil {
		return nil, err
	}
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
	return diffFiles, nil
}

func (system *FileSystem) Save() {
	var file *os.File
	path, _ := utils.Exists(system.root + "/.dependency-filter")
	file, _ = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	bytes, _ := json.Marshal(system.newFiles)
	writer := bufio.NewWriter(file)
	_, _ = writer.Write(bytes)
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
