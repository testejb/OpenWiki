package wiki

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FS interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Glob(pattern string) ([]string, error)
}

type OsFS struct{}

func NewOsFS() *OsFS { return &OsFS{} }

func (o *OsFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (o *OsFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (o *OsFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (o *OsFS) Remove(path string) error {
	return os.Remove(path)
}

func (o *OsFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (o *OsFS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (o *OsFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

type MemFS struct {
	mu    sync.RWMutex
	files map[string][]byte
	dirs  map[string]bool
}

func NewMemFS() *MemFS {
	return &MemFS{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
	}
}

func (m *MemFS) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (m *MemFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		m.dirs[dir] = true
	}

	copied := make([]byte, len(data))
	copy(copied, data)
	m.files[path] = copied
	return nil
}

func (m *MemFS) MkdirAll(path string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dirs[path] = true
	return nil
}

func (m *MemFS) Remove(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.files, path)
	return nil
}

func (m *MemFS) Stat(path string) (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.files[path]; ok {
		return &memFileInfo{name: filepath.Base(path), size: int64(len(m.files[path]))}, nil
	}
	if m.dirs[path] {
		return &memFileInfo{name: filepath.Base(path), isDir: true}, nil
	}
	return nil, fmt.Errorf("路径不存在: %s", path)
}

func (m *MemFS) ReadDir(path string) ([]fs.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	seen := make(map[string]bool)
	var entries []fs.DirEntry
	for p := range m.files {
		if strings.HasPrefix(p, prefix) {
			rest := strings.TrimPrefix(p, prefix)
			parts := strings.SplitN(rest, "/", 2)
			name := parts[0]
			if !seen[name] {
				seen[name] = true
				entries = append(entries, &memDirEntry{name: name, isDir: len(parts) > 1})
			}
		}
	}
	for d := range m.dirs {
		if strings.HasPrefix(d, prefix) && d != path {
			rest := strings.TrimPrefix(d, prefix)
			parts := strings.SplitN(rest, "/", 2)
			name := parts[0]
			if !seen[name] {
				seen[name] = true
				entries = append(entries, &memDirEntry{name: name, isDir: true})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

func (m *MemFS) Glob(pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []string
	for p := range m.files {
		matched, err := filepath.Match(pattern, p)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, p)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *memFileInfo) Name() string       { return m.name }
func (m *memFileInfo) Size() int64        { return m.size }
func (m *memFileInfo) Mode() os.FileMode  { return 0644 }
func (m *memFileInfo) ModTime() time.Time { return time.Time{} }
func (m *memFileInfo) IsDir() bool        { return m.isDir }
func (m *memFileInfo) Sys() interface{}   { return nil }

type memDirEntry struct {
	name  string
	isDir bool
}

func (m *memDirEntry) Name() string      { return m.name }
func (m *memDirEntry) IsDir() bool       { return m.isDir }
func (m *memDirEntry) Type() os.FileMode { return 0 }
func (m *memDirEntry) Info() (os.FileInfo, error) {
	return &memFileInfo{name: m.name, isDir: m.isDir}, nil
}
