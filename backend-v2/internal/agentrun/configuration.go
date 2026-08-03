package agentrun

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"

	"edith/backend-v2/internal/images"
	"edith/backend-v2/internal/models"
	"edith/backend-v2/internal/sandbox"
	"edith/backend-v2/internal/skills"
	"edith/backend-v2/internal/usage"
	"edith/backend-v2/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// runConfigurations 聚合模型、用户设置、MCP、图片、上传文件和 Skills 配置。
type runConfigurations struct {
	models    *models.Catalog
	settings  *userconfig.Settings
	providers *userconfig.Providers
	mcp       *userconfig.MCP
	images    *images.AgentInput
	files     *sandbox.AgentInput
	skills    *skills.Catalog
}

// configuredRun 是已经可以交给 ManagedRunner 的一次运行。
type configuredRun struct {
	ctx        context.Context
	request    Request
	definition models.Definition
	run        usage.Run
	message    model.Message
	options    []agent.RunOption
	closeMCP   func() error
}

// Load 从各功能模块加载配置，并返回完整的框架运行输入。
func (c *runConfigurations) Load(request Request) (*configuredRun, *Error) {
	ctx := context.Background()
	if request.ModelID == "" {
		modelID, err := c.settings.LoadDefaultModelID(ctx, request.UserID)
		if err != nil {
			return nil, internalError("load default model", err)
		}
		request.ModelID = modelID
	}
	if request.ModelID == "" {
		request.ModelID = models.DefaultModelID
	}
	definition, found := c.models.Find(request.ModelID)
	if !found {
		return nil, &Error{Type: "invalid_request", Message: "unsupported modelId"}
	}
	if len(request.ImageIDs) > 0 && !definition.Info.Capabilities.Vision {
		return nil, &Error{Type: "invalid_request", Message: "selected model does not support image input"}
	}

	apiKey, err := c.providers.LoadAPIKey(ctx, request.UserID, definition.ProviderID)
	if err != nil {
		return nil, &Error{Type: "invalid_request", Message: fmt.Sprintf("load model credential: %v", err)}
	}
	personality, err := c.settings.LoadPersonality(ctx, request.UserID)
	if err != nil {
		return nil, internalError("load user personality", err)
	}
	uploads, err := c.files.ValidateUploads(ctx, request.UserID, request.SessionID, request.UploadPaths)
	if err != nil {
		return nil, &Error{Type: "invalid_request", Message: fmt.Sprintf("prepare uploaded files: %v", err)}
	}
	message := model.NewUserMessage(messageWithUploads(request.Message, uploads))
	ctx = images.WithHydratedSession(ctx)
	if len(request.ImageIDs) > 0 {
		ctx, err = c.images.AddMessageImages(ctx, request.UserID, request.SessionID, request.ImageIDs, &message)
		if err != nil {
			return nil, &Error{Type: "invalid_request", Message: fmt.Sprintf("prepare message images: %v", err)}
		}
	}
	mcpTools, closeMCP, err := c.mcp.OpenTools(ctx, request.UserID)
	if err != nil {
		return nil, internalError("open MCP tools", err)
	}
	userOverview, err := c.skills.ReadUserOverview(ctx, request.UserID)
	if err != nil {
		if closeMCP != nil {
			_ = closeMCP()
		}
		return nil, internalError("read user skill overview", err)
	}

	run := usage.Run{
		RequestID: request.RequestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	options := frameworkRunOptions(runOptionInput{
		requestID:         request.RequestID,
		modelID:           request.ModelID,
		contextWindow:     definition.ContextWindow,
		apiKey:            apiKey,
		globalInstruction: "你是 EDITH AI Agent智能助手\n\n" + personality,
		instruction:       runInstruction(c.skills.ListSystemSummaries(), userOverview, uploads),
		additionalTools:   mcpTools,
	})
	return &configuredRun{
		ctx:        ctx,
		request:    request,
		definition: definition,
		run:        run,
		message:    message,
		options:    options,
		closeMCP:   closeMCP,
	}, nil
}

// messageWithUploads 把附件名称写入用户消息，保证会话历史也能展示本次上传。
func messageWithUploads(content string, uploads []string) string {
	content = strings.TrimSpace(content)
	if len(uploads) == 0 {
		return content
	}
	files := make([]string, 0, len(uploads))
	for _, upload := range uploads {
		files = append(files, "- `"+path.Base(upload)+"`")
	}
	attachments := "附件：\n" + strings.Join(files, "\n")
	if content == "" {
		return attachments
	}
	return content + "\n\n" + attachments
}

// runInstruction 将本次已验证的上传文件附加到 Agent 指令，路径只可由 Sandbox 提供。
func runInstruction(summaries []skills.SkillSummary, overview string, uploads []string) string {
	instruction := skillInstruction(summaries, overview)
	if len(uploads) == 0 {
		return instruction
	}
	files := "本次用户上传文件：\n- " + strings.Join(uploads, "\n- ") + "\n请通过 Sandbox 文件工具从 uploads/ 读取这些文件。"
	if instruction == "" {
		return files
	}
	return instruction + "\n\n" + files
}

// skillInstruction 把公共和用户 Skill 摘要拼成一次运行的 Instruction。
// 这里只注入摘要，完整 Skill 正文留给 Agent 通过 Sandbox 按需加载。
func skillInstruction(summaries []skills.SkillSummary, userOverview string) string {
	if len(summaries) == 0 && strings.TrimSpace(userOverview) == "" {
		return ""
	}
	lines := make([]string, 0, len(summaries)+4)
	if len(summaries) > 0 {
		lines = append(lines, "可用公共 Skills：")
	}
	for _, summary := range summaries {
		lines = append(lines, fmt.Sprintf("- %s：%s", summary.Name, summary.Description))
	}
	if overview := strings.TrimSpace(userOverview); overview != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "可用用户 Skills：", overview)
	}
	lines = append(lines, fmt.Sprintf("完整 Skill 文件和资源位于 Sandbox 工作区：公共 Skills %s/<skill-name>/，用户 Skills %s/<skill-name>/。需要完整规则或资源时，通过 Sandbox 文件工具读取对应目录。", skills.SystemPath, skills.CustomPath))
	return strings.Join(lines, "\n")
}

// Close 释放配置加载阶段建立的 MCP 连接。
func (run *configuredRun) Close() {
	if run.closeMCP != nil {
		if err := run.closeMCP(); err != nil {
			log.Printf("关闭任务 %q 的 MCP 工具: %v", run.request.RequestID, err)
		}
	}
}

func internalError(action string, err error) *Error {
	return &Error{Type: "internal_error", Message: fmt.Sprintf("%s: %v", action, err)}
}
