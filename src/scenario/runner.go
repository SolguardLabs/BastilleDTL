package scenario

import (
	"fmt"

	"github.com/solguardlabs/bastilledtl/src/domain"
	"github.com/solguardlabs/bastilledtl/src/engine"
)

func Run(definition Definition) (Result, error) {
	if definition.Name == "" {
		definition.Name = "scenario"
	}
	if len(definition.Bootstrap.Institutions) == 0 {
		definition.Bootstrap = DefaultBootstrap()
	}
	service, err := engine.NewService(definition.Bootstrap)
	if err != nil {
		return Result{}, err
	}
	state := &runState{
		service:           service,
		approvals:         map[string]domain.Approval{},
		approvalIDByLabel: map[string]domain.ApprovalID{},
	}
	results := make([]ActionResult, 0, len(definition.Actions))
	for _, action := range definition.Actions {
		result, err := state.runAction(action)
		if action.ExpectError != "" {
			if err == nil {
				return Result{}, fmt.Errorf("action %s expected error %s but succeeded", action.Label, action.ExpectError)
			}
			domainErr, ok := normalizeError(err)
			if !ok {
				return Result{}, err
			}
			if domainErr.Code != action.ExpectError {
				return Result{}, fmt.Errorf("action %s expected error %s but got %s", action.Label, action.ExpectError, domainErr.Code)
			}
			result.Error = &domainErr
			results = append(results, result)
			continue
		}
		if err != nil {
			return Result{}, err
		}
		results = append(results, result)
	}
	snapshot := service.Snapshot()
	return Result{
		Name:     definition.Name,
		Results:  results,
		Snapshot: snapshot,
		Report:   engine.BuildTreasuryReport(snapshot),
	}, nil
}

type runState struct {
	service           *engine.Service
	approvals         map[string]domain.Approval
	approvalIDByLabel map[string]domain.ApprovalID
}

func (s *runState) runAction(action Action) (ActionResult, error) {
	result := ActionResult{Type: action.Type, Label: action.Label}
	switch action.Type {
	case "approve":
		approval, err := s.approve(action)
		if err != nil {
			return result, err
		}
		result.Approval = &approval
		if action.Label != "" {
			s.approvals[action.Label] = approval
			s.approvalIDByLabel[action.Label] = approval.ID
		}
		return result, nil
	case "execute":
		receipt, err := s.execute(action)
		if err != nil {
			return result, err
		}
		result.Execution = &receipt
		return result, nil
	case "rotate":
		receipt, err := s.service.Rotate(action.Rotation)
		if err != nil {
			return result, err
		}
		result.Rotation = &receipt
		return result, nil
	case "advance":
		epoch := s.service.AdvanceEpoch(action.Delta)
		result.Epoch = &epoch
		return result, nil
	case "snapshot":
		snapshot := s.service.Snapshot()
		result.Snapshot = &snapshot
		return result, nil
	case "report":
		report := engine.BuildTreasuryReport(s.service.Snapshot())
		result.Report = &report
		return result, nil
	default:
		return result, domain.NewError(domain.CodeUnknownAction, "unknown action type "+action.Type)
	}
}

func (s *runState) approve(action Action) (domain.Approval, error) {
	request := domain.ApprovalRequest{
		Operation: action.Approval.Operation,
		SignerID:  action.Approval.SignerID,
		Decision:  action.Approval.Decision,
		Reason:    action.Approval.Reason,
	}
	if request.Operation.ID == "" {
		request.Operation = action.Operation
	}
	if request.SignerID == "" {
		return domain.Approval{}, domain.Invalid("approval signer id is required")
	}
	if request.Decision == "" {
		request.Decision = domain.ApprovalDecisionApprove
	}
	refs := append([]string(nil), action.Approval.ApprovalRefs...)
	refs = append(refs, action.ApprovalRefs...)
	request.Operation.ApprovalIDs = append(request.Operation.ApprovalIDs, s.resolveApprovalRefs(refs)...)
	return s.service.Approve(request)
}

func (s *runState) execute(action Action) (engine.ExecutionReceipt, error) {
	operation := action.Operation
	operation.ApprovalIDs = append(operation.ApprovalIDs, s.resolveApprovalRefs(action.ApprovalRefs)...)
	return s.service.Execute(operation)
}

func (s *runState) resolveApprovalRefs(refs []string) []domain.ApprovalID {
	ids := make([]domain.ApprovalID, 0, len(refs))
	for _, ref := range refs {
		if id, ok := s.approvalIDByLabel[ref]; ok {
			ids = append(ids, id)
			continue
		}
		ids = append(ids, domain.ApprovalID(ref))
	}
	return ids
}

func normalizeError(err error) (domain.Error, bool) {
	if err == nil {
		return domain.Error{}, false
	}
	if domainErr, ok := err.(domain.Error); ok {
		return domainErr, true
	}
	if domainErr, ok := err.(*domain.Error); ok {
		return *domainErr, true
	}
	return domain.Error{Code: domain.CodeInvalidRequest, Message: err.Error()}, true
}
