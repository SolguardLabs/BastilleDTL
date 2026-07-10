package domain

type EnginePolicy struct {
	LargeTransferThreshold Money    `json:"largeTransferThreshold"`
	RequiredApprovals      uint8    `json:"requiredApprovals"`
	ApprovalTTL            uint64   `json:"approvalTtl"`
	MaxApprovalAmount      Money    `json:"maxApprovalAmount"`
	WithdrawalDailyCap     Money    `json:"withdrawalDailyCap"`
	InternalDailyCap       Money    `json:"internalDailyCap"`
	AllowApprovalReuse     bool     `json:"allowApprovalReuse"`
	MinimumSignerWeight    uint8    `json:"minimumSignerWeight"`
	AllowedApprovalRoles   []Role   `json:"allowedApprovalRoles"`
	AllowedWithdrawalRails []string `json:"allowedWithdrawalRails"`
	StrictDestinationCheck bool     `json:"strictDestinationCheck"`
	RotationDelayEpochs    uint64   `json:"rotationDelayEpochs"`
}

func DefaultPolicy() EnginePolicy {
	return EnginePolicy{
		LargeTransferThreshold: 50000,
		RequiredApprovals:      2,
		ApprovalTTL:            24,
		MaxApprovalAmount:      900000,
		WithdrawalDailyCap:     700000,
		InternalDailyCap:       1200000,
		AllowApprovalReuse:     true,
		MinimumSignerWeight:    1,
		AllowedApprovalRoles:   []Role{RoleAdmin, RoleTreasurer, RoleRisk, RoleApprover},
		AllowedWithdrawalRails: []string{"swift", "sepa", "wire", "ach"},
		StrictDestinationCheck: false,
		RotationDelayEpochs:    0,
	}
}

func (p EnginePolicy) Normalize() EnginePolicy {
	def := DefaultPolicy()
	if p.LargeTransferThreshold <= 0 {
		p.LargeTransferThreshold = def.LargeTransferThreshold
	}
	if p.RequiredApprovals == 0 {
		p.RequiredApprovals = def.RequiredApprovals
	}
	if p.ApprovalTTL == 0 {
		p.ApprovalTTL = def.ApprovalTTL
	}
	if p.MaxApprovalAmount <= 0 {
		p.MaxApprovalAmount = def.MaxApprovalAmount
	}
	if p.WithdrawalDailyCap <= 0 {
		p.WithdrawalDailyCap = def.WithdrawalDailyCap
	}
	if p.InternalDailyCap <= 0 {
		p.InternalDailyCap = def.InternalDailyCap
	}
	if p.MinimumSignerWeight == 0 {
		p.MinimumSignerWeight = def.MinimumSignerWeight
	}
	if len(p.AllowedApprovalRoles) == 0 {
		p.AllowedApprovalRoles = def.AllowedApprovalRoles
	}
	if len(p.AllowedWithdrawalRails) == 0 {
		p.AllowedWithdrawalRails = def.AllowedWithdrawalRails
	}
	return p
}

func (p EnginePolicy) Validate() error {
	if p.LargeTransferThreshold <= 0 {
		return Invalid("large transfer threshold must be positive")
	}
	if p.RequiredApprovals == 0 {
		return Invalid("required approvals must be positive")
	}
	if p.ApprovalTTL == 0 {
		return Invalid("approval ttl must be positive")
	}
	if p.MaxApprovalAmount < p.LargeTransferThreshold {
		return Invalid("max approval amount must cover large transfer threshold")
	}
	if p.MinimumSignerWeight == 0 {
		return Invalid("minimum signer weight must be positive")
	}
	return nil
}

func (p EnginePolicy) RoleAllowed(role Role) bool {
	for _, allowed := range p.AllowedApprovalRoles {
		if allowed == role {
			return true
		}
	}
	return false
}

func (p EnginePolicy) RailAllowed(rail string) bool {
	for _, allowed := range p.AllowedWithdrawalRails {
		if allowed == rail {
			return true
		}
	}
	return false
}

type ExposureCap struct {
	ID          CapID         `json:"id"`
	Institution InstitutionID `json:"institution"`
	Account     AccountID     `json:"account,omitempty"`
	Asset       AssetID       `json:"asset"`
	Kind        OperationKind `json:"kind,omitempty"`
	Limit       Money         `json:"limit"`
	Window      uint64        `json:"window,omitempty"`
	Enabled     bool          `json:"enabled"`
}

func (c ExposureCap) Validate() error {
	if err := ValidateID("cap", c.ID.String()); err != nil {
		return err
	}
	if err := ValidateID("institution", c.Institution.String()); err != nil {
		return err
	}
	if err := ValidateID("asset", c.Asset.String()); err != nil {
		return err
	}
	if c.Limit <= 0 {
		return Invalidf("cap %s limit must be positive", c.ID)
	}
	return nil
}

func (c ExposureCap) Matches(operation Operation) bool {
	if !c.Enabled {
		return false
	}
	if c.Institution != operation.Institution {
		return false
	}
	if c.Asset != operation.Asset {
		return false
	}
	if c.Account != "" && c.Account != operation.SourceAccount {
		return false
	}
	if c.Kind != "" && c.Kind != operation.Kind {
		return false
	}
	return true
}
