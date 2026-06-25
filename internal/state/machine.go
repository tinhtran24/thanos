package state

import (
	"fmt"
	"time"

	"github.com/tinhtran/thanos/internal/model"
)

var transitions = map[model.Phase][]model.Phase{
	model.PhaseInit:     {model.PhasePlan, model.PhaseCode},
	model.PhasePlan:     {model.PhaseCode},
	model.PhaseCode:     {model.PhaseReview},
	model.PhaseReview:   {model.PhaseCode, model.PhaseTest},
	model.PhaseTest:     {model.PhaseCode, model.PhaseOverview},
	model.PhaseOverview: {model.PhaseDone, model.PhasePending},
	model.PhasePending:  {model.PhaseDone},

	// Legacy transitions remain valid so sessions created by older versions can
	// be resumed and moved into the simplified workflow.
	model.PhaseDesign:       {model.PhaseDesignReview, model.PhaseCode},
	model.PhaseDesignReview: {model.PhaseCode, model.PhaseDesign},
	model.PhaseAmend:        {model.PhaseReview, model.PhaseTest},
	model.PhaseDeepReview:   {model.PhaseOverview, model.PhaseAccept, model.PhaseCode},
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
	current.Phase = to
	if to != model.PhaseBlocked && to != model.PhaseAttention {
		current.Role = RoleForPhase(to)
	}
	current.Active = to != model.PhaseDone && to != model.PhasePending
	current.UpdatedAt = time.Now().UTC()
	return current, nil
}

// Complete applies the final workflow state after external evidence validation.
// It is used by the manual recovery command for ready-for-review or legacy
// pending-review tickets.
func Complete(current model.State) (model.State, error) {
	switch current.Phase {
	case model.PhaseReview, model.PhaseTest, model.PhaseOverview, model.PhasePending, model.PhaseDone:
	default:
		return current, fmt.Errorf("cannot complete %s from %s", current.FeatureID, current.Phase)
	}
	current.Phase = model.PhaseDone
	current.Role = ""
	current.Active = false
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
