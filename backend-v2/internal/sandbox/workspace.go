package sandbox

import (
	"fmt"
	"path"
)

// WorkspaceLayout 是每个 E2B Sandbox 的固定目录约定。
type WorkspaceLayout struct{ Root, Uploads, Work, Artifacts string }

var Workspace = WorkspaceLayout{Root: "/home/user", Uploads: "uploads", Work: "work", Artifacts: "artifacts"}

func (l WorkspaceLayout) WorkPath() string { return path.Join(l.Root, l.Work) }
func (l WorkspaceLayout) ToolGuide() string {
	return fmt.Sprintf("Sandbox 根目录是 %s。路径必须相对此目录；%s/ 放上传文件，%s/ 放处理文件，%s/ 放最终交付文件。不能使用绝对路径或 ..。", l.Root, l.Uploads, l.Work, l.Artifacts)
}
