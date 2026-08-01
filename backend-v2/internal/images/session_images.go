package images

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
)

// SessionImages 让框架会话历史安全保存图片持久标记。
type SessionImages struct{ service *service }

// Wrap 为框架 Session Service 加入图片读写转换。
func (s *SessionImages) Wrap(next frameworksession.Service) frameworksession.Service {
	return &imageSessionService{Service: next, images: s.service}
}

type imageSessionService struct {
	frameworksession.Service
	images *service
}

func (s *imageSessionService) GetSession(ctx context.Context, key frameworksession.Key, options ...frameworksession.Option) (*frameworksession.Session, error) {
	session, err := s.Service.GetSession(ctx, key, options...)
	if err != nil || session == nil || !shouldHydrateSession(ctx) {
		return session, err
	}
	copy := session.Clone()
	copy.Events = make([]event.Event, len(session.Events))
	for index := range session.Events {
		hydrated, err := s.hydrateEvent(ctx, &session.Events[index], key.UserID, key.SessionID)
		if err != nil {
			return nil, err
		}
		copy.Events[index] = *hydrated
	}
	return copy, nil
}

func (s *imageSessionService) AppendEvent(ctx context.Context, session *frameworksession.Session, source *event.Event, options ...frameworksession.Option) error {
	persistedSession := session.Clone()
	if err := s.Service.AppendEvent(ctx, persistedSession, dehydrateEvent(ctx, source), options...); err != nil {
		return err
	}
	session.UpdateUserSession(source, options...)
	return nil
}

func (s *imageSessionService) hydrateEvent(ctx context.Context, source *event.Event, userID, sessionID string) (*event.Event, error) {
	if source == nil || source.Response == nil {
		return source, nil
	}
	copy := *source
	response := source.Response.Clone()
	for choiceIndex := range response.Choices {
		message, err := s.hydrateMessage(ctx, response.Choices[choiceIndex].Message, userID, sessionID)
		if err != nil {
			return nil, err
		}
		response.Choices[choiceIndex].Message = message
	}
	copy.Response = response
	return &copy, nil
}

func (s *imageSessionService) hydrateMessage(ctx context.Context, source model.Message, userID, sessionID string) (model.Message, error) {
	message := source
	message.ContentParts = append([]model.ContentPart(nil), source.ContentParts...)
	for index := range message.ContentParts {
		part := &message.ContentParts[index]
		if part.Image == nil {
			continue
		}
		imageID, ok := ImageIDFromReference(part.Image.URL)
		if !ok {
			continue
		}
		url, err := s.images.openForSession(ctx, userID, sessionID, imageID)
		if err != nil {
			return model.Message{}, fmt.Errorf("hydrate image %q: %w", imageID, err)
		}
		image := *part.Image
		image.URL = url
		part.Image = &image
	}
	return message, nil
}

func dehydrateEvent(ctx context.Context, source *event.Event) *event.Event {
	if source == nil || source.Response == nil {
		return source
	}
	copy := *source
	response := source.Response.Clone()
	changed := false
	for choiceIndex := range response.Choices {
		message := response.Choices[choiceIndex].Message
		message.ContentParts = append([]model.ContentPart(nil), message.ContentParts...)
		for index := range message.ContentParts {
			part := &message.ContentParts[index]
			if part.Image == nil {
				continue
			}
			imageID, ok := imageIDForRuntimeURL(ctx, part.Image.URL)
			if !ok {
				continue
			}
			image := *part.Image
			image.URL = Reference(imageID)
			part.Image = &image
			changed = true
		}
		response.Choices[choiceIndex].Message = message
	}
	if !changed {
		return source
	}
	copy.Response = response
	return &copy
}
