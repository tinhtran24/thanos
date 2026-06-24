package input

import "testing"

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
