package webapi

import "sync"

// ActiveSessionRuns enforces EDITH's single-instance rule: one user session
// may own at most one Agent Run at a time. It is a process-level concurrency
// guard, separate from ManagedRunner's requestID control plane.
type ActiveSessionRuns struct {
	mu         sync.Mutex
	requestIDs map[sessionRunKey]string
}

type sessionRunKey struct {
	userID    string
	sessionID string
}

// NewActiveSessionRuns creates the process-wide session Run registry.
func NewActiveSessionRuns() *ActiveSessionRuns {
	return &ActiveSessionRuns{
		requestIDs: map[sessionRunKey]string{},
	}
}

// TryAcquire reserves one user session for requestID. It returns false when
// that session already has a Run that has not finished its handler cleanup.
func (r *ActiveSessionRuns) TryAcquire(userID, sessionID, requestID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := sessionRunKey{userID: userID, sessionID: sessionID}
	if _, exists := r.requestIDs[key]; exists {
		return false
	}
	r.requestIDs[key] = requestID
	return true
}

// Release frees a session only when the caller still owns its reservation.
// The requestID check prevents an old cleanup path from clearing a newer Run.
func (r *ActiveSessionRuns) Release(userID, sessionID, requestID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := sessionRunKey{userID: userID, sessionID: sessionID}
	if r.requestIDs[key] == requestID {
		delete(r.requestIDs, key)
	}
}
