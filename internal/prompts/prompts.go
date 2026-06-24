package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
)

//go:embed *.md.tmpl locale.tmpl profiles/*.md
var files embed.FS

type Data struct {
	Feature       model.Feature
	Config        model.Config
	State         model.State
	Root          string
	Skills        []model.Skill
	Profiles      []Profile
	Iteration     int
	ReviewerIndex int
	ReviewerCount int
	Locale        string
	LocaleName    string
	CodebaseGraph string
	FeatureMemory string

	// Execution-chunk context. ExecutionChunk is the chunk the current role works
	// on (nil during feature-level planning/accept). Plan is the full chunk list.
	// ECPrefix is the artifact path prefix for the current chunk ("" for a single
	// implicit chunk, "ec-<i>/" for a multi-chunk plan). CodingStyle is the
	// optional project coding-style doc.
	ExecutionChunk *model.ExecutionChunk
	Plan           []model.ExecutionChunk
	ECPrefix       string
	CodingStyle    string
}

type Profile struct {
	Name    string
	Content string
}

func Render(role model.Role, data Data) (string, error) {
	data.Locale = data.Config.Locale
	data.LocaleName = localeName(data.Locale)
	data.Skills = skillsForRole(data.Config.Skills, role)
	data.CodebaseGraph = filepath.ToSlash(filepath.Join(".thanos", "codebase", "summary.md"))
	data.FeatureMemory = featuregraph.ContextMarkdown(filepath.Join(data.Root, ".thanos"), data.Feature.ID)
	if role == model.RoleTester {
		profiles, err := loadProfiles()
		if err != nil {
			return "", err
		}
		data.Profiles = profiles
	}
	name := string(role) + ".md.tmpl"
	source, err := files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("load %s prompt: %w", role, err)
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"sub": func(a, b int) int { return a - b },
	}).Parse(string(source))
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	if localeSource, err := files.ReadFile("locale.tmpl"); err == nil {
		localeTemplate, parseErr := template.New("locale").Parse(string(localeSource))
		if parseErr != nil {
			return "", parseErr
		}
		if err := localeTemplate.Execute(&output, data); err != nil {
			return "", err
		}
	}
	if err := tmpl.Execute(&output, data); err != nil {
		return "", err
	}
	if len(data.Skills) > 0 {
		output.WriteString("\n== Configured Skills ==\n")
		output.WriteString("Read and follow these project skills before acting:\n")
		for _, skill := range data.Skills {
			fmt.Fprintf(&output, "- %s: %s\n", skill.Name, skill.Path)
		}
	}
	if len(data.Config.LSP) > 0 {
		output.WriteString("\n== Language Servers ==\n")
		output.WriteString("Use diagnostics, definitions, references, and symbol context from these configured language servers when your runner exposes LSP tools:\n")
		for name, server := range data.Config.LSP {
			if !server.Disabled {
				fmt.Fprintf(&output, "- %s: %s %s\n", name, server.Command, strings.Join(server.Args, " "))
			}
		}
	}
	if len(data.Config.MCP) > 0 {
		output.WriteString("\n== MCP Servers ==\n")
		output.WriteString("These project MCP capabilities are registered. Use matching runner-native MCP tools when available:\n")
		for name, server := range data.Config.MCP {
			if !server.Disabled {
				fmt.Fprintf(&output, "- %s: %s\n", name, server.Type)
			}
		}
	}
	output.WriteString("\n== Persistent Feature Memory ==\n")
	output.WriteString("Use this impact map before planning or changing code. Treat inherited business rules as invariants unless the task explicitly changes them.\n")
	output.WriteString(data.FeatureMemory)
	output.WriteString("\n")
	output.WriteString("\n== Codebase Graph ==\n")
	fmt.Fprintf(&output, "Read `%s` before exploring source files. It contains the local codebase map, hub symbols, relationships, and detected conventions. Use `.thanos/codebase/graph.json` for machine-readable edges.\n", data.CodebaseGraph)

	// User-attached context (files/@-refs). The TUI writes the manifest before a
	// run; reference it (EC-level overrides feature-level) when present.
	for _, rel := range []string{filepath.Join(data.ECPrefix, "context", "attachments.md"), filepath.Join("context", "attachments.md")} {
		abs := filepath.Join(data.Root, ".thanos", data.Feature.ID, rel)
		if _, err := os.Stat(abs); err == nil {
			output.WriteString("\n== Attached context ==\n")
			fmt.Fprintf(&output, "The user attached files and notes for this task. Read `.thanos/%s` and use them as primary context.\n", filepath.ToSlash(filepath.Join(data.Feature.ID, rel)))
			break
		}
	}
	return output.String(), nil
}

func localeName(locale string) string {
	switch strings.ToLower(locale) {
	case "vi":
		return "Vietnamese"
	case "zh-tw":
		return "Traditional Chinese"
	case "zh-cn":
		return "Simplified Chinese"
	case "ja":
		return "Japanese"
	case "ko":
		return "Korean"
	case "es":
		return "Spanish"
	case "", "en":
		return "English"
	default:
		return locale
	}
}

func skillsForRole(skills []model.Skill, role model.Role) []model.Skill {
	var result []model.Skill
	for _, skill := range skills {
		if len(skill.Roles) == 0 {
			result = append(result, skill)
			continue
		}
		for _, allowed := range skill.Roles {
			if allowed == string(role) {
				result = append(result, skill)
				break
			}
		}
	}
	return result
}

func loadProfiles() ([]Profile, error) {
	paths, err := files.ReadDir("profiles")
	if err != nil {
		return nil, err
	}
	profiles := make([]Profile, 0, len(paths))
	for _, entry := range paths {
		content, err := files.ReadFile(filepath.ToSlash(filepath.Join("profiles", entry.Name())))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, Profile{
			Name:    strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Content: string(content),
		})
	}
	return profiles, nil
}
