package chat

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// SidebarData carries everything the capabilities sidebar needs to render.
type SidebarData struct {
	Config  model.Config
	Runner  string
	Feature model.Feature
	DotDir  string
}

// RenderSidebar draws the right "Capabilities" pane: active runner, code
// intelligence, extensions, and per-feature impact memory.
func RenderSidebar(d SidebarData, width int) string {
	var b strings.Builder
	b.WriteString(styles.SectionTitle("Active model runner"))
	b.WriteString("\n")
	b.WriteString(styles.AccentS.Bold(true).Render(d.Runner))
	if configured, ok := d.Config.Runners[d.Runner]; ok {
		b.WriteString("\n")
		b.WriteString(styles.MutedS.Render(util.Truncate(configured.Command+" "+strings.Join(configured.Args, " "), width)))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.SectionTitle("Code intelligence"))
	graphPath := filepath.Join(d.DotDir, "codebase", "graph.json")
	if _, err := exec.LookPath("git"); err == nil {
		b.WriteString(styles.StatusLine(true, "Repository context"))
	}
	b.WriteString(styles.StatusLine(fileExists(graphPath), "Local code graph"))
	for _, name := range sortedKeys(d.Config.LSP) {
		cfg := d.Config.LSP[name]
		ok := !cfg.Disabled
		if ok {
			_, err := exec.LookPath(cfg.Command)
			ok = err == nil
		}
		b.WriteString(styles.StatusLine(ok, "LSP · "+name))
	}
	if len(d.Config.LSP) == 0 {
		b.WriteString(styles.MutedS.Render("\nNo LSP configured"))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.SectionTitle("Extensions"))
	for _, name := range sortedMCP(d.Config.MCP) {
		cfg := d.Config.MCP[name]
		b.WriteString(styles.StatusLine(!cfg.Disabled, "MCP · "+name+" ["+cfg.Type+"]"))
	}
	if len(d.Config.MCP) == 0 {
		b.WriteString(styles.MutedS.Render("\nNo MCP configured"))
	}
	for _, skill := range d.Config.Skills {
		b.WriteString(styles.StatusLine(true, "Skill · "+skill.Name))
	}

	if d.Feature.ID != "" {
		if memory, err := featuregraph.Resolve(d.DotDir, d.Feature.ID); err == nil &&
			(len(memory.Rules) > 0 || len(memory.Decisions) > 0 || len(memory.Paths) > 0) {
			b.WriteString("\n\n")
			b.WriteString(styles.SectionTitle("Impact memory"))
			b.WriteString("\n")
			b.WriteString(styles.MutedS.Render(
				itoa(len(memory.Rules)) + " rules · " + itoa(len(memory.Decisions)) +
					" decisions · " + itoa(len(memory.Paths)) + " paths"))
			for index, path := range memory.Paths {
				if index == 4 {
					b.WriteString("\n" + styles.MutedS.Render("…and "+itoa(len(memory.Paths)-index)+" more"))
					break
				}
				b.WriteString("\n" + styles.MutedS.Render("• "+util.Truncate(path.Path, util.Max(8, width-2))))
			}
		}
	}
	return b.String()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sortedKeys(items map[string]model.LSP) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedMCP(items map[string]model.MCP) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
