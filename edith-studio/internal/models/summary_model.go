package models

import (
	"context"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

// SummaryModel 返回本次选择对应的摘要模型能力。
// 它复用启动时创建的供应商实例；思考参数只通过轻量请求适配注入。
func (m *Module) SummaryModel(selection Selection) (model.Model, error) {
	modelID, entry, thinkingMode, err := m.resolveSelection(selection)
	if err != nil {
		return nil, err
	}
	baseModel := m.models[modelID]
	fields := thinkingFields(entry, thinkingMode)
	if len(fields) == 0 {
		return baseModel, nil
	}
	return &requestFieldsModel{base: baseModel, fields: fields}, nil
}

type requestFieldsModel struct {
	base   model.Model
	fields map[string]any
}

func (m *requestFieldsModel) GenerateContent(ctx context.Context, request *model.Request) (<-chan *model.Response, error) {
	if request.ExtraFields == nil {
		request.ExtraFields = make(map[string]any, len(m.fields))
	}
	for key, value := range m.fields {
		request.ExtraFields[key] = value
	}
	return m.base.GenerateContent(ctx, request)
}

func (m *requestFieldsModel) Info() model.Info {
	return m.base.Info()
}
