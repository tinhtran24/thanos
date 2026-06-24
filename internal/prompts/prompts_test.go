package prompts

import (
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/model"
)

func TestAllTemplatesRender(t *testing.T) {
	roles := []model.Role{
		model.RoleDesigner,
		model.RoleDesignReviewer,
		model.RoleCoder,
		model.RoleReviewer,
		model.RoleTester,
		model.RoleDeepReviewer,
		model.RoleAcceptor,
		model.RoleMiniCoder,
		model.RoleReVerifier,
		model.RoleSynthesizer,
		model.RoleGate,
	}
	data := Data{
		Feature: model.Feature{ID: "F001-test", Title: "Test"},
		Config: model.Config{
			Project: model.Project{Name: "test"},
			Skills:  []model.Skill{{Name: "testing", Path: ".agents/skills/testing/SKILL.md"}},
		},
		State:         model.State{Round: 1},
		Root:          "/tmp/test",
		Iteration:     1,
		ReviewerCount: 3,
	}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			output, err := Render(role, data)
			if err != nil {
				t.Fatal(err)
			}
			if output == "" || !strings.Contains(output, "Configured Skills") {
				t.Fatalf("incomplete prompt: %q", output)
			}
			if !strings.Contains(output, "Codebase Graph") {
				t.Fatalf("codebase graph instructions missing: %q", output)
			}
			if !strings.Contains(output, "Persistent Feature Memory") {
				t.Fatalf("feature memory instructions missing: %q", output)
			}
		})
	}
}

func TestSkillRoleFiltering(t *testing.T) {
	data := Data{
		Feature: model.Feature{ID: "F001", Title: "Test"},
		Config: model.Config{Skills: []model.Skill{
			{Name: "coder-only", Path: "coder.md", Roles: []string{"coder"}},
			{Name: "global", Path: "global.md"},
		}},
		State: model.State{Round: 1},
	}
	output, err := Render(model.RoleTester, data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "coder-only") || !strings.Contains(output, "global") {
		t.Fatalf("unexpected skills in prompt:\n%s", output)
	}
}

func TestLocaleInstruction(t *testing.T) {
	output, err := Render(model.RoleDesigner, Data{
		Feature: model.Feature{ID: "F001", Title: "Test"},
		Config:  model.Config{Locale: "vi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Vietnamese") {
		t.Fatalf("locale instruction missing:\n%s", output)
	}
}
