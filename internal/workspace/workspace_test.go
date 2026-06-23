package workspace

import (
	"testing"

	"github.com/tinhtran/thanos/internal/model"
)

func TestFeatureRoundTripAndShortID(t *testing.T) {
	ws := Open(t.TempDir())
	if err := ws.Init(model.Config{Project: model.Project{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-user-auth", Title: "User auth", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	got, err := ws.LoadFeature("F001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != feature.Title {
		t.Fatalf("title = %q", got.Title)
	}
}
