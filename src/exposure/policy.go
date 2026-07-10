package exposure

import "github.com/solguardlabs/bastilledtl/src/domain"

type CapAdmission struct {
	CapID     domain.CapID `json:"capId"`
	Limit     domain.Money `json:"limit"`
	Current   domain.Money `json:"current"`
	Projected domain.Money `json:"projected"`
	Accepted  bool         `json:"accepted"`
}

type Admission struct {
	Operation  domain.OperationID `json:"operation"`
	Accepted   bool               `json:"accepted"`
	Reason     string             `json:"reason,omitempty"`
	CapResults []CapAdmission     `json:"capResults"`
}

type Flow struct {
	Epoch       uint64               `json:"epoch"`
	CapID       domain.CapID         `json:"capId"`
	Operation   domain.OperationID   `json:"operation"`
	Institution domain.InstitutionID `json:"institution"`
	Account     domain.AccountID     `json:"account"`
	Asset       domain.AssetID       `json:"asset"`
	Kind        domain.OperationKind `json:"kind"`
	Amount      domain.Money         `json:"amount"`
	Reason      string               `json:"reason,omitempty"`
}

func BuildDefaultCaps(institution domain.InstitutionID, operating domain.AccountID, asset domain.AssetID, policy domain.EnginePolicy) []domain.ExposureCap {
	return []domain.ExposureCap{
		{
			ID:          domain.CapID(domain.JoinScope("cap", institution.String(), asset.String(), "withdrawal")),
			Institution: institution,
			Account:     operating,
			Asset:       asset,
			Kind:        domain.OperationWithdrawal,
			Limit:       policy.WithdrawalDailyCap,
			Window:      24,
			Enabled:     true,
		},
		{
			ID:          domain.CapID(domain.JoinScope("cap", institution.String(), asset.String(), "internal")),
			Institution: institution,
			Account:     operating,
			Asset:       asset,
			Kind:        domain.OperationInternal,
			Limit:       policy.InternalDailyCap,
			Window:      24,
			Enabled:     true,
		},
	}
}

func AdmissionIssue(admission Admission) *domain.AuditIssue {
	if admission.Accepted {
		return nil
	}
	fields := map[string]string{}
	for _, cap := range admission.CapResults {
		if !cap.Accepted {
			fields[cap.CapID.String()] = cap.Projected.String()
		}
	}
	return &domain.AuditIssue{
		Code:      "admission_rejected",
		Severity:  "high",
		Message:   admission.Reason,
		Operation: admission.Operation,
		Fields:    fields,
	}
}
