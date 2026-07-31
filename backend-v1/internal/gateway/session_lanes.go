package gateway

import "sync"

// sessionLanes 是进程内的同会话单实例规则。
// 输入是用户、会话和请求 ID；同一用户的同一会话只允许一个活跃 Run，其他会话互不影响。
type sessionLanes struct {
	mu     sync.Mutex
	active map[sessionKey]string
}

type sessionKey struct {
	userID    string
	sessionID string
}

func newSessionLanes() *sessionLanes {
	return &sessionLanes{active: make(map[sessionKey]string)}
}

func (l *sessionLanes) tryAcquire(userID, sessionID, requestID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := sessionKey{userID: userID, sessionID: sessionID}
	if _, exists := l.active[key]; exists {
		return false
	}
	l.active[key] = requestID
	return true
}

func (l *sessionLanes) release(userID, sessionID, requestID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := sessionKey{userID: userID, sessionID: sessionID}
	if l.active[key] == requestID {
		delete(l.active, key)
	}
}
