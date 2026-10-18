package auth

import (
	"errors"

	"github.com/alexagc/kalyx/internal/core"
)

var (
	ErrForbiddenPropose = errors.New("user is not authorized to propose contracts for this team")
	ErrForbiddenApprove = errors.New("user is not authorized to approve this change proposal")
)

// CanPropose verifies if an authenticated user is permitted to create an event
// definition or submit a contract change proposal.
// Rule: True if the user is a member of the event's owner_team, or has 'lead' or 'data_steward' role.
func CanPropose(user *UserContext, event *core.EventDefinition) bool {
	if user == nil || event == nil {
		return false
	}

	// Data stewards and team leads can propose changes across any team
	if user.HasRole(RoleDataSteward) || user.HasRole(RoleLead) {
		return true
	}

	// Event without specified owner team can be proposed by any authenticated user
	if event.OwnerTeam == "" {
		return true
	}

	// Members must belong to the event's owner_team
	return user.IsInTeam(event.OwnerTeam)
}

// CanApprove verifies if an authenticated user is permitted to approve a contract proposal.
// Rules:
// - If isBreaking is true: the user MUST possess the 'data_steward' role.
// - If isBreaking is false: the user must be a 'lead' of the event's owner_team, or a 'data_steward'.
func CanApprove(user *UserContext, event *core.EventDefinition, isBreaking bool) bool {
	if user == nil || event == nil {
		return false
	}

	// Breaking changes strictly mandate Data Steward approval
	if isBreaking {
		return user.HasRole(RoleDataSteward)
	}

	// Data stewards can approve non-breaking changes across all teams
	if user.HasRole(RoleDataSteward) {
		return true
	}

	// Team leads can approve non-breaking changes for their own team
	if user.HasRole(RoleLead) {
		if event.OwnerTeam == "" || user.IsInTeam(event.OwnerTeam) {
			return true
		}
	}

	return false
}

// CanDeprecate checks if a user is permitted to transition an event version to StatusDeprecated.
// Rule: Lead of the event's owner team, or data steward.
func CanDeprecate(user *UserContext, event *core.EventDefinition) bool {
	if user == nil || event == nil {
		return false
	}

	if user.HasRole(RoleDataSteward) {
		return true
	}

	if user.HasRole(RoleLead) {
		return event.OwnerTeam == "" || user.IsInTeam(event.OwnerTeam)
	}

	return false
}
