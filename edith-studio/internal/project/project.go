package project

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxTextBytes = 1 << 20

// Dependencies 是创建 Module 所需的稳定项目配置。
type Dependencies struct {
	// ProjectRoot 是本次 Studio 进程服务的项目根目录。
	ProjectRoot string
}

// Module 是一个 ProjectRoot 的长期受限文件读取能力。
type Module struct {
	// projectRoot 是规范化后的绝对项目根目录，也是所有读取的安全边界。
	projectRoot string
}

// New 校验 ProjectRoot 并返回只服务该目录的文件 Module。
func New(dependencies Dependencies) (*Module, error) {
	projectRoot := strings.TrimSpace(dependencies.ProjectRoot)
	if projectRoot == "" {
		return nil, errors.New("project root is required")
	}
	absoluteRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root symlinks: %w", err)
	}
	rootInfo, err := os.Stat(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("stat project root: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, errors.New("project root is not a directory")
	}
	return &Module{projectRoot: filepath.Clean(resolvedRoot)}, nil
}

// ProjectRoot 返回 Module 持有的规范化绝对项目根目录，供 Workspace 组装其他能力。
func (m *Module) ProjectRoot() string {
	return m.projectRoot
}

// ListChildren 返回 relativeDir 中按目录优先、名称排序的直接子项；空字符串表示 ProjectRoot。
func (m *Module) ListChildren(relativeDir string) ([]FileEntry, error) {
	directoryPath, relativeDir, err := m.resolve(relativeDir, true)
	if err != nil {
		return nil, err
	}
	directoryInfo, err := os.Stat(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("stat directory %q: %w", relativeDir, err)
	}
	if !directoryInfo.IsDir() {
		return nil, fmt.Errorf("read directory %q: %w", relativeDir, ErrNotDirectory)
	}
	directoryEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", relativeDir, err)
	}

	entries := make([]FileEntry, 0, len(directoryEntries))
	for _, directoryEntry := range directoryEntries {
		entryPath := filepath.Join(directoryPath, directoryEntry.Name())
		entryInfo, err := os.Lstat(entryPath)
		if err != nil {
			return nil, fmt.Errorf("stat directory entry %q: %w", directoryEntry.Name(), err)
		}
		kind, ok, err := m.entryKind(entryPath, entryInfo)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		entries = append(entries, FileEntry{
			Path: filepath.Join(relativeDir, directoryEntry.Name()),
			Name: directoryEntry.Name(),
			Kind: kind,
		})
	}
	sort.SliceStable(entries, func(left, right int) bool {
		if entries[left].Kind != entries[right].Kind {
			return entries[left].Kind == EntryKindDirectory
		}
		return strings.ToLower(entries[left].Name) < strings.ToLower(entries[right].Name)
	})
	return entries, nil
}

// ReadText 返回 relativeFile 的文本内容；超出首版大小上限时返回内容前缀并标记 Truncated。
func (m *Module) ReadText(relativeFile string) (FileContent, error) {
	filePath, cleanPath, err := m.resolve(relativeFile, false)
	if err != nil {
		return FileContent{}, err
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return FileContent{}, fmt.Errorf("stat file %q: %w", cleanPath, err)
	}
	if !fileInfo.Mode().IsRegular() {
		return FileContent{}, fmt.Errorf("read file %q: %w", cleanPath, ErrNotRegularFile)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return FileContent{}, fmt.Errorf("open file %q: %w", cleanPath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxTextBytes+1))
	if err != nil {
		return FileContent{}, fmt.Errorf("read file %q: %w", cleanPath, err)
	}
	truncated := len(content) > maxTextBytes
	if truncated {
		content = content[:maxTextBytes]
	}
	if strings.IndexByte(string(content), 0) >= 0 {
		return FileContent{}, fmt.Errorf("read file %q: %w", cleanPath, ErrNotTextFile)
	}
	return FileContent{
		Path:      cleanPath,
		Language:  languageFor(cleanPath),
		Content:   string(content),
		Truncated: truncated,
	}, nil
}

func (m *Module) resolve(relativePath string, allowRoot bool) (string, string, error) {
	if filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
		return "", "", fmt.Errorf("%w: absolute path", ErrInvalidPath)
	}
	if relativePath == "" {
		if !allowRoot {
			return "", "", fmt.Errorf("%w: file path is required", ErrInvalidPath)
		}
		return m.projectRoot, "", nil
	}
	cleanPath := filepath.Clean(relativePath)
	if cleanPath == "." {
		return "", "", fmt.Errorf("%w: root directory must use an empty path", ErrInvalidPath)
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("%w: parent path", ErrPathOutsideRoot)
	}
	candidatePath := filepath.Join(m.projectRoot, cleanPath)
	contained, err := isWithinRoot(m.projectRoot, candidatePath)
	if err != nil {
		return "", "", err
	}
	if !contained {
		return "", "", fmt.Errorf("%w: %q", ErrPathOutsideRoot, cleanPath)
	}
	resolvedPath, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve path %q: %w", cleanPath, err)
	}
	contained, err = isWithinRoot(m.projectRoot, resolvedPath)
	if err != nil {
		return "", "", err
	}
	if !contained {
		return "", "", fmt.Errorf("%w: %q", ErrPathOutsideRoot, cleanPath)
	}
	return resolvedPath, cleanPath, nil
}

func (m *Module) entryKind(entryPath string, entryInfo fs.FileInfo) (EntryKind, bool, error) {
	if entryInfo.Mode()&os.ModeSymlink != 0 {
		resolvedPath, err := filepath.EvalSymlinks(entryPath)
		if err != nil {
			return "", false, nil
		}
		contained, err := isWithinRoot(m.projectRoot, resolvedPath)
		if err != nil {
			return "", false, err
		}
		if !contained {
			return "", false, nil
		}
		entryInfo, err = os.Stat(resolvedPath)
		if err != nil {
			return "", false, nil
		}
	}
	if entryInfo.IsDir() {
		return EntryKindDirectory, true, nil
	}
	if entryInfo.Mode().IsRegular() {
		return EntryKindFile, true, nil
	}
	return "", false, nil
}

func isWithinRoot(projectRoot, candidatePath string) (bool, error) {
	relativePath, err := filepath.Rel(projectRoot, candidatePath)
	if err != nil {
		return false, fmt.Errorf("compare project paths: %w", err)
	}
	return relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) && !filepath.IsAbs(relativePath), nil
}

func languageFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".py":
		return "python"
	case ".sh":
		return "shellscript"
	default:
		return "plaintext"
	}
}
