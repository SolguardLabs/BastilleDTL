package engine

import (
	"sort"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type TreasuryReport struct {
	Epoch             uint64                            `json:"epoch"`
	BalanceByAccount  map[domain.AccountID]domain.Money `json:"balanceByAccount"`
	WithdrawalTotal   domain.Money                      `json:"withdrawalTotal"`
	InternalTotal     domain.Money                      `json:"internalTotal"`
	ApprovalReuseHits int                               `json:"approvalReuseHits"`
	Executed          []domain.OperationID              `json:"executed"`
	Rejected          []domain.OperationID              `json:"rejected"`
}

func BuildTreasuryReport(snapshot domain.SystemSnapshot) TreasuryReport {
	report := TreasuryReport{
		Epoch:            snapshot.Epoch,
		BalanceByAccount: make(map[domain.AccountID]domain.Money),
		Executed:         make([]domain.OperationID, 0),
		Rejected:         make([]domain.OperationID, 0),
	}
	for _, balance := range snapshot.Balances {
		report.BalanceByAccount[balance.Account] += balance.Available + balance.Reserved
	}
	for _, operation := range snapshot.Operations {
		switch operation.Status {
		case domain.OperationExecuted:
			report.Executed = append(report.Executed, operation.Operation.ID)
			if operation.Operation.Kind == domain.OperationWithdrawal {
				report.WithdrawalTotal += operation.Operation.Amount
			}
			if operation.Operation.Kind == domain.OperationInternal {
				report.InternalTotal += operation.Operation.Amount
			}
		case domain.OperationRejected:
			report.Rejected = append(report.Rejected, operation.Operation.ID)
		}
	}
	for _, issue := range snapshot.AuditIssues {
		if issue.Code == "approval_operation_mismatch" || issue.Code == "approval_full_hash_mismatch" {
			report.ApprovalReuseHits++
		}
	}
	sort.Slice(report.Executed, func(i int, j int) bool { return report.Executed[i] < report.Executed[j] })
	sort.Slice(report.Rejected, func(i int, j int) bool { return report.Rejected[i] < report.Rejected[j] })
	return report
}
