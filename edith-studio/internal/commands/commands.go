package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"edith/studio/internal/engine"
)

// Dependencies 是创建命令模块所需的长期产品能力。
type Dependencies struct {
	// Engine 是命令需要调用的 Agent 核心能力。
	Engine *engine.Engine
}

// Module 提供命令目录，并把命令转给对应的产品能力。
type Module struct {
	engine *engine.Engine
}

// New 使用已经组装好的 Engine 创建命令模块。
func New(dependencies Dependencies) (*Module, error) {
	if dependencies.Engine == nil {
		return nil, errors.New("commands dependencies are incomplete")
	}
	return &Module{engine: dependencies.Engine}, nil
}

// List 返回当前版本支持的 Slash Command 目录。
func (m *Module) List() []Definition {
	return []Definition{
		{
			Name:        "compact",
			Description: "压缩当前会话上下文",
			Syntax:      "/compact",
		},
	}
}

// Execute 解析并执行一个 Slash Command。
func (m *Module) Execute(ctx context.Context, input Input) error {
	command, err := parse(input.Command)
	if err != nil {
		return err
	}
	switch command {
	case "compact":
		return m.engine.Compact(ctx, engine.CompactInput{
			SessionID:    input.SessionID,
			ModelID:      input.ModelID,
			ThinkingMode: input.ThinkingMode,
		})
	default:
		return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
}

func parse(raw string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return "", ErrInvalidCommand
	}
	if len(fields) != 1 {
		return "", fmt.Errorf("%w: command arguments are not supported", ErrInvalidCommand)
	}
	name := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	if name == "" {
		return "", ErrInvalidCommand
	}
	if name != "compact" {
		return "", fmt.Errorf("%w: %s", ErrUnknownCommand, name)
	}
	return name, nil
}
