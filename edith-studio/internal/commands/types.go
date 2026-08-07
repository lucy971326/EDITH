// Package commands 管理 Studio 产品层的 Slash Command。
package commands

import "errors"

var (
	// ErrInvalidCommand 表示输入不是合法的 Slash Command。
	ErrInvalidCommand = errors.New("invalid command")
	// ErrUnknownCommand 表示命令目录中不存在该命令。
	ErrUnknownCommand = errors.New("unknown command")
)

// Definition 是一个命令的前端展示说明，不负责执行命令。
type Definition struct {
	// Name 是命令的稳定名称，不包含开头的斜杠。
	Name string `json:"name"`
	// Description 是命令在 Web 提示中的简短说明。
	Description string `json:"description"`
	// Syntax 是用户应该输入的完整命令格式。
	Syntax string `json:"syntax"`
}

// Input 是 Web 请求执行一个命令时提供的输入。
type Input struct {
	// SessionID 是命令作用的会话。
	SessionID string `json:"sessionId"`
	// Command 是用户输入的原始 Slash Command。
	Command string `json:"command"`
	// ModelID 是当前 Composer 选中的模型。
	ModelID string `json:"modelId,omitempty"`
	// ThinkingMode 是当前 Composer 选中的思考模式。
	ThinkingMode string `json:"thinkingMode,omitempty"`
}

// Result 是命令执行成功后的简短状态。
type Result struct {
	// Command 是已经执行的命令名称，不包含斜杠。
	Command string `json:"command"`
	// Status 是固定的完成状态，后续可以扩展为其他命令状态。
	Status string `json:"status"`
}
