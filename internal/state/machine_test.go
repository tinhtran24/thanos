package state

import (
	"testing"

	"github.com/tinhtran/thanos/internal/model"
)

func TestTransitionFlow(t *testing.T) {
	s := model.State{Phase: model.PhaseInit, MaxRounds: 2}
	for _, phase := range []model.Phase{
		model.PhaseDesign,
		model.PhaseDesignReview,
		model.PhaseCode,
		model.PhaseReview,
		model.PhaseTest,
		model.PhaseDeepReview,
		model.PhaseAccept,
		model.PhasePending,
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

func TestInvalidTransition(t *testing.T) {
	_, err := Transition(model.State{Phase: model.PhaseInit}, model.PhaseTest)
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestRoundLimitEscalates(t *testing.T) {
	s := model.State{Phase: model.PhaseReview, Round: 2, MaxRounds: 2}
	got, err := Transition(s, model.PhaseAmend)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != model.PhaseAttention {
		t.Fatalf("phase = %s, want %s", got.Phase, model.PhaseAttention)
	}
}
