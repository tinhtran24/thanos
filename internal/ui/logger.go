package ui

import (
	"io"
	"strings"
)

func Success(output io.Writer, message string) {
	writeLine(output, SuccessStyle.Render("✓")+" "+message)
}

func Error(output io.Writer, message string) {
	writeLine(output, ErrorStyle.Render("✗")+" "+message)
}

func Warning(output io.Writer, message string) {
	writeLine(output, WarningStyle.Render("⚠")+" "+message)
}

func Info(output io.Writer, message string) {
	writeLine(output, InfoStyle.Render("ℹ")+" "+message)
}

func Debug(output io.Writer, message string) {
	writeLine(output, MutedStyle.Render("• "+message))
}

func Raw(output io.Writer, value string) {
	_, _ = io.WriteString(output, value)
}

func Line(output io.Writer, value string) {
	writeLine(output, value)
}

func Block(output io.Writer, value string) {
	if value == "" {
		return
	}
	Raw(output, strings.TrimRight(value, "\n")+"\n")
}

func writeLine(output io.Writer, value string) {
	_, _ = io.WriteString(output, value+"\n")
}
