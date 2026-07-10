package domain

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleOperator   Role = "operator"
	RoleTreasurer  Role = "treasurer"
	RoleRisk       Role = "risk"
	RoleApprover   Role = "approver"
	RoleAuditor    Role = "auditor"
	RoleWithdrawal Role = "withdrawal"
	RoleInternal   Role = "internal"
)

type RoleSet map[Role]bool

func NewRoleSet(roles ...Role) RoleSet {
	set := make(RoleSet, len(roles))
	for _, role := range roles {
		if role != "" {
			set[role] = true
		}
	}
	return set
}

func (s RoleSet) Has(role Role) bool {
	if s == nil {
		return false
	}
	return s[role]
}

func (s RoleSet) HasAny(roles ...Role) bool {
	for _, role := range roles {
		if s.Has(role) {
			return true
		}
	}
	return false
}

func (s RoleSet) HasAll(roles ...Role) bool {
	for _, role := range roles {
		if !s.Has(role) {
			return false
		}
	}
	return true
}

func (s RoleSet) Slice() []Role {
	roles := make([]Role, 0, len(s))
	for role := range s {
		roles = append(roles, role)
	}
	return roles
}

func (s RoleSet) Clone() RoleSet {
	clone := make(RoleSet, len(s))
	for role, ok := range s {
		clone[role] = ok
	}
	return clone
}

func RoleSetFromSlice(roles []Role) RoleSet {
	set := make(RoleSet, len(roles))
	for _, role := range roles {
		if role != "" {
			set[role] = true
		}
	}
	return set
}

func RequiredSubmitRole(kind OperationKind) Role {
	switch kind {
	case OperationInternal:
		return RoleInternal
	case OperationWithdrawal:
		return RoleWithdrawal
	default:
		return RoleOperator
	}
}

func RoleCanApprove(role Role) bool {
	switch role {
	case RoleAdmin, RoleTreasurer, RoleRisk, RoleApprover:
		return true
	default:
		return false
	}
}

func RoleWeight(role Role) uint8 {
	switch role {
	case RoleAdmin:
		return 3
	case RoleRisk:
		return 2
	case RoleTreasurer, RoleApprover:
		return 1
	default:
		return 0
	}
}

type Principal struct {
	ID            PrincipalID   `json:"id"`
	Institution   InstitutionID `json:"institution"`
	DisplayName   string        `json:"displayName,omitempty"`
	Roles         []Role        `json:"roles"`
	Enabled       bool          `json:"enabled"`
	CreatedEpoch  uint64        `json:"createdEpoch,omitempty"`
	DisabledEpoch uint64        `json:"disabledEpoch,omitempty"`
}

func (p Principal) Validate() error {
	if err := ValidateID("principal", p.ID.String()); err != nil {
		return err
	}
	if err := ValidateID("institution", p.Institution.String()); err != nil {
		return err
	}
	if len(p.Roles) == 0 {
		return Invalidf("principal %s requires at least one role", p.ID)
	}
	return nil
}

func (p Principal) RoleSet() RoleSet {
	return RoleSetFromSlice(p.Roles)
}

func (p Principal) CanSubmit(kind OperationKind) bool {
	if !p.Enabled {
		return false
	}
	roles := p.RoleSet()
	return roles.Has(RoleAdmin) || roles.Has(RoleOperator) || roles.Has(RequiredSubmitRole(kind))
}

func (p Principal) CanManageSigners() bool {
	if !p.Enabled {
		return false
	}
	roles := p.RoleSet()
	return roles.Has(RoleAdmin) || roles.Has(RoleRisk)
}
