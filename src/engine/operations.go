package engine

import (
	"sort"
	"strings"

	"github.com/solguardlabs/bastilledtl/src/domain"
	"github.com/solguardlabs/bastilledtl/src/exposure"
	"github.com/solguardlabs/bastilledtl/src/ledger"
)

type ExecutionReceipt struct {
	Operation       domain.OperationID    `json:"operation"`
	Kind            domain.OperationKind  `json:"kind"`
	Institution     domain.InstitutionID  `json:"institution"`
	SourceAccount   domain.AccountID      `json:"sourceAccount"`
	Destination     string                `json:"destination"`
	Asset           domain.AssetID        `json:"asset"`
	Amount          domain.Money          `json:"amount"`
	LedgerEntry     domain.LedgerEntryID  `json:"ledgerEntry"`
	ApprovalBundle  domain.ApprovalBundle `json:"approvalBundle"`
	Exposure        exposure.Admission    `json:"exposure"`
	PartialHash     string                `json:"partialHash"`
	FullHash        string                `json:"fullHash"`
	ExternalRail    string                `json:"externalRail,omitempty"`
	ExternalAddress string                `json:"externalAddress,omitempty"`
}

type RotationReceipt struct {
	Retired   domain.Signer `json:"retired"`
	Activated domain.Signer `json:"activated"`
	Epoch     uint64        `json:"epoch"`
}

func (s *Service) validateOperationContext(operation domain.Operation) error {
	if err := operation.Validate(); err != nil {
		return err
	}
	institution, err := s.institution(operation.Institution)
	if err != nil {
		return err
	}
	if !institution.Enabled {
		return domain.Permissionf("institution %s is disabled", institution.ID)
	}
	asset, err := s.asset(operation.Asset)
	if err != nil {
		return err
	}
	if !asset.Enabled {
		return domain.Permissionf("asset %s is disabled", asset.ID)
	}
	source, err := s.account(operation.SourceAccount)
	if err != nil {
		return err
	}
	if source.Institution != operation.Institution {
		return domain.Permissionf("source account %s is outside institution %s", source.ID, operation.Institution)
	}
	if !source.CanDebit(operation.Asset) {
		return domain.AccountBlockedf("source account %s cannot debit %s", source.ID, operation.Asset)
	}
	requester, err := s.principal(operation.RequestedBy)
	if err != nil {
		return err
	}
	if requester.Institution != operation.Institution {
		return domain.Permissionf("requester %s belongs to a different institution", requester.ID)
	}
	if !requester.CanSubmit(operation.Kind) {
		return domain.Permissionf("requester %s cannot submit %s operations", requester.ID, operation.Kind)
	}
	if operation.Kind == domain.OperationInternal {
		destination, err := s.account(operation.DestinationAccount)
		if err != nil {
			return err
		}
		if destination.Institution != operation.Institution {
			return domain.Permissionf("destination account %s is outside institution %s", destination.ID, operation.Institution)
		}
		if !destination.CanCredit(operation.Asset) {
			return domain.AccountBlockedf("destination account %s cannot credit %s", destination.ID, operation.Asset)
		}
	}
	if operation.Kind == domain.OperationWithdrawal {
		if !s.policy.RailAllowed(strings.ToLower(operation.ExternalRail)) {
			return domain.Permissionf("withdrawal rail %s is not allowed", operation.ExternalRail)
		}
	}
	return nil
}

func (s *Service) validateOperationPolicy(operation domain.Operation) error {
	if operation.Kind == domain.OperationInternal && operation.Amount > s.policy.InternalDailyCap {
		return domain.Limitf("internal movement amount %s exceeds policy daily cap %s", operation.Amount, s.policy.InternalDailyCap)
	}
	if operation.Kind == domain.OperationWithdrawal && operation.Amount > s.policy.WithdrawalDailyCap {
		return domain.Limitf("withdrawal amount %s exceeds policy daily cap %s", operation.Amount, s.policy.WithdrawalDailyCap)
	}
	return nil
}

func (s *Service) applyLedger(operation domain.Operation, epoch uint64) (ledger.Entry, error) {
	switch operation.Kind {
	case domain.OperationInternal:
		return s.ledger.Transfer(operation.SourceAccount, operation.DestinationAccount, operation.Asset, operation.Amount, operation.ID, operation.Memo, epoch)
	case domain.OperationWithdrawal:
		return s.ledger.Withdraw(operation.SourceAccount, operation.Asset, operation.Amount, operation.ID, operation.ExternalBeneficiary, operation.ExternalRail, epoch)
	default:
		return ledger.Entry{}, domain.Invalidf("unsupported operation kind %s", operation.Kind)
	}
}

func (s *Service) institution(id domain.InstitutionID) (domain.Institution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	institution, ok := s.institutions[id]
	if !ok {
		return domain.Institution{}, domain.NotFoundf("institution %s not found", id)
	}
	return institution, nil
}

func (s *Service) asset(id domain.AssetID) (domain.Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asset, ok := s.assets[id]
	if !ok {
		return domain.Asset{}, domain.NotFoundf("asset %s not found", id)
	}
	return asset, nil
}

func (s *Service) account(id domain.AccountID) (domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[id]
	if !ok {
		return domain.Account{}, domain.NotFoundf("account %s not found", id)
	}
	return account, nil
}

func (s *Service) principal(id domain.PrincipalID) (domain.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	principal, ok := s.principals[id]
	if !ok {
		return domain.Principal{}, domain.NotFoundf("principal %s not found", id)
	}
	if !principal.Enabled {
		return domain.Principal{}, domain.Permissionf("principal %s is disabled", id)
	}
	return principal, nil
}

func (s *Service) recordOperation(record domain.OperationRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[record.Operation.ID] = record
}

func (s *Service) rejectOperation(id domain.OperationID, reason string) {
	s.mu.Lock()
	record := s.operations[id]
	record.Status = domain.OperationRejected
	record.RejectedReason = reason
	s.operations[id] = record
	s.mu.Unlock()
	s.record(domain.NewEvent("", domain.EventOperationRejected, s.Epoch(), "operation rejected").WithOperation(id).WithField("reason", reason))
}

func (s *Service) completeOperation(id domain.OperationID, bundle domain.ApprovalBundle, admission exposure.Admission) {
	s.mu.Lock()
	record := s.operations[id]
	record.Status = domain.OperationExecuted
	record.ApprovalIDs = approvalIDs(bundle.Accepted)
	record.ExecutedEpoch = s.epoch
	record.ExposureApplied = exposureAmount(admission)
	s.operations[id] = record
	s.mu.Unlock()
	s.record(domain.NewEvent("", domain.EventOperationApproved, s.Epoch(), "operation approved").WithOperation(id))
	s.record(domain.NewEvent("", domain.EventExposureApplied, s.Epoch(), "exposure applied").WithOperation(id))
}

func approvalIDs(approvals []domain.Approval) []domain.ApprovalID {
	ids := make([]domain.ApprovalID, 0, len(approvals))
	for _, approval := range approvals {
		ids = append(ids, approval.ID)
	}
	return ids
}

func exposureAmount(admission exposure.Admission) domain.Money {
	var max domain.Money
	for _, result := range admission.CapResults {
		delta := result.Projected - result.Current
		if delta > max {
			max = delta
		}
	}
	return max
}

func sortOperations(operations []domain.OperationRecord) {
	sort.Slice(operations, func(i int, j int) bool {
		return operations[i].Operation.ID < operations[j].Operation.ID
	})
}
