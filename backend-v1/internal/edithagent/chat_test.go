package edithagent

import "testing"

func TestChatInfo(t *testing.T) {
	if got := Chat.Info().Name; got != "edith-chat" {
		t.Fatalf("Info().Name = %q, want edith-chat", got)
	}
}

func TestChatIncludesDefaultTools(t *testing.T) {
	for _, want := range []string{"get_current_time"} {
		found := false
		for _, item := range Chat.Tools() {
			if item.Declaration().Name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Chat.Tools() does not contain %q", want)
		}
	}
}
