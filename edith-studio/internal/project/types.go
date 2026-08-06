// Package project 提供当前项目目录的受限文件读取能力。
package project

import "errors"

// EntryKind 表示文件树条目的真实文件系统类型。
type EntryKind string

const (
	// EntryKindDirectory 表示目录。
	EntryKindDirectory EntryKind = "directory"
	// EntryKindFile 表示普通文件。
	EntryKindFile EntryKind = "file"
)

var (
	// ErrInvalidPath 表示输入不是允许的项目相对路径。
	ErrInvalidPath = errors.New("project path must be relative")
	// ErrPathOutsideRoot 表示路径或其符号链接目标逃出了项目根目录。
	ErrPathOutsideRoot = errors.New("project path is outside project root")
	// ErrNotDirectory 表示请求列举的目标不是目录。
	ErrNotDirectory = errors.New("project path is not a directory")
	// ErrNotRegularFile 表示请求读取的目标不是普通文件。
	ErrNotRegularFile = errors.New("project path is not a regular file")
	// ErrNotTextFile 表示普通文件包含二进制内容，不能作为文本返回。
	ErrNotTextFile = errors.New("project file is not text")
)

// FileEntry 是一个目录直接包含的文件或目录。
type FileEntry struct {
	// Path 是相对 ProjectRoot 的安全路径。
	Path string `json:"path"`
	// Name 是该条目的文件名。
	Name string `json:"name"`
	// Kind 区分目录与普通文件。
	Kind EntryKind `json:"kind"`
}

// FileContent 是一次文本文件读取的结果。
type FileContent struct {
	// Path 是相对 ProjectRoot 的安全路径。
	Path string `json:"path"`
	// Language 是由文件扩展名推断的基础代码语言。
	Language string `json:"language"`
	// Content 是读取到的文本内容。
	Content string `json:"content"`
	// Truncated 表示文件超过首版返回上限，Content 仅包含文件前缀。
	Truncated bool `json:"truncated"`
}
