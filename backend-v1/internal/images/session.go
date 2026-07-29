package images

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	frameworksession "trpc.group/trpc-go/trpc-agent-go/session"
)

// WrapSessionService adds only image URL conversion to a normal framework
// SessionService. Its embedded Service preserves all other Session behavior.
func WrapSessionService(next frameworksession.Service, images *Service) frameworksession.Service {
	return &sessionService{Service: next, images: images}
}

type sessionService struct {
	frameworksession.Service
	images *Service
}

func (s *sessionService) GetSession(
	ctx context.Context,
	key frameworksession.Key,
	options ...frameworksession.Option,
) (*frameworksession.Session, error) {
	session, err := s.Service.GetSession(ctx, key, options...)
	if err != nil || session == nil || !shouldHydrateSession(ctx) {
		return session, err
	}
	return s.hydrateSession(ctx, session, key.UserID, key.SessionID)
}

func (s *sessionService) AppendEvent(
	ctx context.Context,
	session *frameworksession.Session,
	source *event.Event,
	options ...frameworksession.Option,
) error {
	// SQLite updates the Session passed to AppendEvent as well as persisting the
	// event. Give it an isolated Session copy containing the durable marker,
	// then update the Runner's in-memory Session with the original event. The
	// current model call must keep its short-lived https URL; only the database
	// may keep edith-image:// references.
	persistedSession := session.Clone()
	persistedEvent := dehydrateEvent(ctx, source)
	if err := s.Service.AppendEvent(ctx, persistedSession, persistedEvent, options...); err != nil {
		return err
	}
	session.UpdateUserSession(source, options...)
	return nil
}

func (s *sessionService) hydrateSession(
	ctx context.Context,
	source *frameworksession.Session,
	userID string,
	sessionID string,
) (*frameworksession.Session, error) {
	copy := *source
	copy.Events = make([]event.Event, len(source.Events))
	for index := range source.Events {
		hydrated, err := s.hydrateEvent(ctx, &source.Events[index], userID, sessionID)
		if err != nil {
			return nil, err
		}
		copy.Events[index] = *hydrated
	}
	return &copy, nil
}

func (s *sessionService) hydrateEvent(
	ctx context.Context,
	source *event.Event,
	userID string,
	sessionID string,
) (*event.Event, error) {
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

func (s *sessionService) hydrateMessage(
	ctx context.Context,
	source model.Message,
	userID string,
	sessionID string,
) (model.Message, error) {
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
		url, err := s.images.OpenForSession(ctx, userID, sessionID, imageID)
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
		message, didChange := dehydrateMessage(ctx, response.Choices[choiceIndex].Message)
		if !didChange {
			continue
		}
		response.Choices[choiceIndex].Message = message
		changed = true
	}
	if !changed {
		return source
	}
	copy.Response = response
	return &copy
}

func dehydrateMessage(ctx context.Context, source model.Message) (model.Message, bool) {
	message := source
	message.ContentParts = append([]model.ContentPart(nil), source.ContentParts...)
	changed := false
	for index := range message.ContentParts {
		part := &message.ContentParts[index]
		if part.Image == nil {
			continue
		}
		imageID, ok := imageIDForURL(ctx, part.Image.URL)
		if !ok {
			continue
		}
		image := *part.Image
		image.URL = Reference(imageID)
		part.Image = &image
		changed = true
	}
	return message, changed
}
