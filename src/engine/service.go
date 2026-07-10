package engine

import (
	"sync"

	"github.com/solguardlabs/bastilledtl/src/authz"
	"github.com/solguardlabs/bastilledtl/src/domain"
	"github.com/solguardlabs/bastilledtl/src/exposure"
	"github.com/solguardlabs/bastilledtl/src/ledger"
)

type Service struct {
	mu           sync.Mutex
	epoch        uint64
	operationSeq uint64
	eventSeq     uint64
	institutions map[domain.InstitutionID]domain.Institution
	assets       map[domain.AssetID]domain.Asset
	accounts     map[domain.AccountID]domain.Account
	principals   map[domain.PrincipalID]domain.Principal
	policy       domain.EnginePolicy
	ledger       *ledger.Book
	exposure     *exposure.Book
	signers      *authz.SignerRegistry
	approvals    *authz.ApprovalRegistry
	verifier     authz.Verifier
	operations   map[domain.OperationID]domain.OperationRecord
	events       []domain.Event
}

func NewService(bootstrap Bootstrap) (*Service, error) {
	bootstrap = bootstrap.Normalize()
	if err := bootstrap.Validate(); err != nil {
		return nil, err
	}
	institutions := make(map[domain.InstitutionID]domain.Institution)
	for _, institution := range bootstrap.Institutions {
		institutions[institution.ID] = institution
	}
	assets := make(map[domain.AssetID]domain.Asset)
	for _, asset := range bootstrap.Assets {
		assets[asset.ID] = asset
	}
	accounts := make(map[domain.AccountID]domain.Account)
	for _, account := range bootstrap.Accounts {
		accounts[account.ID] = account
	}
	principals := make(map[domain.PrincipalID]domain.Principal)
	for _, principal := range bootstrap.Principals {
		principals[principal.ID] = principal
	}
	book := ledger.NewBook()
	if err := book.ApplySeeds(bootstrap.Balances); err != nil {
		return nil, err
	}
	exposureBook := exposure.NewBook()
	if err := exposureBook.RegisterMany(bootstrap.ExposureCaps); err != nil {
		return nil, err
	}
	signers, err := authz.NewSignerRegistry(bootstrap.Signers)
	if err != nil {
		return nil, err
	}
	approvalRegistry := authz.NewApprovalRegistry()
	service := &Service{
		institutions: institutions,
		assets:       assets,
		accounts:     accounts,
		principals:   principals,
		policy:       bootstrap.Policy.Normalize(),
		ledger:       book,
		exposure:     exposureBook,
		signers:      signers,
		approvals:    approvalRegistry,
		verifier:     authz.NewVerifier(signers, approvalRegistry, bootstrap.Policy.Normalize()),
		operations:   make(map[domain.OperationID]domain.OperationRecord),
		events:       make([]domain.Event, 0, 128),
	}
	service.record(domain.NewEvent("", domain.EventServiceStarted, 0, "service initialized"))
	return service, nil
}

func (s *Service) Policy() domain.EnginePolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy
}

func (s *Service) Epoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

func (s *Service) AdvanceEpoch(delta uint64) uint64 {
	if delta == 0 {
		delta = 1
	}
	s.mu.Lock()
	s.epoch += delta
	epoch := s.epoch
	s.mu.Unlock()
	s.record(domain.NewEvent("", domain.EventEpochAdvanced, epoch, "epoch advanced"))
	return epoch
}

func (s *Service) Approve(request domain.ApprovalRequest) (domain.Approval, error) {
	epoch := s.Epoch()
	if err := s.validateOperationContext(request.Operation); err != nil {
		return domain.Approval{}, err
	}
	signer, err := s.signers.Active(request.SignerID, epoch)
	if err != nil {
		return domain.Approval{}, err
	}
	approval, err := s.approvals.Create(request, signer, epoch, s.policy)
	if err != nil {
		return domain.Approval{}, err
	}
	eventType := domain.EventApprovalCreated
	if approval.Decision == domain.ApprovalDecisionReject {
		eventType = domain.EventApprovalRejected
	}
	s.record(domain.NewEvent("", eventType, epoch, "approval recorded").
		WithInstitution(approval.Institution).
		WithOperation(approval.OperationID).
		WithApproval(approval.ID).
		WithSigner(approval.SignerID).
		WithAmount(approval.Asset, approval.Amount).
		WithField("partialHash", approval.PartialHash))
	return approval, nil
}

