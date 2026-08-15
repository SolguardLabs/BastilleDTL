package treasury

import (
	"testing"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

func internal(id string, window uint64, source, destination domain.AccountID, asset domain.AssetID, amount domain.Money) Instruction {
	return Instruction{
		ID: id, Window: window, Institution: "inst:alpha", Kind: domain.OperationInternal,
		SourceAccount: source, DestinationAccount: destination, Asset: asset, Amount: amount,
		Priority: 3, CutoffEpoch: window + 1,
	}
}

func withdrawal(id string, window uint64, source domain.AccountID, asset domain.AssetID, rail string, amount domain.Money) Instruction {
	return Instruction{
		ID: id, Window: window, Institution: "inst:alpha", Kind: domain.OperationWithdrawal,
		SourceAccount: source, ExternalBeneficiary: "beneficiary:" + id, Rail: rail,
		Asset: asset, Amount: amount, Priority: 5, CutoffEpoch: window + 1,
	}
}

func TestPlannerCombinesIntradayNettingBufferAndRailStress(t *testing.T) {
	planner, err := NewPlanner(Limits{
		ReserveBufferBps: 1_000,
		RailStressBps:    map[string]uint16{"swift": 2_000},
		MaxRailGross:     map[string]domain.Money{"swift": 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planner.Preview(80, []Instruction{
		internal("i-1", 80, "acct:operating", "acct:reserve", "usdc", 100),
		internal("i-2", 80, "acct:reserve", "acct:operating", "usdc", 40),
		withdrawal("i-3", 80, "acct:operating", "usdc", "swift", 60),
	}, map[domain.AssetID]domain.Money{"usdc": 143})
	if err != nil {
		t.Fatal(err)
	}
	summary, ok := plan.Asset("usdc")
	if !ok {
		t.Fatal("missing usdc summary")
	}
	if summary.InternalGross != 140 || summary.ExternalGross != 60 || summary.PeakAccountDebit != 120 {
		t.Fatalf("unexpected gross or peak values: %+v", summary)
	}
	if summary.ReserveRequired != 132 || summary.StressedReserve != 144 || summary.ReserveShortfall != 1 {
		t.Fatalf("unexpected reserve values: %+v", summary)
	}
	if summary.RailHHIBps != 10_000 || plan.Ready {
		t.Fatalf("unexpected concentration or readiness: %+v", plan)
	}
	if len(plan.Digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(plan.Digest))
	}
}

func TestPlannerComputesRailSharesAndHHI(t *testing.T) {
	planner, err := NewPlanner(Limits{ReserveBufferBps: 500, RailStressBps: map[string]uint16{"swift": 1_000, "sepa": 500}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planner.Preview(81, []Instruction{
		withdrawal("w-1", 81, "acct:a", "usdc", "swift", 60),
		withdrawal("w-2", 81, "acct:b", "usdc", "sepa", 40),
	}, map[domain.AssetID]domain.Money{"usdc": 200})
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := plan.Asset("usdc")
	if summary.RailHHIBps != 5_200 || summary.ExternalRailCount != 2 {
		t.Fatalf("unexpected rail metrics: %+v", summary)
	}
	if plan.Rails[0].ShareBps != 4_000 || plan.Rails[1].ShareBps != 6_000 {
		t.Fatalf("unexpected rail shares: %+v", plan.Rails)
	}
	if !plan.Ready || len(plan.Reasons) != 0 {
		t.Fatalf("expected funded plan: %+v", plan)
	}
}

func TestPlannerKeepsAssetsAndPositionsSeparate(t *testing.T) {
	planner, _ := NewPlanner(Limits{})
	plan, err := planner.Preview(82, []Instruction{
		withdrawal("u-1", 82, "acct:a", "usdc", "swift", 25),
		withdrawal("e-1", 82, "acct:a", "eurc", "sepa", 40),
	}, map[domain.AssetID]domain.Money{"usdc": 100, "eurc": 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Assets) != 2 || len(plan.Positions) != 2 {
		t.Fatalf("unexpected plan cardinality: %+v", plan)
	}
	usdc, _ := plan.Asset("usdc")
	eurc, _ := plan.Asset("eurc")
	if usdc.ExternalGross != 25 || eurc.ExternalGross != 40 {
		t.Fatalf("asset values crossed domains: usdc=%+v eurc=%+v", usdc, eurc)
	}
}

func TestPlannerRejectsDuplicateIDsAndWrongWindows(t *testing.T) {
	planner, _ := NewPlanner(Limits{})
	duplicate := []Instruction{
		withdrawal("same", 83, "acct:a", "usdc", "swift", 10),
		withdrawal("same", 83, "acct:b", "usdc", "sepa", 10),
	}
	if _, err := planner.Preview(83, duplicate, nil); err == nil {
		t.Fatal("expected duplicate instruction rejection")
	}
	if _, err := planner.Preview(83, []Instruction{withdrawal("wrong", 84, "acct:a", "usdc", "swift", 10)}, nil); err == nil {
		t.Fatal("expected window rejection")
	}
}

func TestPlannerSignalsRailCapAndConcentration(t *testing.T) {
	planner, err := NewPlanner(Limits{
		MaxRailShareBps: 6_000,
		MaxRailGross:    map[string]domain.Money{"swift": 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planner.Preview(84, []Instruction{
		withdrawal("s-1", 84, "acct:a", "usdc", "swift", 70),
		withdrawal("a-1", 84, "acct:b", "usdc", "ach", 30),
	}, map[domain.AssetID]domain.Money{"usdc": 200})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ready || len(plan.Reasons) != 2 {
		t.Fatalf("expected two policy signals: %+v", plan.Reasons)
	}
	if plan.Rails[1].WithinLimit || plan.Rails[1].WithinShareCap {
		t.Fatalf("swift limits should be exceeded: %+v", plan.Rails[1])
	}
}

func TestPlannerDigestIsIndependentOfInputOrder(t *testing.T) {
	planner, _ := NewPlanner(Limits{})
	first := withdrawal("z", 85, "acct:a", "usdc", "swift", 20)
	second := internal("a", 85, "acct:a", "acct:b", "usdc", 5)
	reserves := map[domain.AssetID]domain.Money{"usdc": 100}
	left, err := planner.Preview(85, []Instruction{first, second}, reserves)
	if err != nil {
		t.Fatal(err)
	}
	right, err := planner.Preview(85, []Instruction{second, first}, reserves)
	if err != nil {
		t.Fatal(err)
	}
	if left.Digest != right.Digest {
		t.Fatalf("digest changed with input order: %s != %s", left.Digest, right.Digest)
	}
}
