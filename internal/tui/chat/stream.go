package chat

import (
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// OnEvent advances the chat log in response to an orchestrator lifecycle event
// (read from events.jsonl). role-start opens a new speaker bubble; transition
// inserts a divider; role-end closes the open bubble and surfaces any error.
func (m *Model) OnEvent(ev model.Event) {
	switch ev.Type {
	case "role-start":
		// Finish whatever was open, then open a fresh role bubble.
		if i := m.openIndex(); i >= 0 {
			m.messages[i].Done = true
		}
		workItem := ""
		if ev.Data != nil {
			if value, ok := ev.Data["work_item"].(string); ok {
				workItem = value
			}
		}
		m.messages = append(m.messages, Message{
			ID:      m.id(),
			Kind:    KindRole,
			Role:    ev.Role,
			Phase:   ev.Phase,
			Context: workItem,
			TS:      ev.Timestamp,
		})
		m.rerender()
	case "transition":
		to := ""
		if ev.Data != nil {
			if v, ok := ev.Data["to"].(string); ok {
				to = v
			}
		}
		label := styles.PhaseLabel(model.Phase(to))
		m.add(Message{Kind: KindSystem, Body: "→ " + label})
	case "role-end":
		if i := m.openIndex(); i >= 0 {
			m.messages[i].Done = true
		}
		if ev.Data != nil {
			if success, ok := ev.Data["success"].(bool); ok && !success {
				if errText, ok := ev.Data["error"].(string); ok && errText != "" {
					m.AddError(util.Truncate(errText, 400))
				}
			}
		}
		m.rerender()
	}
}
