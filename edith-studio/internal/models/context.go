package models

import "context"

type selectionContextKey struct{}

// WithSelection 将本次 Run 的模型选择放入请求 context，供会话摘要器读取。
func WithSelection(ctx context.Context, selection Selection) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, selectionContextKey{}, selection)
}

// SelectionFromContext 读取请求 context 中的模型选择。
func SelectionFromContext(ctx context.Context) (Selection, bool) {
	if ctx == nil {
		return Selection{}, false
	}
	selection, ok := ctx.Value(selectionContextKey{}).(Selection)
	return selection, ok
}
