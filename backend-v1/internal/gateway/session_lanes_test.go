package gateway

import "testing"

func TestSessionLanesOnlyReleasesCurrentOwner(t *testing.T) {
	lanes := newSessionLanes()
	if !lanes.tryAcquire("alice", "session-1", "request-1") {
		t.Fatal("first request did not acquire lane")
	}
	if lanes.tryAcquire("alice", "session-1", "request-2") {
		t.Fatal("second request acquired occupied lane")
	}
	lanes.release("alice", "session-1", "request-2")
	if lanes.tryAcquire("alice", "session-1", "request-3") {
		t.Fatal("non-owner release cleared lane")
	}
	lanes.release("alice", "session-1", "request-1")
	if !lanes.tryAcquire("alice", "session-1", "request-3") {
		t.Fatal("owner release did not clear lane")
	}
}
