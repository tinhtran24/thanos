package state

import (
	"fmt"
	"time"

	"github.com/tinhtran/thanos/internal/model"
)

var transitions = map[model.Phase][]model.Phase{
	model.PhaseInit:     {model.PhasePlan, model.PhaseCode},
	model.PhasePlan:     {model.PhaseCode},
	model.PhaseCode:     {model.PhaseTest},
	model.PhaseAmend:    {model.PhaseTest},
	model.PhaseTest:     {model.PhaseCode, model.PhaseAmend, model.PhaseOverview},
	model.PhaseOverview: {model.PhasePending},
	model.PhasePending:  {model.PhaseDone},

	// Legacy transitions remain valid so sessions created by older versions can
	// be resumed and moved into the simplified workflow.
	model.PhaseDesign:       {model.PhaseDesignReview, model.PhaseCode},
	model.PhaseDesignReview: {model.PhaseCode, model.PhaseDesign},
	model.PhaseReview:       {model.PhaseCode, model.PhaseTest, model.PhaseAmend},
	model.PhaseDeepReview:   {model.PhaseOverview, model.PhaseAccept, model.PhaseCode, model.PhaseAmend},
	model.PhaseAccept:       {model.PhasePending},
	model.PhaseBlocked:      {model.PhasePlan, model.PhaseCode, model.PhaseTest, model.PhaseOverview},
	model.PhaseAttention:    {model.PhasePlan, model.PhaseCode},
}

func CanTransition(from, to model.Phase) bool {
	if from == model.PhaseDone {
		return false
	}
	if to == model.PhaseBlocked || to == model.PhaseAttention {
		return true
	}
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

func Transition(current model.State, to model.Phase) (model.State, error) {
	if !CanTransition(current.Phase, to) {
		return current, fmt.Errorf("invalid transition %s -> %s", current.Phase, to)
	}
	if to == model.PhaseAmend {
		current.Round++
		if current.MaxRounds > 0 && current.Round > current.MaxRounds {
			to = model.PhaseAttention
			current.Reason = "maximum amendment rounds reached"
		}
	}
	// The orchestrator resets Round before starting a new EC. Other entries into
	// Code (legacy migration and continue) preserve the current retry round.
	if to == model.PhaseCode && current.Round == 0 {
		current.Round = 1
	}
	current.Phase = to
	if to != model.PhaseBlocked && to != model.PhaseAttention {
		current.Role = RoleForPhase(to)
	}
	current.Active = to != model.PhaseDone && to != model.PhasePending
	current.UpdatedAt = time.Now().UTC()
	return current, nil
}

func ResumeFailedRound(current model.State, round int) (model.State, error) {
	if current.Phase != model.PhaseAttention {
		return current, fmt.Errorf("cannot continue %s from %s", current.FeatureID, current.Phase)
	}
	if round < 1 || round > current.Round {
		return current, fmt.Errorf("invalid failed round %d for current round %d", round, current.Round)
	}
	current.Phase = model.PhaseAmend
	current.Role = model.RoleCoder
	current.Round = round
	current.Active = true
	current.Reason = ""
	current.UpdatedAt = time.Now().UTC()
	return current, nil
}

func RoleForPhase(phase model.Phase) model.Role {
	switch phase {
	case model.PhasePlan:
		return model.RolePlanner
	case model.PhaseDesign:
		return model.RoleDesigner
	case model.PhaseDesignReview:
		return model.RoleDesignReviewer
	case model.PhaseCode, model.PhaseAmend:
		return model.RoleCoder
	case model.PhaseReview:
		return model.RoleReviewer
	case model.PhaseTest:
		return model.RoleTester
	case model.PhaseOverview:
		return model.RoleAcceptor
	case model.PhaseDeepReview:
		return model.RoleDeepReviewer
	case model.PhaseAccept:
		return model.RoleAcceptor
	default:
		return ""
	}
}
