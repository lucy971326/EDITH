package gateway

import "sync"

// cancelTracker records an accepted user cancellation until the corresponding
// framework completion event arrives. ManagedRunner remains the control-plane
// truth; this only gives the terminal Gateway event an honest name.
type cancelTracker struct {
	mu         sync.Mutex
	requestIDs map[string]struct{}
}

func newCancelTracker() *cancelTracker {
	return &cancelTracker{requestIDs: make(map[string]struct{})}
}

func (t *cancelTracker) mark(requestID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestIDs[requestID] = struct{}{}
}

func (t *cancelTracker) take(requestID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.requestIDs[requestID]; !ok {
		return false
	}
	delete(t.requestIDs, requestID)
	return true
}

func (t *cancelTracker) marked(requestID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.requestIDs[requestID]
	return ok
}
