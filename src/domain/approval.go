package domain

type ApprovalDecision string

const (
	ApprovalDecisionApprove ApprovalDecision = "approve"
	ApprovalDecisionReject  ApprovalDecision = "reject"
)

type ApprovalStatus string

const (
	ApprovalActive   ApprovalStatus = "active"
	ApprovalConsumed ApprovalStatus = "consumed"
	ApprovalExpired  ApprovalStatus = "expired"
	ApprovalRevoked  ApprovalStatus = "revoked"
)

type Approval struct {
	ID            ApprovalID       `json:"id"`
	OperationID   OperationID      `json:"operationId"`
	Institution   InstitutionID    `json:"institution"`
	SignerID      SignerID         `json:"signerId"`
	SignerRole    Role             `json:"signerRole"`
	Decision      ApprovalDecision `json:"decision"`
	Status        ApprovalStatus   `json:"status"`
	PartialHash   string           `json:"partialHash"`
	FullHash      string           `json:"fullHash"`
	OperationKind OperationKind    `json:"operationKind"`
	Amount        Money            `json:"amount"`
	Asset         AssetID          `json:"asset"`
	SourceAccount AccountID        `json:"sourceAccount"`
	CreatedEpoch  uint64           `json:"createdEpoch"`
	ExpiresEpoch  uint64           `json:"expiresEpoch"`
	ConsumedBy    []OperationID    `json:"consumedBy,omitempty"`
	Reason        string           `json:"reason,omitempty"`
	SignerWeight  uint8            `json:"signerWeight"`
}

func (a Approval) Validate() error {
	if err := ValidateID("approval", a.ID.String()); err != nil {
		return err
	}
	if err := ValidateID("operation", a.OperationID.String()); err != nil {
		return err
	}
	if err := ValidateID("institution", a.Institution.String()); err != nil {
		return err
	}
	if err := ValidateID("signer", a.SignerID.String()); err != nil {
		return err
	}
	if a.Decision != ApprovalDecisionApprove && a.Decision != ApprovalDecisionReject {
		return Invalidf("approval %s decision is invalid", a.ID)
	}
	if a.Status == "" {
		return Invalidf("approval %s status is required", a.ID)
	}
	if a.PartialHash == "" || a.FullHash == "" {
		return Invalidf("approval %s hashes are required", a.ID)
	}
	return nil
}

func (a Approval) IsUsable(epoch uint64) bool {
	if a.Decision != ApprovalDecisionApprove {
		return false
	}
	if a.Status != ApprovalActive && a.Status != ApprovalConsumed {
		return false
	}
	if a.ExpiresEpoch > 0 && epoch > a.ExpiresEpoch {
		return false
	}
	return true
}

func (a Approval) WasConsumedBy(operation OperationID) bool {
	for _, used := range a.ConsumedBy {
		if used == operation {
			return true
		}
	}
	return false
}

type ApprovalRequest struct {
	Operation Operation        `json:"operation"`
	SignerID  SignerID         `json:"signerId"`
	Decision  ApprovalDecision `json:"decision"`
	Reason    string           `json:"reason,omitempty"`
}

func (r ApprovalRequest) Validate() error {
	if err := r.Operation.Validate(); err != nil {
		return err
	}
	if err := ValidateID("signer", r.SignerID.String()); err != nil {
		return err
	}
	if r.Decision == "" {
		return Invalid("approval decision is required")
	}
	return nil
}

type ApprovalBundle struct {
	Required       uint8      `json:"required"`
	Accepted       []Approval `json:"accepted"`
	Rejected       []Approval `json:"rejected,omitempty"`
	SignerIDs      []SignerID `json:"signerIds"`
	Weight         uint8      `json:"weight"`
	PartialHash    string     `json:"partialHash"`
	FullHash       string     `json:"fullHash"`
	OperationBound bool       `json:"operationBound"`
	ReuseDetected  bool       `json:"reuseDetected"`
}

func (b ApprovalBundle) Approved() bool {
	return uint8(len(b.Accepted)) >= b.Required && b.Weight >= b.Required
}

type ApprovalUse struct {
	ApprovalID  ApprovalID  `json:"approvalId"`
	OperationID OperationID `json:"operationId"`
	Epoch       uint64      `json:"epoch"`
	PartialHash string      `json:"partialHash"`
	FullHash    string      `json:"fullHash"`
}
