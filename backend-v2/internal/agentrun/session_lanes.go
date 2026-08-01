package agentrun

import "sync"

// sessionLanes 保证同一用户的同一会话一次只运行一个任务。
type sessionLanes struct {
	mu     sync.Mutex
	active map[sessionKey]string
}

type sessionKey struct {
	userID    string
	sessionID string
}

func (lanes *sessionLanes) acquire(userID, sessionID, requestID string) bool {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()

	key := sessionKey{userID: userID, sessionID: sessionID}
	if _, exists := lanes.active[key]; exists {
		return false
	}
	lanes.active[key] = requestID
	return true
}

func (lanes *sessionLanes) release(userID, sessionID, requestID string) {
	lanes.mu.Lock()
	defer lanes.mu.Unlock()

	key := sessionKey{userID: userID, sessionID: sessionID}
	if lanes.active[key] == requestID {
		delete(lanes.active, key)
	}
}
