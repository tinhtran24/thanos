// Package attachments handles files and images attached to a run: persistence
// under .thanos/<feature>/attachments and the chip strip shown above the input.
// It mirrors crush's internal/ui/attachments.
package attachments

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/tinhtran/thanos/internal/workspace"
)

// Attachment is a stored file reference.
type Attachment struct {
	Name    string // display name (sanitized base name)
	RelPath string // path relative to the feature runtime dir
	AbsPath string // absolute path on disk
	Size    int64
	MIME    string
	IsImage bool
}

// dir returns (and creates) the attachments dir for a feature.
func dir(ws *workspace.Workspace, featureID string) (string, error) {
	d := filepath.Join(ws.RuntimeDir(featureID), "attachments")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return d, nil
}

// SaveFile copies an existing file on disk into the feature's attachments dir.
func SaveFile(ws *workspace.Workspace, featureID, srcPath string) (Attachment, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return Attachment{}, err
	}
	return SaveBytes(ws, featureID, filepath.Base(srcPath), data)
}

// SaveBytes writes raw bytes (e.g. a pasted image) as a named attachment.
func SaveBytes(ws *workspace.Workspace, featureID, name string, data []byte) (Attachment, error) {
	d, err := dir(ws, featureID)
	if err != nil {
		return Attachment{}, err
	}
	name = sanitize(name)
	abs := filepath.Join(d, name)
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return Attachment{}, err
	}
	mt := mime.TypeByExtension(filepath.Ext(name))
	return Attachment{
		Name:    name,
		RelPath: filepath.Join("attachments", name),
		AbsPath: abs,
		Size:    int64(len(data)),
		MIME:    mt,
		IsImage: strings.HasPrefix(mt, "image/"),
	}, nil
}

// Reference returns the agent-facing reference for an attachment (its path
// relative to the runtime dir).
func Reference(a Attachment) string { return a.RelPath }

// WriteManifest records staged attachments and @-file references into
// .thanos/<feature>/context/attachments.md, which prompts.Render references so
// the agent reads them. Small text files are inlined; large/binary ones are
// listed by path. Returns false when there is nothing to write.
func WriteManifest(ws *workspace.Workspace, featureID string, items []Attachment, refs []string) (bool, error) {
	if len(items) == 0 && len(refs) == 0 {
		return false, nil
	}
	dir := filepath.Join(ws.RuntimeDir(featureID), "context")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	var b strings.Builder
	b.WriteString("# Attached context\n\nThe user attached these files/notes for this task.\n")
	for _, a := range items {
		fmt.Fprintf(&b, "\n## %s (`%s`)\n", a.Name, a.RelPath)
		if !a.IsImage && a.Size > 0 && a.Size <= 64*1024 {
			if data, err := os.ReadFile(a.AbsPath); err == nil {
				b.WriteString("```\n")
				b.Write(data)
				b.WriteString("\n```\n")
				continue
			}
		}
		fmt.Fprintf(&b, "(see file at `.thanos/%s/%s`)\n", featureID, a.RelPath)
	}
	for _, ref := range refs {
		fmt.Fprintf(&b, "\n## @%s\n", ref)
		abs := filepath.Join(ws.Root, ref)
		if info, err := os.Stat(abs); err == nil && !info.IsDir() && info.Size() <= 64*1024 {
			if data, err := os.ReadFile(abs); err == nil {
				b.WriteString("```\n")
				b.Write(data)
				b.WriteString("\n```\n")
				continue
			}
		}
		fmt.Fprintf(&b, "(workspace file `%s`)\n", ref)
	}
	return true, os.WriteFile(filepath.Join(dir, "attachments.md"), []byte(b.String()), 0o644)
}

// ClearManifest removes the context manifest for a feature.
func ClearManifest(ws *workspace.Workspace, featureID string) {
	_ = os.Remove(filepath.Join(ws.RuntimeDir(featureID), "context", "attachments.md"))
}

func sanitize(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "_")
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	return name
}
