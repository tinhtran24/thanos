// Package attachments handles files and images attached to a run: persistence
// under .thanos/<feature>/attachments and the chip strip shown above the input.
// It mirrors crush's internal/ui/attachments.
package attachments

import (
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
// relative to the runtime dir). Wiring this into prompts/runner is a follow-up.
func Reference(a Attachment) string { return a.RelPath }

func sanitize(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "_")
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	return name
}
