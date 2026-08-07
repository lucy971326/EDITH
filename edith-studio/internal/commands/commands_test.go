package commands

import (
	"errors"
	"testing"
)

func TestListUsesSyntaxInsteadOfUsage(t *testing.T) {
	definitions := (&Module{}).List()
	if len(definitions) != 1 || definitions[0].Syntax != "/compact" {
		t.Fatalf("definitions = %#v", definitions)
	}
}

func TestParseCompact(t *testing.T) {
	for _, input := range []string{"/compact", " /COMPACT "} {
		got, err := parse(input)
		if err != nil || got != "compact" {
			t.Fatalf("parse(%q) = %q, %v", input, got, err)
		}
	}
}

func TestParseRejectsUnknownAndArguments(t *testing.T) {
	if _, err := parse("/help"); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("unknown command error = %v", err)
	}
	if _, err := parse("/compact now"); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("argument error = %v", err)
	}
	if _, err := parse("hello"); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("plain text error = %v", err)
	}
}

func TestExecuteRequiresEngine(t *testing.T) {
	_, err := New(Dependencies{})
	if err == nil {
		t.Fatal("New accepted missing engine")
	}
}
