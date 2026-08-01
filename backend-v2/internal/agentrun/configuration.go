package agentrun

import (
	"context"
	"fmt"
	"log"

	"edith/backend-v2/internal/images"
	"edith/backend-v2/internal/models"
	"edith/backend-v2/internal/usage"
	"edith/backend-v2/internal/userconfig"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// runConfigurations 聚合模型、用户设置、MCP 和图片配置。
type runConfigurations struct {
	models    *models.Catalog
	settings  *userconfig.Settings
	providers *userconfig.Providers
	mcp       *userconfig.MCP
	images    *images.AgentInput
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
	message := model.NewUserMessage(request.Message)
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

	run := usage.Run{
		RequestID: request.RequestID,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		ModelID:   request.ModelID,
	}
	options := frameworkRunOptions(runOptionInput{
		requestID:         request.RequestID,
		modelID:           request.ModelID,
		apiKey:            apiKey,
		globalInstruction: "你是 EDITH AI Agent智能助手\n\n" + personality,
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
