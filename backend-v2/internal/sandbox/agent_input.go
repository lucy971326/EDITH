package sandbox

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/eric642/e2b-go-sdk"
)

// AgentInput 校验本次运行可引用的用户上传文件，不暴露 Workspace 实现。
type AgentInput struct{ workspaces *service }

// ValidateUploads 只接受当前用户、当前会话 uploads/ 内已存在的普通文件。
func (a *AgentInput) ValidateUploads(ctx context.Context, userID, sessionID string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	workspace, err := a.workspaces.ExistingWorkspace(ctx, strings.TrimSpace(userID), strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("open upload workspace: %w", err)
	}
	validated := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		relative := strings.TrimSpace(strings.ReplaceAll(value, `\\`, "/"))
		if !strings.HasPrefix(relative, Workspace.Uploads+"/") || path.Clean(relative) != relative || strings.Contains(relative, "..") {
			return nil, fmt.Errorf("file must be inside %s/", Workspace.Uploads)
		}
		if _, ok := seen[relative]; ok {
			continue
		}
		info, statErr := workspace.Files.Stat(ctx, path.Join(Workspace.Root, relative), e2b.FsOptions{})
		if statErr != nil || info.Type == e2b.EntryTypeDirectory {
			return nil, fmt.Errorf("uploaded file does not exist: %s", relative)
		}
		seen[relative] = struct{}{}
		validated = append(validated, relative)
	}
	return validated, nil
}
