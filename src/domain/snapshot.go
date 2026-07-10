package domain

type BalanceSnapshot struct {
	Account   AccountID `json:"account"`
	Asset     AssetID   `json:"asset"`
	Available Money     `json:"available"`
	Reserved  Money     `json:"reserved"`
}

type ExposureSnapshot struct {
	CapID       CapID         `json:"capId"`
	Institution InstitutionID `json:"institution"`
	Account     AccountID     `json:"account,omitempty"`
	Asset       AssetID       `json:"asset"`
	Kind        OperationKind `json:"kind,omitempty"`
	Limit       Money         `json:"limit"`
	Used        Money         `json:"used"`
	Remaining   Money         `json:"remaining"`
}

type SignerStatus string

const (
	SignerActive    SignerStatus = "active"
	SignerRetired   SignerStatus = "retired"
	SignerSuspended SignerStatus = "suspended"
	SignerPending   SignerStatus = "pending"
)

type Signer struct {
	ID             SignerID      `json:"id"`
	Institution    InstitutionID `json:"institution"`
	Principal      PrincipalID   `json:"principal"`
	Role           Role          `json:"role"`
	Status         SignerStatus  `json:"status"`
	Weight         uint8         `json:"weight"`
	MaxAmount      Money         `json:"maxAmount"`
	ActivatedEpoch uint64        `json:"activatedEpoch"`
	RetiredEpoch   uint64        `json:"retiredEpoch,omitempty"`
	RotationGroup  string        `json:"rotationGroup,omitempty"`
	DelegatedBy    SignerID      `json:"delegatedBy,omitempty"`
	Notes          string        `json:"notes,omitempty"`
}

func (s Signer) Validate() error {
	if err := ValidateID("signer", s.ID.String()); err != nil {
		return err
	}
	if err := ValidateID("institution", s.Institution.String()); err != nil {
		return err
	}
	if err := ValidateID("principal", s.Principal.String()); err != nil {
		return err
	}
	if s.Role == "" {
		return Invalidf("signer %s role is required", s.ID)
	}
	if s.Status == "" {
		return Invalidf("signer %s status is required", s.ID)
	}
	if s.Weight == 0 {
		return Invalidf("signer %s weight must be positive", s.ID)
	}
	if s.MaxAmount <= 0 {
		return Invalidf("signer %s max amount must be positive", s.ID)
	}
	return nil
}

func (s Signer) ActiveAt(epoch uint64) bool {
	if s.Status != SignerActive {
		return false
	}
	if epoch < s.ActivatedEpoch {
		return false
	}
	if s.RetiredEpoch > 0 && epoch >= s.RetiredEpoch {
		return false
	}
	return true
}

func (s Signer) CanApprove(amount Money) bool {
	return RoleCanApprove(s.Role) && amount <= s.MaxAmount && s.Weight > 0
}

type SignerRotation struct {
	Retire   SignerID    `json:"retire"`
	Activate Signer      `json:"activate"`
	Actor    PrincipalID `json:"actor"`
	Reason   string      `json:"reason,omitempty"`
}

type AuditIssue struct {
	Code      string            `json:"code"`
	Severity  string            `json:"severity"`
	Message   string            `json:"message"`
	Operation OperationID       `json:"operation,omitempty"`
	Approval  ApprovalID        `json:"approval,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type SystemSnapshot struct {
	Epoch        uint64             `json:"epoch"`
	Institutions []Institution      `json:"institutions"`
	Assets       []Asset            `json:"assets"`
	Accounts     []Account          `json:"accounts"`
	Principals   []Principal        `json:"principals"`
	Signers      []Signer           `json:"signers"`
	Balances     []BalanceSnapshot  `json:"balances"`
	Exposures    []ExposureSnapshot `json:"exposures"`
	Approvals    []Approval         `json:"approvals"`
	Operations   []OperationRecord  `json:"operations"`
	Events       []Event            `json:"events"`
	AuditIssues  []AuditIssue       `json:"auditIssues"`
}

func (s SystemSnapshot) Balance(account AccountID, asset AssetID) BalanceSnapshot {
	for _, balance := range s.Balances {
		if balance.Account == account && balance.Asset == asset {
			return balance
		}
	}
	return BalanceSnapshot{Account: account, Asset: asset}
}

func (s SystemSnapshot) Exposure(capID CapID) ExposureSnapshot {
	for _, exposure := range s.Exposures {
		if exposure.CapID == capID {
			return exposure
		}
	}
	return ExposureSnapshot{CapID: capID}
}
