package sandbox

// FileTreeResponse 是工作区目录的只读树节点列表。
type FileTreeResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// FileContentResponse 是小型 UTF-8 文本文件的预览结果。
type FileContentResponse struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}
