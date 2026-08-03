// Package agentrun 聚合一次 Agent 执行所需的配置并调用 ManagedRunner。
package agentrun

import "edith/backend-v2/internal/agentstream"

// Request 是已经完成身份与会话转换的执行输入。
type Request struct {
	RequestID         string
	UserID            string
	SessionID         string
	Message           string
	ImageIDs          []string
	UploadPaths       []string
	ModelID           string
	ReasoningOptionID string
}

// Error 描述任务启动、执行或控制失败。
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Stream 是 AgentRun 的唯一输出；调用方必须消费到 channel 关闭。
type Stream struct {
	Events <-chan agentstream.Event
}

// Status 是 ManagedRunner 仍在管理的活跃任务状态。
type Status struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
}
