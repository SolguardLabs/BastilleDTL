package authz

import (
	"fmt"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type Verifier struct {
	signers   *SignerRegistry
	approvals *ApprovalRegistry
	policy    domain.EnginePolicy
}

func NewVerifier(signers *SignerRegistry, approvals *ApprovalRegistry, policy domain.EnginePolicy) Verifier {
	return Verifier{signers: signers, approvals: approvals, policy: policy.Normalize()}
}

func (v Verifier) Authorize(operation domain.Operation, epoch uint64) (domain.ApprovalBundle, error) {
	if err := operation.Validate(); err != nil {
		return domain.ApprovalBundle{}, err
	}
	partial := PartialEconomicHash(operation)
	full := FullOperationHash(operation)
	required := v.policy.RequiredApprovals
	if !operation.IsLarge(v.policy.LargeTransferThreshold) {
		return domain.ApprovalBundle{
			Required:       0,
			Accepted:       nil,
			SignerIDs:      nil,
			Weight:         0,
			PartialHash:    partial,
			FullHash:       full,
			OperationBound: true,
		}, nil
	}
	if len(operation.ApprovalIDs) == 0 {
		return domain.ApprovalBundle{Required: required, PartialHash: partial, FullHash: full}, domain.ApprovalRequired("large operation requires approvals")
	}
	approvals, err := v.approvals.Resolve(operation.ApprovalIDs)
	if err != nil {
		return domain.ApprovalBundle{}, err
	}
	accepted := make([]domain.Approval, 0, len(approvals))
	rejected := make([]domain.Approval, 0)
	seenSigners := map[domain.SignerID]bool{}
	var weight uint8
	operationBound := true
	reuseDetected := false
	for _, approval := range approvals {
		if approval.PartialHash != partial {
			rejected = append(rejected, approval)
			continue
		}
		if !approval.IsUsable(epoch) {
			rejected = append(rejected, approval)
			continue
		}
		if seenSigners[approval.SignerID] {
			return domain.ApprovalBundle{}, domain.SignerConflictf("signer %s appears more than once", approval.SignerID)
		}
		signer, err := v.signers.Active(approval.SignerID, epoch)
		if err != nil {
			return domain.ApprovalBundle{}, err
		}
		if signer.Institution != operation.Institution {
			return domain.ApprovalBundle{}, domain.Permission("approval signer belongs to a different institution")
		}
		if !v.policy.RoleAllowed(signer.Role) {
			return domain.ApprovalBundle{}, domain.Permissionf("signer role %s is not allowed", signer.Role)
		}
		if !signer.CanApprove(operation.Amount) {
			return domain.ApprovalBundle{}, domain.Permissionf("signer %s cannot approve amount %s", signer.ID, operation.Amount)
		}
		if approval.OperationID != operation.ID || approval.FullHash != full {
			operationBound = false
			reuseDetected = true
		}
		seenSigners[approval.SignerID] = true
		weight += signer.Weight
		accepted = append(accepted, approval)
	}
	bundle := domain.ApprovalBundle{
		Required:       required,
		Accepted:       accepted,
		Rejected:       rejected,
		SignerIDs:      signerIDs(accepted),
		Weight:         weight,
		PartialHash:    partial,
		FullHash:       full,
		OperationBound: operationBound,
		ReuseDetected:  reuseDetected,
	}
	if uint8(len(accepted)) < required || weight < required {
		return bundle, domain.ApprovalRejectedf("operation %s has %d approvals and weight %d, requires %d", operation.ID, len(accepted), weight, required)
	}
	return bundle, nil
}

func (v Verifier) Explain(operation domain.Operation, epoch uint64) string {
	bundle, err := v.Authorize(operation, epoch)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("approved=%t required=%d accepted=%d weight=%d reuse=%t", bundle.Approved(), bundle.Required, len(bundle.Accepted), bundle.Weight, bundle.ReuseDetected)
}

func signerIDs(approvals []domain.Approval) []domain.SignerID {
	ids := make([]domain.SignerID, 0, len(approvals))
	for _, approval := range approvals {
		ids = append(ids, approval.SignerID)
	}
	return ids
}
