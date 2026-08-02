package skills

import (
	"net/http"
	"strings"

	"edith/backend-v2/internal/httpx"
)

// HTTP 是 Skills 模块对 Web BFF 公开的只读能力。
type HTTP struct {
	catalog *Catalog
}

// Register 注册 Skills 列表接口。
func (h *HTTP) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/skills", h.listSkills)
}

// listSkills 返回公共和当前用户的 Skill 摘要。
func (h *HTTP) listSkills(writer http.ResponseWriter, request *http.Request) {
	userID := strings.TrimSpace(request.URL.Query().Get("userId"))
	if userID == "" {
		httpx.WriteError(writer, http.StatusBadRequest, "invalid_request", "userId is required")
		return
	}

	custom, err := h.catalog.ListUserSummaries(request.Context(), userID)
	if err != nil {
		httpx.WriteError(writer, http.StatusInternalServerError, "skills_load_failed", "load user skills failed")
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, SkillsResponse{
		System: skillListItems(h.catalog.ListSystemSummaries()),
		Custom: skillListItems(custom),
	})
}

func skillListItems(summaries []SkillSummary) []SkillListItem {
	items := make([]SkillListItem, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, SkillListItem{Name: summary.Name, Description: summary.Description})
	}
	return items
}
