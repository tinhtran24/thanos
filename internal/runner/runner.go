package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tinhtran/thanos/internal/model"
)

type Runner interface {
	Run(context.Context, string, model.Runner, string, io.Writer, io.Writer) error
}

type Subprocess struct{}

func (Subprocess) Run(ctx context.Context, root string, config model.Runner, prompt string, stdout, stderr io.Writer) error {
	if config.Command == "" {
		return fmt.Errorf("runner command is empty")
	}
	args := append([]string{}, config.Args...)
	cmd := exec.CommandContext(ctx, config.Command, args...)
	cmd.Dir = root
	cmd.Stdin = stringReader(prompt)
	cmd.Stdout = io.MultiWriter(stdout, promptLog(root, "runner.stdout.log"))
	cmd.Stderr = io.MultiWriter(stderr, promptLog(root, "runner.stderr.log"))
	cmd.Env = append(os.Environ(), "THANOS_PROJECT_ROOT="+root)
	return cmd.Run()
}

func promptLog(root, name string) io.Writer {
	path := filepath.Join(root, ".thanos", name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard
	}
	return file
}

type reader struct {
	value string
	index int
}

func stringReader(value string) *reader { return &reader{value: value} }

func (r *reader) Read(target []byte) (int, error) {
	if r.index >= len(r.value) {
		return 0, io.EOF
	}
	n := copy(target, r.value[r.index:])
	r.index += n
	return n, nil
}
