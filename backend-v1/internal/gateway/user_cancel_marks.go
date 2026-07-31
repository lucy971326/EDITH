package gateway

import "sync"

// userCancelMarks 暂存“用户主动停止过”的活跃任务 ID。
// ManagedRunner 负责真正取消任务；本结构只在任务最终结束时帮助 message.go 输出
// run.canceled，而不是将主动停止误判成 run.error。
type userCancelMarks struct {
	mu         sync.Mutex
	requestIDs map[string]struct{}
}

func newUserCancelMarks() *userCancelMarks {
	return &userCancelMarks{requestIDs: make(map[string]struct{})}
}

func (m *userCancelMarks) mark(requestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestIDs[requestID] = struct{}{}
}

func (m *userCancelMarks) marked(requestID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.requestIDs[requestID]
	return ok
}

func (m *userCancelMarks) take(requestID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.requestIDs[requestID]
	delete(m.requestIDs, requestID)
	return ok
}
