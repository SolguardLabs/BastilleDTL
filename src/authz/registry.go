package authz

import (
	"sort"
	"sync"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type ApprovalRegistry struct {
	mu        sync.Mutex
	approvals map[domain.ApprovalID]domain.Approval
	uses      []domain.ApprovalUse
	seq       uint64
}

func NewApprovalRegistry() *ApprovalRegistry {
	return &ApprovalRegistry{
		approvals: make(map[domain.ApprovalID]domain.Approval),
		uses:      make([]domain.ApprovalUse, 0, 128),
	}
}

func (r *ApprovalRegistry) Create(request domain.ApprovalRequest, signer domain.Signer, epoch uint64, policy domain.EnginePolicy) (domain.Approval, error) {
	if err := request.Validate(); err != nil {
		return domain.Approval{}, err
	}
	if signer.Institution != request.Operation.Institution {
		return domain.Approval{}, domain.Permission("signer belongs to a different institution")
	}
	if !signer.ActiveAt(epoch) {
		return domain.Approval{}, domain.SignerInactivef("signer %s is not active", signer.ID)
	}
	if !policy.RoleAllowed(signer.Role) || !signer.CanApprove(request.Operation.Amount) {
		return domain.Approval{}, domain.Permissionf("signer %s cannot approve amount %s", signer.ID, request.Operation.Amount)
	}
	if request.Operation.Amount > policy.MaxApprovalAmount {
		return domain.Approval{}, domain.Limitf("operation amount %s exceeds max approval amount %s", request.Operation.Amount, policy.MaxApprovalAmount)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	status := domain.ApprovalActive
	if request.Decision == domain.ApprovalDecisionReject {
		status = domain.ApprovalRevoked
	}
	approval := domain.Approval{
		ID:            domain.NewApprovalID(epoch, r.seq),
		OperationID:   request.Operation.ID,
		Institution:   request.Operation.Institution,
		SignerID:      signer.ID,
		SignerRole:    signer.Role,
		Decision:      request.Decision,
		Status:        status,
		PartialHash:   PartialEconomicHash(request.Operation),
		FullHash:      FullOperationHash(request.Operation),
		OperationKind: request.Operation.Kind,
		Amount:        request.Operation.Amount,
		Asset:         request.Operation.Asset,
		SourceAccount: request.Operation.SourceAccount,
		CreatedEpoch:  epoch,
		ExpiresEpoch:  epoch + policy.ApprovalTTL,
		Reason:        request.Reason,
		SignerWeight:  signer.Weight,
	}
	if err := approval.Validate(); err != nil {
		return domain.Approval{}, err
	}
	r.approvals[approval.ID] = approval
	return approval, nil
}

func (r *ApprovalRegistry) Get(id domain.ApprovalID) (domain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	approval, ok := r.approvals[id]
	if !ok {
		return domain.Approval{}, domain.NotFoundf("approval %s not found", id)
	}
	return approval, nil
}

func (r *ApprovalRegistry) Resolve(ids []domain.ApprovalID) ([]domain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	approvals := make([]domain.Approval, 0, len(ids))
	for _, id := range ids {
		approval, ok := r.approvals[id]
		if !ok {
			return nil, domain.NotFoundf("approval %s not found", id)
		}
		approvals = append(approvals, approval)
	}
	return approvals, nil
}

func (r *ApprovalRegistry) FindByPartialHash(hash string) []domain.Approval {
	r.mu.Lock()
	defer r.mu.Unlock()
	approvals := make([]domain.Approval, 0)
	for _, approval := range r.approvals {
		if approval.PartialHash == hash {
			approvals = append(approvals, approval)
		}
	}
	sort.Slice(approvals, func(i int, j int) bool {
		return approvals[i].ID < approvals[j].ID
	})
	return approvals
}

func (r *ApprovalRegistry) MarkUsed(approvals []domain.Approval, operation domain.Operation, epoch uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, approval := range approvals {
		current, ok := r.approvals[approval.ID]
		if !ok {
			continue
		}
		already := false
		for _, used := range current.ConsumedBy {
			if used == operation.ID {
				already = true
				break
			}
		}
		if !already {
			current.ConsumedBy = append(current.ConsumedBy, operation.ID)
		}
		r.approvals[current.ID] = current
		r.uses = append(r.uses, domain.ApprovalUse{
			ApprovalID:  current.ID,
			OperationID: operation.ID,
			Epoch:       epoch,
			PartialHash: PartialEconomicHash(operation),
			FullHash:    FullOperationHash(operation),
		})
	}
}

func (r *ApprovalRegistry) Revoke(id domain.ApprovalID, reason string) (domain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	approval, ok := r.approvals[id]
	if !ok {
		return domain.Approval{}, domain.NotFoundf("approval %s not found", id)
	}
	approval.Status = domain.ApprovalRevoked
	approval.Reason = reason
	r.approvals[id] = approval
	return approval, nil
}

func (r *ApprovalRegistry) List() []domain.Approval {
	r.mu.Lock()
	defer r.mu.Unlock()
	approvals := make([]domain.Approval, 0, len(r.approvals))
	for _, approval := range r.approvals {
		approvals = append(approvals, approval)
	}
	sort.Slice(approvals, func(i int, j int) bool {
		return approvals[i].ID < approvals[j].ID
	})
	return approvals
}

func (r *ApprovalRegistry) Uses() []domain.ApprovalUse {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.ApprovalUse(nil), r.uses...)
}
