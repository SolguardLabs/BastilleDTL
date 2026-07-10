package domain

import "strings"

type OperationKind string

const (
	OperationInternal   OperationKind = "internal"
	OperationWithdrawal OperationKind = "withdrawal"
)

type OperationStatus string

const (
	OperationDraft     OperationStatus = "draft"
	OperationApproved  OperationStatus = "approved"
	OperationExecuted  OperationStatus = "executed"
	OperationRejected  OperationStatus = "rejected"
	OperationCancelled OperationStatus = "cancelled"
)

type RiskBucket string

const (
	RiskLow      RiskBucket = "low"
	RiskStandard RiskBucket = "standard"
	RiskElevated RiskBucket = "elevated"
	RiskCritical RiskBucket = "critical"
)

type Operation struct {
	ID                  OperationID       `json:"id"`
	Institution         InstitutionID     `json:"institution"`
	Kind                OperationKind     `json:"kind"`
	SourceAccount       AccountID         `json:"sourceAccount"`
	DestinationAccount  AccountID         `json:"destinationAccount,omitempty"`
	ExternalBeneficiary string            `json:"externalBeneficiary,omitempty"`
	ExternalRail        string            `json:"externalRail,omitempty"`
	Asset               AssetID           `json:"asset"`
	Amount              Money             `json:"amount"`
	RequestedBy         PrincipalID       `json:"requestedBy"`
	Memo                string            `json:"memo,omitempty"`
	RiskBucket          RiskBucket        `json:"riskBucket,omitempty"`
	ApprovalIDs         []ApprovalID      `json:"approvalIds,omitempty"`
	IdempotencyKey      string            `json:"idempotencyKey,omitempty"`
	CreatedEpoch        uint64            `json:"createdEpoch,omitempty"`
	ExpiresEpoch        uint64            `json:"expiresEpoch,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

func (o Operation) Validate() error {
	if err := ValidateID("operation", o.ID.String()); err != nil {
		return err
	}
	if err := ValidateID("institution", o.Institution.String()); err != nil {
		return err
	}
	if o.Kind != OperationInternal && o.Kind != OperationWithdrawal {
		return Invalidf("operation %s kind is unsupported", o.ID)
	}
	if err := ValidateID("source account", o.SourceAccount.String()); err != nil {
		return err
	}
	if o.Kind == OperationInternal {
		if err := ValidateID("destination account", o.DestinationAccount.String()); err != nil {
			return err
		}
	}
	if o.Kind == OperationWithdrawal {
		if strings.TrimSpace(o.ExternalBeneficiary) == "" {
			return Invalidf("operation %s external beneficiary is required", o.ID)
		}
		if strings.TrimSpace(o.ExternalRail) == "" {
			return Invalidf("operation %s external rail is required", o.ID)
		}
	}
	if err := ValidateID("asset", o.Asset.String()); err != nil {
		return err
	}
	if err := MustPositiveMoney("amount", o.Amount); err != nil {
		return err
	}
	if err := ValidateID("requested by", o.RequestedBy.String()); err != nil {
		return err
	}
	return nil
}

func (o Operation) NeedsExternalRail() bool {
	return o.Kind == OperationWithdrawal
}

func (o Operation) EffectiveRiskBucket() RiskBucket {
	if o.RiskBucket != "" {
		return o.RiskBucket
	}
	if o.Amount >= 500000 {
		return RiskCritical
	}
	if o.Amount >= 100000 {
		return RiskElevated
	}
	if o.Amount >= 25000 {
		return RiskStandard
	}
	return RiskLow
}

func (o Operation) DestinationKey() string {
	if o.Kind == OperationInternal {
		return o.DestinationAccount.String()
	}
	return strings.ToLower(strings.TrimSpace(o.ExternalBeneficiary))
}

func (o Operation) IsLarge(threshold Money) bool {
	return o.Amount >= threshold
}

func (o Operation) CloneWithApprovals(ids []ApprovalID) Operation {
	clone := o
	clone.ApprovalIDs = append([]ApprovalID(nil), ids...)
	if o.Metadata != nil {
		clone.Metadata = make(map[string]string, len(o.Metadata))
		for key, value := range o.Metadata {
			clone.Metadata[key] = value
		}
	}
	return clone
}

type OperationRecord struct {
	Operation       Operation       `json:"operation"`
	Status          OperationStatus `json:"status"`
	PartialHash     string          `json:"partialHash"`
	FullHash        string          `json:"fullHash"`
	ApprovalIDs     []ApprovalID    `json:"approvalIds,omitempty"`
	ExecutedEpoch   uint64          `json:"executedEpoch,omitempty"`
	RejectedReason  string          `json:"rejectedReason,omitempty"`
	ExposureApplied Money           `json:"exposureApplied,omitempty"`
}

func NewOperationRecord(operation Operation, partialHash string, fullHash string) OperationRecord {
	return OperationRecord{
		Operation:   operation,
		Status:      OperationDraft,
		PartialHash: partialHash,
		FullHash:    fullHash,
		ApprovalIDs: append([]ApprovalID(nil), operation.ApprovalIDs...),
	}
}

func (r OperationRecord) IsTerminal() bool {
	return r.Status == OperationExecuted || r.Status == OperationRejected || r.Status == OperationCancelled
}
