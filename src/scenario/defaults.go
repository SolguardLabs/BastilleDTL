package scenario

import (
	"github.com/solguardlabs/bastilledtl/src/domain"
	"github.com/solguardlabs/bastilledtl/src/engine"
)

func DefaultBootstrap() engine.Bootstrap {
	policy := domain.DefaultPolicy()
	return engine.Bootstrap{
		Institutions: []domain.Institution{
			{
				ID:               "inst:alpha",
				Name:             "Alpha Custody Group",
				Enabled:          true,
				HomeJurisdiction: "EU",
				DefaultAsset:     "usdc",
			},
		},
		Assets: []domain.Asset{
			{ID: "usdc", Symbol: "USDC", Decimals: 6, Stable: true, Enabled: true, Network: "solana", RiskClass: "stable"},
			{ID: "eurc", Symbol: "EURC", Decimals: 6, Stable: true, Enabled: true, Network: "solana", RiskClass: "stable"},
		},
		Accounts: []domain.Account{
			{
				ID:            "acct:alpha:operating",
				Institution:   "inst:alpha",
				Kind:          domain.AccountOperating,
				Status:        domain.AccountActive,
				AllowedAssets: []domain.AssetID{"usdc", "eurc"},
				Roles:         []domain.Role{domain.RoleInternal, domain.RoleWithdrawal},
				Owner:         "treasury operations",
				ExposureClass: "primary",
			},
			{
				ID:            "acct:alpha:reserve",
				Institution:   "inst:alpha",
				Kind:          domain.AccountReserve,
				Status:        domain.AccountActive,
				AllowedAssets: []domain.AssetID{"usdc", "eurc"},
				Roles:         []domain.Role{domain.RoleInternal},
				Owner:         "reserve desk",
				ExposureClass: "reserve",
			},
			{
				ID:            "acct:alpha:settlement",
				Institution:   "inst:alpha",
				Kind:          domain.AccountSettlement,
				Status:        domain.AccountActive,
				AllowedAssets: []domain.AssetID{"usdc"},
				Roles:         []domain.Role{domain.RoleWithdrawal},
				Owner:         "external settlement",
				ExposureClass: "settlement",
			},
			{
				ID:            "acct:alpha:blocked",
				Institution:   "inst:alpha",
				Kind:          domain.AccountReserve,
				Status:        domain.AccountFrozen,
				AllowedAssets: []domain.AssetID{"usdc"},
				Roles:         []domain.Role{domain.RoleInternal},
				Owner:         "frozen reserve",
				ExposureClass: "blocked",
			},
		},
		Principals: []domain.Principal{
			{ID: "user:ops", Institution: "inst:alpha", DisplayName: "Ops Desk", Roles: []domain.Role{domain.RoleOperator, domain.RoleInternal, domain.RoleWithdrawal}, Enabled: true},
			{ID: "user:risk", Institution: "inst:alpha", DisplayName: "Risk Manager", Roles: []domain.Role{domain.RoleRisk, domain.RoleAdmin}, Enabled: true},
			{ID: "user:audit", Institution: "inst:alpha", DisplayName: "Read Only Audit", Roles: []domain.Role{domain.RoleAuditor}, Enabled: true},
			{ID: "user:treasury", Institution: "inst:alpha", DisplayName: "Treasury", Roles: []domain.Role{domain.RoleTreasurer, domain.RoleApprover}, Enabled: true},
		},
		Signers: []domain.Signer{
			{ID: "signer:alice", Institution: "inst:alpha", Principal: "user:treasury", Role: domain.RoleTreasurer, Status: domain.SignerActive, Weight: 1, MaxAmount: 900000, ActivatedEpoch: 0, RotationGroup: "A"},
			{ID: "signer:bob", Institution: "inst:alpha", Principal: "user:risk", Role: domain.RoleRisk, Status: domain.SignerActive, Weight: 1, MaxAmount: 900000, ActivatedEpoch: 0, RotationGroup: "B"},
			{ID: "signer:carol", Institution: "inst:alpha", Principal: "user:audit", Role: domain.RoleAuditor, Status: domain.SignerActive, Weight: 1, MaxAmount: 900000, ActivatedEpoch: 0, RotationGroup: "C"},
		},
		Balances: []domain.BalanceSeed{
			{Account: "acct:alpha:operating", Asset: "usdc", Available: 1000000},
			{Account: "acct:alpha:reserve", Asset: "usdc", Available: 100000},
			{Account: "acct:alpha:settlement", Asset: "usdc", Available: 0},
			{Account: "acct:alpha:operating", Asset: "eurc", Available: 250000},
			{Account: "acct:alpha:reserve", Asset: "eurc", Available: 0},
		},
		ExposureCaps: []domain.ExposureCap{
			{ID: "cap:alpha:usdc:withdrawal", Institution: "inst:alpha", Account: "acct:alpha:operating", Asset: "usdc", Kind: domain.OperationWithdrawal, Limit: policy.WithdrawalDailyCap, Window: 24, Enabled: true},
			{ID: "cap:alpha:usdc:internal", Institution: "inst:alpha", Account: "acct:alpha:operating", Asset: "usdc", Kind: domain.OperationInternal, Limit: policy.InternalDailyCap, Window: 24, Enabled: true},
			{ID: "cap:alpha:eurc:internal", Institution: "inst:alpha", Account: "acct:alpha:operating", Asset: "eurc", Kind: domain.OperationInternal, Limit: 500000, Window: 24, Enabled: true},
		},
		Policy: policy,
	}
}

func InternalOperation(id domain.OperationID, amount domain.Money) domain.Operation {
	return domain.Operation{
		ID:                 id,
		Institution:        "inst:alpha",
		Kind:               domain.OperationInternal,
		SourceAccount:      "acct:alpha:operating",
		DestinationAccount: "acct:alpha:reserve",
		Asset:              "usdc",
		Amount:             amount,
		RequestedBy:        "user:ops",
		Memo:               "treasury rebalance",
	}
}

func WithdrawalOperation(id domain.OperationID, amount domain.Money, beneficiary string) domain.Operation {
	return domain.Operation{
		ID:                  id,
		Institution:         "inst:alpha",
		Kind:                domain.OperationWithdrawal,
		SourceAccount:       "acct:alpha:operating",
		ExternalBeneficiary: beneficiary,
		ExternalRail:        "swift",
		Asset:               "usdc",
		Amount:              amount,
		RequestedBy:         "user:ops",
		Memo:                "external treasury payout",
	}
}
