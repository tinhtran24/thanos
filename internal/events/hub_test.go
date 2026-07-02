package events

import "testing"

func TestHubPublishesToSubscribersAndKeepsHistory(t *testing.T) {
	hub := NewHub(2)
	events, unsubscribe := hub.Subscribe(1)
	defer unsubscribe()

	hub.Publish(Event{Event: "task.created", TaskID: "T-1"})

	got := <-events
	if got.Event != "task.created" || got.TaskID != "T-1" {
		t.Fatalf("unexpected event: %#v", got)
	}
	if len(hub.History()) != 1 {
		t.Fatalf("history length = %d, want 1", len(hub.History()))
	}
}
