// Package promptlog 记录发送给模型的框架层请求，方便本地检查 Prompt。
package promptlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/plugin"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Plugin 把每次模型请求写入用户级 Prompt 日志文件。
type Plugin struct {
	path     string
	sequence atomic.Uint64
	mu       sync.Mutex
}

// New 创建 Prompt 日志插件。日志写入 ~/.edith/logs/llm-prompts.log。
func New() (*Plugin, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find user home directory: %w", err)
	}
	return &Plugin{
		path: filepath.Join(homeDir, ".edith", "logs", "llm-prompts.log"),
	}, nil
}

// Name 返回 Runner 中唯一的插件名称。
func (p *Plugin) Name() string {
	return "prompt_log"
}

// Register 在模型调用前记录完整的框架层请求。
func (p *Plugin) Register(reg *plugin.Registry) {
	if p == nil || reg == nil {
		return
	}
	reg.BeforeModel(p.beforeModel)
}

func (p *Plugin) beforeModel(ctx context.Context, args *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
	if args == nil || args.Request == nil {
		return nil, nil
	}

	entry := requestEntry{
		Time:      time.Now(),
		Sequence:  p.sequence.Add(1),
		Request:   snapshotRequest(args.Request),
		ModelName: invocationModelName(ctx),
		SessionID: invocationSessionID(ctx),
	}
	_ = p.write(entry)
	return nil, nil
}

type requestEntry struct {
	Time      time.Time       `json:"time"`
	Sequence  uint64          `json:"sequence"`
	ModelName string          `json:"model_name,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Request   requestSnapshot `json:"request"`
}

type requestSnapshot struct {
	Messages         []model.Message         `json:"messages"`
	GenerationConfig model.GenerationConfig  `json:"generation_config,omitempty"`
	StructuredOutput *model.StructuredOutput `json:"structured_output,omitempty"`
	ExtraFields      map[string]any          `json:"extra_fields,omitempty"`
	Tools            []tool.Declaration      `json:"tools,omitempty"`
}

func snapshotRequest(request *model.Request) requestSnapshot {
	return requestSnapshot{
		Messages:         request.Messages,
		GenerationConfig: request.GenerationConfig,
		StructuredOutput: request.StructuredOutput,
		ExtraFields:      request.ExtraFields,
		Tools:            toolDeclarations(request.Tools),
	}
}

func toolDeclarations(tools map[string]tool.Tool) []tool.Declaration {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	declarations := make([]tool.Declaration, 0, len(names))
	for _, name := range names {
		if tools[name] == nil || tools[name].Declaration() == nil {
			continue
		}
		declarations = append(declarations, *tools[name].Declaration())
	}
	return declarations
}

func invocationModelName(ctx context.Context) string {
	invocation, ok := agent.InvocationFromContext(ctx)
	if !ok || invocation == nil {
		return ""
	}
	return invocation.RunOptions.ModelName
}

func invocationSessionID(ctx context.Context) string {
	invocation, ok := agent.InvocationFromContext(ctx)
	if !ok || invocation == nil || invocation.Session == nil {
		return ""
	}
	return invocation.Session.ID
}

func (p *Plugin) write(entry requestEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "\n%s\n", "======================================================================"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "模型请求 #%d  %s\n", entry.Sequence, entry.Time.Format("2006-01-02 15:04:05")); err != nil {
		return err
	}
	if entry.ModelName != "" {
		if _, err := fmt.Fprintf(file, "模型: %s\n", entry.ModelName); err != nil {
			return err
		}
	}
	if entry.SessionID != "" {
		if _, err := fmt.Fprintf(file, "会话: %s\n", entry.SessionID); err != nil {
			return err
		}
	}

	return writeJSON(file, entry.Request)
}

func writeJSON(file *os.File, value any) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	_, err := file.WriteString("\n[模型请求]\n")
	if err != nil {
		return err
	}
	return encoder.Encode(value)
}
