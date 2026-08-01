package images

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

const referencePrefix = "edith-image://"

type messageImageURLsKey struct{}
type hydrateSessionKey struct{}

// AgentInput 在 AgentRun 组装一次模型调用时添加用户选中的图片。
type AgentInput struct{ service *service }

// AddMessageImages 把已确认图片转换为本次模型调用可访问的短期 URL。
func (i *AgentInput) AddMessageImages(ctx context.Context, userID, sessionID string, imageIDs []string, message *model.Message) (context.Context, error) {
	if len(imageIDs) == 0 {
		return ctx, nil
	}
	if message == nil {
		return nil, errors.New("message is required")
	}

	urls := make(map[string]string, len(imageIDs))
	seen := make(map[string]struct{}, len(imageIDs))
	for _, imageID := range imageIDs {
		imageID = strings.TrimSpace(imageID)
		if imageID == "" {
			return nil, errors.New("imageID is required")
		}
		if _, duplicated := seen[imageID]; duplicated {
			return nil, fmt.Errorf("image %q was supplied more than once", imageID)
		}
		seen[imageID] = struct{}{}
		url, err := i.service.openForSession(ctx, userID, sessionID, imageID)
		if err != nil {
			return nil, err
		}
		message.AddImageURL(url, "")
		urls[url] = imageID
	}
	return context.WithValue(ctx, messageImageURLsKey{}, urls), nil
}

// Reference 返回写入框架会话历史的持久图片标记。
func Reference(imageID string) string { return referencePrefix + imageID }

// ImageIDFromReference 从持久图片标记取回图片 ID。
func ImageIDFromReference(value string) (string, bool) {
	imageID := strings.TrimPrefix(value, referencePrefix)
	return imageID, imageID != value && imageID != ""
}

// WithHydratedSession 标记本次 Runner 读取会话时需要把图片标记替换为 COS URL。
func WithHydratedSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, hydrateSessionKey{}, true)
}

func imageIDForRuntimeURL(ctx context.Context, url string) (string, bool) {
	urls, ok := ctx.Value(messageImageURLsKey{}).(map[string]string)
	if !ok {
		return "", false
	}
	imageID, ok := urls[url]
	return imageID, ok
}

func shouldHydrateSession(ctx context.Context) bool {
	value, _ := ctx.Value(hydrateSessionKey{}).(bool)
	return value
}
