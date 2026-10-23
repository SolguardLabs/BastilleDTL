package engine_test

import (
	"testing"

	"github.com/solguardlabs/bastilledtl/src/domain"
	"github.com/solguardlabs/bastilledtl/src/engine"
	"github.com/solguardlabs/bastilledtl/src/scenario"
)

func TestWithdrawalReserveStressPrecedesLedger(t *testing.T) {
	s, err := engine.NewService(scenario.DefaultBootstrap())
	if err != nil {
		t.Fatal(err)
	}
	large := scenario.WithdrawalOperation("op:stress:large", 400_000, "beneficiary:clearing")
	for _, signer := range []domain.SignerID{"signer:alice", "signer:bob"} {
		approval, err := s.Approve(domain.ApprovalRequest{Operation: large, SignerID: signer, Decision: domain.ApprovalDecisionApprove})
		if err != nil {
			t.Fatal(err)
		}
		large.ApprovalIDs = append(large.ApprovalIDs, approval.ID)
	}
	if _, err := s.Execute(large); err == nil {
		t.Fatal("expected reserve policy rejection")
	}
	if balance := s.Snapshot().Balance("acct:alpha:operating", "usdc"); balance.Available != 1_000_000 {
		t.Fatalf("rejected withdrawal changed source balance: %+v", balance)
	}
	small := scenario.WithdrawalOperation("op:stress:small", 1_000, "beneficiary:clearing")
	small.ExternalRail = "sepa"
	receipt, err := s.Execute(small)
	if err != nil || receipt.ReserveStress == nil || receipt.ReserveStress.RequiredReserve != 10 {
		t.Fatalf("small withdrawal=%+v err=%v", receipt, err)
	}
	if report := engine.BuildTreasuryReport(s.Snapshot()); report.WithdrawalTotal != 1_000 {
		t.Fatalf("withdrawal total=%d", report.WithdrawalTotal)
	}
	if _, err := s.Execute(small); err == nil {
		t.Fatal("repeated operation ID should not post another entry")
	}
	if report := engine.BuildTreasuryReport(s.Snapshot()); report.WithdrawalTotal != 1_000 {
		t.Fatalf("repeated operation changed withdrawal total=%d", report.WithdrawalTotal)
	}
}

func TestApprovalBundleIsScopedToExecutedOperation(t *testing.T) {
	s, err := engine.NewService(scenario.DefaultBootstrap())
	if err != nil {
		t.Fatal(err)
	}
	first := scenario.InternalOperation("op:scope:first", 120_000)
	for _, signer := range []domain.SignerID{"signer:alice", "signer:bob"} {
		approval, err := s.Approve(domain.ApprovalRequest{Operation: first, SignerID: signer, Decision: domain.ApprovalDecisionApprove})
		if err != nil {
			t.Fatal(err)
		}
		first.ApprovalIDs = append(first.ApprovalIDs, approval.ID)
	}
	if _, err := s.Execute(first); err != nil {
		t.Fatal(err)
	}
	second := scenario.InternalOperation("op:scope:second", 120_000)
	second.ApprovalIDs = first.ApprovalIDs
	if _, err := s.Execute(second); err == nil {
		t.Fatal("expected operation-scoped approval rejection")
	}
}
