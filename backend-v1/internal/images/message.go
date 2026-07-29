package images

import (
	"context"
	"strings"
)

const referencePrefix = "edith-image://"

type messageImageURLsKey struct{}
type hydrateSessionKey struct{}

// Reference returns the durable image marker stored in framework Session
// events. It is never sent to a model provider directly.
func Reference(imageID string) string {
	return referencePrefix + imageID
}

// ImageIDFromReference reports whether value is an EDITH image marker.
func ImageIDFromReference(value string) (string, bool) {
	imageID := strings.TrimPrefix(value, referencePrefix)
	return imageID, imageID != value && imageID != ""
}

func withMessageImageURLs(ctx context.Context, urls map[string]string) context.Context {
	return context.WithValue(ctx, messageImageURLsKey{}, urls)
}

func imageIDForURL(ctx context.Context, url string) (string, bool) {
	urls, ok := ctx.Value(messageImageURLsKey{}).(map[string]string)
	if !ok {
		return "", false
	}
	imageID, ok := urls[url]
	return imageID, ok
}

// WithHydratedSession marks a Runner context. Only Runner reads hydrate
// Session image markers into short-lived COS URLs; HTTP history projection
// intentionally receives the durable markers instead.
func WithHydratedSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, hydrateSessionKey{}, true)
}

func shouldHydrateSession(ctx context.Context) bool {
	value, _ := ctx.Value(hydrateSessionKey{}).(bool)
	return value
}
