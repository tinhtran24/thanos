package util

import (
	"io"
	"os"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// CopiedMsg is emitted after an attempted clipboard copy.
type CopiedMsg struct {
	Lines int
	Err   error
}

// Copy returns a command that writes text to the system clipboard using an
// OSC52 escape sequence on out (the program's output writer). OSC52 works across
// the alt-screen and over SSH without shelling out to pbcopy/xclip. When running
// inside tmux it wraps the sequence for passthrough.
func Copy(out io.Writer, text string) tea.Cmd {
	return func() tea.Msg {
		if out == nil {
			out = os.Stdout
		}
		seq := osc52.New(text)
		if strings.TrimSpace(os.Getenv("TMUX")) != "" {
			seq = seq.Tmux()
		}
		_, err := seq.WriteTo(out)
		return CopiedMsg{Lines: strings.Count(text, "\n") + 1, Err: err}
	}
}
