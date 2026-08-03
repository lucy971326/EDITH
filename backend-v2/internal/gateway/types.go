// Package gateway 把渠道身份和会话转换为 EDITH 的运行输入。
package gateway

import (
	"edith/backend-v2/internal/agentrun"
	"edith/backend-v2/internal/agentstream"
)

const (
	WebChannel  = "web"
	CronChannel = "cron"
)

// IncomingMessage 是 Adapter 交给 Gateway 的渠道事实。
type IncomingMessage struct {
	Channel           string
	ExternalUserID    string
	SessionKey        string
	RequestID         string
	Message           string
	ImageIDs          []string
	UploadPaths       []string
	ModelID           string
	ReasoningOptionID string
}

type Error = agentrun.Error
type Stream = agentrun.Stream
type Status = agentrun.Status
type Event = agentstream.Event
