package agentrun

import "sync"

// userStops 记录哪些任务由用户主动停止，用于输出 run.canceled。
type userStops struct {
	mu         sync.Mutex
	requestIDs map[string]struct{}
}

func (stops *userStops) mark(requestID string) {
	stops.mu.Lock()
	defer stops.mu.Unlock()
	stops.requestIDs[requestID] = struct{}{}
}

func (stops *userStops) contains(requestID string) bool {
	stops.mu.Lock()
	defer stops.mu.Unlock()
	_, exists := stops.requestIDs[requestID]
	return exists
}

func (stops *userStops) take(requestID string) bool {
	stops.mu.Lock()
	defer stops.mu.Unlock()
	_, exists := stops.requestIDs[requestID]
	delete(stops.requestIDs, requestID)
	return exists
}
