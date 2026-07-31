package onlyrun

import "sync"

// sessionLanes 是进程内的同会话单实例旁路规则。
// 它只决定任务能否进入 OnlyRun，并在任务收尾后释放，不参与事件转换和渠道输出。
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
