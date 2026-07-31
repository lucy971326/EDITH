package gateway

// RunStatus 查询一个活跃任务。
// 输入：可信的用户 ID 与任务 ID。
// 输出：任务仍由 ManagedRunner 管理时返回 running；否则返回 not_found。
func (s *Gateway) RunStatus(userID, requestID string) (RunStatusResponse, *APIError) {
	return s.onlyRun.RunStatus(userID, requestID)
}
