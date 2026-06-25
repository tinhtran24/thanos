package state

import (
	"testing"

	"github.com/tinhtran/thanos/internal/model"
)

func TestTransitionFlow(t *testing.T) {
	s := model.State{Phase: model.PhaseInit}
	for _, phase := range []model.Phase{
		model.PhasePlan,
		model.PhaseCode,
		model.PhaseReview,
		model.PhaseTest,
		model.PhaseOverview,
		model.PhaseDone,
	} {
		var err error
		s, err = Transition(s, phase)
		if err != nil {
			t.Fatalf("transition to %s: %v", phase, err)
		}
	}
	if s.Active || s.Phase != model.PhaseDone {
		t.Fatalf("unexpected final state: %+v", s)
	}
}

func TestFailedReviewAndTestingReopenDevelopment(t *testing.T) {
	for _, phase := range []model.Phase{model.PhaseReview, model.PhaseTest} {
		got, err := Transition(model.State{Phase: phase}, model.PhaseCode)
		if err != nil {
			t.Fatalf("%s -> coding: %v", phase, err)
		}
		if got.Phase != model.PhaseCode || got.Role != model.RoleCoder || !got.Active {
			t.Fatalf("reopened state = %+v", got)
		}
	}
}

func TestInvalidTransition(t *testing.T) {
	_, err := Transition(model.State{Phase: model.PhaseInit}, model.PhaseTest)
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestRoleForWorkflowPhases(t *testing.T) {
	tests := map[model.Phase]model.Role{
		model.PhasePlan:     model.RolePlanner,
		model.PhaseCode:     model.RoleCoder,
		model.PhaseReview:   model.RoleReviewer,
		model.PhaseTest:     model.RoleTester,
		model.PhaseOverview: model.RoleAcceptor,
	}
	for phase, want := range tests {
		if got := RoleForPhase(phase); got != want {
			t.Fatalf("RoleForPhase(%s) = %s, want %s", phase, got, want)
		}
	}
}

func TestCompleteReadyForReviewState(t *testing.T) {
	got, err := Complete(model.State{
		FeatureID: "F001-test",
		Phase:     model.PhaseReview,
		Role:      model.RoleReviewer,
		Active:    true,
		Reason:    "old error",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != model.PhaseDone || got.Role != "" || got.Active || got.Reason != "" {
		t.Fatalf("completed state = %+v", got)
	}
}

func TestCompleteRejectsPlanning(t *testing.T) {
	if _, err := Complete(model.State{FeatureID: "F001-test", Phase: model.PhasePlan}); err == nil {
		t.Fatal("expected completion error")
	}
}
