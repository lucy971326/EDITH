//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//
//

package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-agent-go/log"
	"trpc.group/trpc-go/trpc-agent-go/model"
	trunner "trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/server/agui/adapter"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// EnqueueUserMessageInput carries the parameters for an enqueue user message request.
type EnqueueUserMessageInput struct {
	// ThreadID is the conversation thread (session) identifier.
	ThreadID string `json:"threadId"`
	// RunID is the active run identifier, used as Core Runner requestID.
	RunID string `json:"runId"`
	// Message is the user message to enqueue (AG-UI type).
	Message types.Message `json:"message"`
}

// SteerableRunner extends Runner with the ability to enqueue user messages
// into an active run. Both the Core Runner's EnqueueUserMessage and the
// AG-UI Tracker's user_message TrackEvent are written, so the appended
// message appears in both the live execution and the AG-UI history.
type SteerableRunner interface {
	Runner

	// EnqueueUserMessage queues a user message into the active run identified
	// by RunID. It also records the message as a AG-UI user_message TrackEvent
	// so that MessagesSnapshot shows it as an independent user message.
	EnqueueUserMessage(ctx context.Context, input *EnqueueUserMessageInput) error
}

// EnqueueUserMessage implements SteerableRunner.
//
// It performs the following steps:
//  1. Resolves appName and userID using the runner's configured resolvers.
//  2. Builds a session.Key for track event writing.
//  3. Converts the AG-UI types.Message to a model.Message for the Core Runner.
//  4. Calls Core Runner's EnqueueUserMessage (the critical path).
//  5. Records the user_message TrackEvent via recordUserMessage (best-effort).
//
// Order: Core Enqueue first, then Track write. If Core succeeds but Track
// fails, the message is still delivered to the run — the Track failure is
// logged as a warning but not returned to the caller, because there is no
// way to undo a Core Enqueue. If Core fails, no Track is written at all.
func (r *runner) EnqueueUserMessage(
	ctx context.Context,
	input *EnqueueUserMessageInput,
) error {
	if r.runner == nil {
		return errors.New("runner is nil")
	}
	if input == nil {
		return errors.New("input cannot be nil")
	}
	if input.RunID == "" {
		return errors.New("run id is required")
	}
	if input.ThreadID == "" {
		return errors.New("thread id is required")
	}

	// Resolve appName and userID using the same resolver chain Run uses.
	agentInput := &adapter.RunAgentInput{
		ThreadID: input.ThreadID,
		RunID:    input.RunID,
	}
	appName, err := r.resolveAppName(ctx, agentInput)
	if err != nil {
		return fmt.Errorf("resolve app name: %w", err)
	}
	userID, err := r.userIDResolver(ctx, agentInput)
	if err != nil {
		return fmt.Errorf("resolve user ID: %w", err)
	}
	key := session.Key{
		AppName:   appName,
		UserID:    userID,
		SessionID: input.ThreadID,
	}

	// Ensure the message has an ID so the tracker event is well-formed.
	msg := input.Message
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}

	// Convert AG-UI types.Message to model.Message for the Core Runner.
	coreMsg, err := convertToEnqueueModelMessage(&msg)
	if err != nil {
		return fmt.Errorf("convert message: %w", err)
	}

	// Step 1: Core Enqueue (critical path — if this fails, stop here).
	steerable, ok := r.runner.(trunner.SteerableRunner)
	if !ok {
		return errors.New("core runner does not support EnqueueUserMessage")
	}
	if err := steerable.EnqueueUserMessage(input.RunID, coreMsg); err != nil {
		return err
	}

	// Step 2: Write AG-UI user_message TrackEvent (best-effort after Core success).
	if r.tracker == nil {
		log.WarnContext(ctx, "agui enqueue user message: tracker is nil, "+
			"core message enqueued but track event not recorded")
		return nil
	}
	if err := r.recordUserMessage(ctx, key, &msg); err != nil {
		// Core already succeeded; there is no rollback. Log and return nil
		// so the caller does not retry a Core operation that already worked.
		log.WarnfContext(ctx,
			"agui enqueue user message: core succeeded but failed to "+
				"record track event: %v", err)
	}
	return nil
}

// convertToEnqueueModelMessage converts an AG-UI types.Message (user role)
// to a model.Message for use with Core Runner's EnqueueUserMessage.
// For simple text messages this is a straightforward conversion.
func convertToEnqueueModelMessage(msg *types.Message) (model.Message, error) {
	content, ok := msg.ContentString()
	if !ok {
		return model.Message{}, fmt.Errorf(
			"message content must be a string for enqueue")
	}
	return model.Message{
		Role:    model.RoleUser,
		Content: content,
	}, nil
}
