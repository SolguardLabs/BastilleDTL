package treasury

import (
	"testing"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

func TestReserveGateTracksHomogeneousRailWithinEpoch(t *testing.T) {
	gate := NewReserveGate(domain.DefaultPolicy())
	first := domain.Operation{
		ID: "op:reserve:first", Institution: "inst:alpha", Kind: domain.OperationWithdrawal,
		SourceAccount: "acct:alpha:operating", ExternalBeneficiary: "beneficiary:first",
		ExternalRail: "swift", Asset: "usdc", Amount: 300_000, RequestedBy: "user:ops",
	}
	assessment, err := gate.Preview(first, 7, 100_000)
	if err != nil || assessment.RequiredReserve != 90_000 || !assessment.Accepted {
		t.Fatalf("first assessment=%+v err=%v", assessment, err)
	}
	gate.Commit(assessment)
	second := first
	second.ID = "op:reserve:second"
	second.Amount = 50_000
	if assessment, err := gate.Preview(second, 7, 100_000); err == nil || assessment.RequiredReserve != 105_000 {
		t.Fatalf("second assessment=%+v err=%v", assessment, err)
	}
	if assessment, err := gate.Preview(second, 8, 100_000); err != nil || assessment.RequiredReserve != 15_000 {
		t.Fatalf("new epoch assessment=%+v err=%v", assessment, err)
	}
}
