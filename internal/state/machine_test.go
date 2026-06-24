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

func TestResumeFailedRound(t *testing.T) {
	current := model.State{
		FeatureID: "F002-test",
		Phase:     model.PhaseAttention,
		Role:      "",
		Round:     5,
		MaxRounds: 10,
		Reason:    "maximum amendment rounds reached",
	}
	got, err := ResumeFailedRound(current, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != model.PhaseAmend || got.Role != model.RoleCoder || got.Round != 3 {
		t.Fatalf("resumed state = %+v", got)
	}
	if !got.Active || got.Reason != "" || got.MaxRounds != 10 {
		t.Fatalf("resumed metadata = %+v", got)
	}
}

func TestResumeFailedRoundRequiresAttention(t *testing.T) {
	_, err := ResumeFailedRound(model.State{
		FeatureID: "F002-test",
		Phase:     model.PhaseReview,
		Round:     3,
	}, 3)
	if err == nil {
		t.Fatal("expected recovery error")
	}
}

func TestPerECTransitions(t *testing.T) {
	// Init → Plan → Design begins the first chunk.
	s := model.State{Phase: model.PhaseInit, MaxRounds: 3}
	var err error
	if s, err = Transition(s, model.PhasePlan); err != nil {
		t.Fatal(err)
	}
	if s.Role != model.RolePlanner {
		t.Fatalf("plan role = %q, want planner", s.Role)
	}
	if s, err = Transition(s, model.PhaseDesign); err != nil {
		t.Fatal(err)
	}
	// Walk EC-1 to a deep-review pass, accumulating a round via amend.
	for _, p := range []model.Phase{model.PhaseDesignReview, model.PhaseCode, model.PhaseReview, model.PhaseAmend, model.PhaseReview, model.PhaseTest, model.PhaseDeepReview} {
		if s, err = Transition(s, p); err != nil {
			t.Fatalf("EC1 -> %s: %v", p, err)
		}
	}
	if s.Round != 2 {
		t.Fatalf("EC1 round = %d, want 2 (one amend)", s.Round)
	}
	// DeepReview pass on a non-last chunk advances to Design (next EC) and resets round.
	if s, err = Transition(s, model.PhaseDesign); err != nil {
		t.Fatalf("advance to next EC: %v", err)
	}
	if s.Round != 0 {
		t.Fatalf("round after starting EC2 = %d, want 0", s.Round)
	}
	// Walk EC-2 to deep-review, then finish via Accept (last chunk).
	for _, p := range []model.Phase{model.PhaseDesignReview, model.PhaseCode, model.PhaseReview, model.PhaseTest, model.PhaseDeepReview, model.PhaseAccept, model.PhasePending, model.PhaseDone} {
		if s, err = Transition(s, p); err != nil {
			t.Fatalf("EC2 -> %s: %v", p, err)
		}
	}
	if s.Phase != model.PhaseDone || s.Active {
		t.Fatalf("final state = %+v", s)
	}
}
