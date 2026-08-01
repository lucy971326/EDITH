package agentrun

import (
	"errors"

	"edith/backend-v2/internal/agentstream"
	"edith/backend-v2/internal/usage"
)

// startManagedRunner 记录运行并把已配置输入交给框架。
func (s *Service) startManagedRunner(configured *configuredRun) (*Stream, *Error) {
	if err := s.usage.Start(configured.ctx, configured.run); err != nil {
		configured.Close()
		s.lanes.release(configured.request.UserID, configured.request.SessionID, configured.request.RequestID)
		if errors.Is(err, usage.ErrRunAlreadyExists) {
			return nil, &Error{Type: "request_conflict", Message: "agent run already exists"}
		}
		return nil, internalError("start usage record", err)
	}

	rawEvents, err := s.runner.Run(
		configured.ctx,
		configured.request.UserID,
		configured.request.SessionID,
		configured.message,
		configured.options...,
	)
	if err != nil {
		_ = s.usage.Fail(configured.ctx, configured.request.RequestID)
		configured.Close()
		s.lanes.release(configured.request.UserID, configured.request.SessionID, configured.request.RequestID)
		return nil, internalError("start agent run", err)
	}

	events := make(chan agentstream.Event)
	go s.readFrameworkEvents(configured, rawEvents, events)
	return &Stream{Events: events}, nil
}
