package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

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
}

type Profile struct {
	Name    string
	Content string
}

func Render(role model.Role, data Data) (string, error) {
	data.Locale = data.Config.Locale
	data.LocaleName = localeName(data.Locale)
	data.Skills = skillsForRole(data.Config.Skills, role)
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
