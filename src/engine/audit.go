package engine

import "github.com/solguardlabs/bastilledtl/src/domain"

func EvaluateApprovalReuse(records []domain.OperationRecord, approvals []domain.Approval) []domain.AuditIssue {
	byID := make(map[domain.ApprovalID]domain.Approval, len(approvals))
	for _, approval := range approvals {
		byID[approval.ID] = approval
	}
	issues := make([]domain.AuditIssue, 0)
	for _, record := range records {
		if record.Status != domain.OperationExecuted {
			continue
		}
		for _, id := range record.ApprovalIDs {
			approval, ok := byID[id]
			if !ok {
				continue
			}
			if approval.OperationID != record.Operation.ID {
				issues = append(issues, domain.AuditIssue{
					Code:      "approval_operation_mismatch",
					Severity:  "critical",
					Message:   "approval was used by an operation different from the operation it was issued for",
					Operation: record.Operation.ID,
					Approval:  approval.ID,
					Fields: map[string]string{
						"issuedFor": approval.OperationID.String(),
						"usedFor":   record.Operation.ID.String(),
					},
				})
				continue
			}
			if approval.FullHash != record.FullHash {
				issues = append(issues, domain.AuditIssue{
					Code:      "approval_full_hash_mismatch",
					Severity:  "critical",
					Message:   "approval partial hash matched but full operation hash differed",
					Operation: record.Operation.ID,
					Approval:  approval.ID,
					Fields: map[string]string{
						"approvalHash":  approval.FullHash,
						"operationHash": record.FullHash,
					},
				})
			}
		}
	}
	return issues
}

func CountAuditIssues(issues []domain.AuditIssue, code string) int {
	count := 0
	for _, issue := range issues {
		if issue.Code == code {
			count++
		}
	}
	return count
}
