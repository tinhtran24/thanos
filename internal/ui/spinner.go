package ui

import (
	"context"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

type operationDoneMsg struct {
	err error
}

type spinnerModel struct {
	spinner spinner.Model
	message string
	run     func() error
	err     error
	done    bool
}

func newSpinnerModel(message string, run func() error) spinnerModel {
	model := spinner.New()
	model.Spinner = spinner.MiniDot
	model.Style = PrimaryStyle
	return spinnerModel{spinner: model, message: message, run: run}
}

func (model spinnerModel) Init() tea.Cmd {
	return tea.Batch(model.spinner.Tick, func() tea.Msg {
		return operationDoneMsg{err: model.run()}
	})
}

func (model spinnerModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case operationDoneMsg:
		model.err = message.err
		model.done = true
		return model, tea.Quit
	default:
		var command tea.Cmd
		model.spinner, command = model.spinner.Update(message)
		return model, command
	}
}

func (model spinnerModel) View() string {
	if model.done {
		return ""
	}
	return model.spinner.View() + " " + model.message
}

func Spin(ctx context.Context, output io.Writer, message string, run func() error) error {
	if !isInteractive(output) {
		Info(output, message)
		err := run()
		if err != nil {
			Error(output, message+" failed")
			return err
		}
		Success(output, message+" completed")
		return nil
	}

	model := newSpinnerModel(message, run)
	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(nil),
		tea.WithOutput(output),
		tea.WithoutSignalHandler(),
	)
	result, err := program.Run()
	if err != nil {
		return err
	}
	final := result.(spinnerModel)
	if final.err != nil {
		Error(output, message+" failed")
		return final.err
	}
	Success(output, message+" completed")
	return nil
}

func Build(ctx context.Context, output io.Writer, run func() error) error {
	return Spin(ctx, output, "Building binaries...", run)
}

func Release(ctx context.Context, output io.Writer, run func() error) error {
	return Spin(ctx, output, "Generating release...", run)
}

func Deploy(ctx context.Context, output io.Writer, run func() error) error {
	return Spin(ctx, output, "Deploying...", run)
}

func Publish(ctx context.Context, output io.Writer, run func() error) error {
	return Spin(ctx, output, "Publishing artifacts...", run)
}

func Upload(ctx context.Context, output io.Writer, run func() error) error {
	return Spin(ctx, output, "Uploading artifacts...", run)
}

func isInteractive(output io.Writer) bool {
	if os.Getenv("CI") != "" || os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
