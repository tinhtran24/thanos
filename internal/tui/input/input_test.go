package input

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestParseCommands(t *testing.T) {
	cases := []struct {
		in      string
		wantCmd string
		wantArg string
	}{
		{"", "/run", ""},
		{"/run", "/run", ""},
		{"/new add login", "/new", "add login"},
		{"/runner claude", "/runner", "claude"},
		{"  /approve  ", "/approve", ""},
		{"just some text", "/run", "just some text"},
	}
	for _, c := range cases {
		cmd, arg := Parse(c.in)
		if cmd != c.wantCmd || arg != c.wantArg {
			t.Errorf("Parse(%q) = (%q,%q), want (%q,%q)", c.in, cmd, arg, c.wantCmd, c.wantArg)
		}
	}
}

func TestParseMultiline(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantCmd string
		wantArg string
	}{
		{
			name:    "implicit run preserves lines",
			value:   "first line\nsecond line",
			wantCmd: "/run",
			wantArg: "first line\nsecond line",
		},
		{
			name:    "slash command splits at newline",
			value:   "/new\nfirst line\nsecond line",
			wantCmd: "/new",
			wantArg: "first line\nsecond line",
		},
		{
			name:    "slash command splits at unicode whitespace",
			value:   "/new\u2003first line\nsecond line",
			wantCmd: "/new",
			wantArg: "first line\nsecond line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, arg := Parse(tt.value)
			if cmd != tt.wantCmd || arg != tt.wantArg {
				t.Fatalf("Parse(%q) = (%q, %q), want (%q, %q)", tt.value, cmd, arg, tt.wantCmd, tt.wantArg)
			}
		})
	}
}

func TestPasteSingleLineInsertsAtCursor(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("prefixsuffix")
	_ = m.Focus()
	for range len("suffix") {
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	}
	m.Update(tea.PasteMsg{Content: " middle "})
	if got, want := m.Value(), "prefix middle suffix"; got != want {
		t.Fatalf("Value() = %q, want %q", got, want)
	}
}

func TestPasteTabIsSanitizedAndAccepted(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("prefixsuffix")
	_ = m.Focus()
	for range len("suffix") {
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	}
	m.Update(tea.PasteMsg{Content: "one\ttwo"})
	if err := m.Err(); err != nil {
		t.Fatalf("tab-containing paste rejected: %v", err)
	}
	if got, want := m.Value(), "prefixone    twosuffix"; got != want {
		t.Fatalf("Value() = %q, want %q", got, want)
	}
}

func TestPasteMultilineNormalizesLineEndings(t *testing.T) {
	for _, content := range []string{"one\ntwo\nthree", "one\r\ntwo\r\nthree", "one\rtwo\rthree"} {
		m := New()
		m.SetWidth(40)
		_ = m.Focus()
		m.Update(tea.PasteMsg{Content: content})
		if got, want := m.Value(), "one\ntwo\nthree"; got != want {
			t.Fatalf("paste %q produced %q, want %q", content, got, want)
		}
	}
}

func TestSetValueAndResetNormalizeMultiline(t *testing.T) {
	m := New()
	m.SetValue("one\r\ntwo\rthree")
	if got, want := m.Value(), "one\ntwo\nthree"; got != want {
		t.Fatalf("Value() = %q, want %q", got, want)
	}
	m.Reset()
	if got := m.Value(); got != "" {
		t.Fatalf("Value() after Reset = %q, want empty", got)
	}
}

func TestContentLongerThanViewportIsNotTruncated(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetMaxHeight(6)
	_ = m.Focus()
	want := strings.Join([]string{"1", "2", "3", "4", "5", "6", "7", "8"}, "\n")
	m.Update(tea.PasteMsg{Content: want})
	if got := m.Value(); got != want {
		t.Fatalf("long value was truncated: got %q", got)
	}
	if got := m.Height(); got != 6 {
		t.Fatalf("Height() = %d, want viewport cap 6", got)
	}
}

func TestPasteContentLimitRejectedAtomically(t *testing.T) {
	m := New()
	m.SetWidth(40)
	m.SetValue("keep")
	_ = m.Focus()
	tooLong := strings.Repeat("x\n", maxContentHeight)
	m.Update(tea.PasteMsg{Content: tooLong})
	if m.Err() == nil {
		t.Fatal("expected paste limit error")
	}
	if got := m.Value(); got != "keep" {
		t.Fatalf("rejected paste mutated value to %q", got)
	}
}

func TestLongPasteVisualContentLimitRejectedAtomically(t *testing.T) {
	m := New()
	m.SetWidth(12)
	m.SetValue("keep")
	_ = m.Focus()
	m.Update(tea.PasteMsg{Content: strings.Repeat("x", maxContentHeight*8)})
	if m.Err() == nil {
		t.Fatal("expected soft-wrapped paste limit error")
	}
	if got := m.Value(); got != "keep" {
		t.Fatalf("rejected soft-wrapped paste mutated value to %q", got)
	}
}

func TestPasteAtContentLimitIsRetained(t *testing.T) {
	m := New()
	m.SetWidth(40)
	_ = m.Focus()
	want := strings.Repeat("x\n", maxContentHeight-1) + "x"
	m.Update(tea.PasteMsg{Content: want})
	if err := m.Err(); err != nil {
		t.Fatalf("paste at limit rejected: %v", err)
	}
	if got := m.Value(); got != want {
		t.Fatalf("paste at limit retained %d bytes, want %d", len(got), len(want))
	}
}

func TestKeyboardEditingStopsAtContentLimit(t *testing.T) {
	m := New()
	m.SetWidth(40)
	want := strings.Repeat("x\n", maxContentHeight-1) + "x"
	m.SetValue(want)
	_ = m.Focus()

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := m.Value(); got != want {
		t.Fatalf("keyboard edit exceeded content limit: got %d bytes, want %d", len(got), len(want))
	}
}
