package sandbox

import (
	"fmt"
	"path"
)

// Workspace is EDITH's fixed directory contract inside every E2B sandbox.
// Tools use this value for both filesystem paths and model-facing guidance.
var Workspace = WorkspaceLayout{
	Root:      "/home/user",
	Uploads:   "uploads",
	Work:      "work",
	Artifacts: "artifacts",
}

// WorkspaceLayout names EDITH's directories inside one session sandbox.
type WorkspaceLayout struct {
	Root      string
	Uploads   string
	Work      string
	Artifacts string
}

func (l WorkspaceLayout) UploadsPath() string {
	return path.Join(l.Root, l.Uploads)
}

func (l WorkspaceLayout) WorkPath() string {
	return path.Join(l.Root, l.Work)
}

func (l WorkspaceLayout) ArtifactsPath() string {
	return path.Join(l.Root, l.Artifacts)
}

// ToolGuide explains EDITH's workspace contract to the model. It is built
// from the same layout value that the tools use at runtime.
func (l WorkspaceLayout) ToolGuide() string {
	return fmt.Sprintf(`Sandbox 工作区根目录是 %s，所有路径都必须相对此目录填写。
- %s/：用户上传的原始输入文件。
- %s/：处理过程和临时文件；命令与进程默认在此执行。
- %s/：最终交付给用户的文件；需要保留或下载的结果必须写入这里。
不能使用绝对路径或 ..。`, l.Root, l.Uploads, l.Work, l.Artifacts)
}