func (s *Service) Execute(operation domain.Operation) (ExecutionReceipt, error) {
	epoch := s.Epoch()
	if operation.ID == "" {
		operation.ID = s.nextOperationID()
	}
	partial := authz.PartialEconomicHash(operation)
	full := authz.FullOperationHash(operation)
	record := domain.NewOperationRecord(operation, partial, full)
	s.recordOperation(record)
	if err := s.validateOperationContext(operation); err != nil {
		s.rejectOperation(operation.ID, err.Error())
		return ExecutionReceipt{}, err
	}
	if err := s.validateOperationPolicy(operation); err != nil {
		s.rejectOperation(operation.ID, err.Error())
		return ExecutionReceipt{}, err
	}
	bundle, err := s.verifier.Authorize(operation, epoch)
	if err != nil {
		s.rejectOperation(operation.ID, err.Error())
		return ExecutionReceipt{}, err
	}
	admission, err := s.exposure.Admit(operation)
	if err != nil {
		s.rejectOperation(operation.ID, err.Error())
		return ExecutionReceipt{}, err
	}
	entry, err := s.applyLedger(operation, epoch)
	if err != nil {
		s.rejectOperation(operation.ID, err.Error())
		return ExecutionReceipt{}, err
	}
	admission, err = s.exposure.Apply(operation, epoch)
	if err != nil {
		_ = s.exposure.Reverse(operation, epoch, "ledger rollback unavailable")
		s.rejectOperation(operation.ID, err.Error())
		return ExecutionReceipt{}, err
	}
	s.approvals.MarkUsed(bundle.Accepted, operation, epoch)
	receipt := ExecutionReceipt{
		Operation:       operation.ID,
		Kind:            operation.Kind,
		Institution:     operation.Institution,
		SourceAccount:   operation.SourceAccount,
		Destination:     operation.DestinationKey(),
		Asset:           operation.Asset,
		Amount:          operation.Amount,
		LedgerEntry:     entry.ID,
		ApprovalBundle:  bundle,
		Exposure:        admission,
		PartialHash:     partial,
		FullHash:        full,
		ExternalRail:    operation.ExternalRail,
		ExternalAddress: operation.ExternalBeneficiary,
	}
	s.completeOperation(operation.ID, bundle, admission)
	event := domain.NewEvent("", domain.EventOperationExecuted, epoch, "operation executed").
		WithInstitution(operation.Institution).
		WithOperation(operation.ID).
		WithAmount(operation.Asset, operation.Amount).
		WithField("kind", string(operation.Kind)).
		WithField("partialHash", partial)
	if bundle.ReuseDetected {
		event = event.WithField("approvalReuse", "true")
	}
	s.record(event)
	return receipt, nil
}

func (s *Service) Rotate(request domain.SignerRotation) (RotationReceipt, error) {
	epoch := s.Epoch()
	actor, err := s.principal(request.Actor)
	if err != nil {
		return RotationReceipt{}, err
	}
	retired, activated, err := s.signers.Rotate(request.Retire, request.Activate, actor, epoch, s.policy)
	if err != nil {
		return RotationReceipt{}, err
	}
	s.record(domain.NewEvent("", domain.EventSignerRotated, epoch, "signer rotated").
		WithInstitution(activated.Institution).
		WithSigner(activated.ID).
		WithField("retired", retired.ID.String()).
		WithField("reason", request.Reason))
	return RotationReceipt{Retired: retired, Activated: activated, Epoch: epoch}, nil
}

func (s *Service) Snapshot() domain.SystemSnapshot {
	s.mu.Lock()
	institutions := sortedInstitutions(s.institutions)
	assets := sortedAssets(s.assets)
	accounts := sortedAccounts(s.accounts)
	principals := sortedPrincipals(s.principals)
	operations := make([]domain.OperationRecord, 0, len(s.operations))
	for _, operation := range s.operations {
		operations = append(operations, operation)
	}
	events := append([]domain.Event(nil), s.events...)
	epoch := s.epoch
	s.mu.Unlock()
	sortOperations(operations)
	issues := make([]domain.AuditIssue, 0)
	issues = append(issues, s.ledger.AssertSolvent()...)
	issues = append(issues, s.exposure.Issues()...)
	issues = append(issues, EvaluateApprovalReuse(operations, s.approvals.List())...)
	return domain.SystemSnapshot{
		Epoch:        epoch,
		Institutions: institutions,
		Assets:       assets,
		Accounts:     accounts,
		Principals:   principals,
		Signers:      s.signers.List(),
		Balances:     s.ledger.Snapshots(),
		Exposures:    s.exposure.Snapshot(),
		Approvals:    s.approvals.List(),
		Operations:   operations,
		Events:       events,
		AuditIssues:  issues,
	}
}

func (s *Service) record(event domain.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventSeq++
	event.ID = domain.NewEventID(s.epoch, s.eventSeq)
	if event.Epoch == 0 {
		event.Epoch = s.epoch
	}
	s.events = append(s.events, event)
}

func (s *Service) nextOperationID() domain.OperationID {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operationSeq++
	return domain.NewOperationID(s.epoch, s.operationSeq)
}
