package events

import (
	"encoding/json"
	"sync"
	"time"
)

type Event struct {
	ID        string         `json:"id,omitempty"`
	ProjectID string         `json:"project_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	Event     string         `json:"event"`
	Stage     string         `json:"stage,omitempty"`
	Status    string         `json:"status,omitempty"`
	Artifact  string         `json:"artifact,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
	history     []Event
	historyCap  int
}

func NewHub(historyCap int) *Hub {
	if historyCap < 0 {
		historyCap = 0
	}
	return &Hub{
		subscribers: make(map[chan Event]struct{}),
		historyCap:  historyCap,
	}
}

func (h *Hub) Publish(event Event) {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	h.mu.Lock()
	if h.historyCap > 0 {
		h.history = append(h.history, event)
		if overflow := len(h.history) - h.historyCap; overflow > 0 {
			h.history = append([]Event(nil), h.history[overflow:]...)
		}
	}
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan Event, buffer)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subscribers[ch]; ok {
			delete(h.subscribers, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) History() []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]Event(nil), h.history...)
}

func Marshal(event Event) ([]byte, error) {
	return json.Marshal(event)
}
