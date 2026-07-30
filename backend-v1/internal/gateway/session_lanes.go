package gateway

import "sync"

// sessionLanes is a process-local single-execution guard. One user session
// may have one active Run; different sessions remain independent.
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
