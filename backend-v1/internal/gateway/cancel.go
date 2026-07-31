package gateway

// Cancel 停止一个活跃任务。
// 输入：可信的用户 ID 与任务 ID。
// 输出：ManagedRunner 接受取消信号时返回 nil；任务不存在或不属于该用户时返回 not_found。
func (s *Gateway) Cancel(userID, requestID string) *APIError {
	return s.onlyRun.Cancel(userID, requestID)
}
