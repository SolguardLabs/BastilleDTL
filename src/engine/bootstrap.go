package engine

import (
	"sort"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type Bootstrap struct {
	Institutions []domain.Institution `json:"institutions"`
	Assets       []domain.Asset       `json:"assets"`
	Accounts     []domain.Account     `json:"accounts"`
	Principals   []domain.Principal   `json:"principals"`
	Signers      []domain.Signer      `json:"signers"`
	Balances     []domain.BalanceSeed `json:"balances"`
	ExposureCaps []domain.ExposureCap `json:"exposureCaps"`
	Policy       domain.EnginePolicy  `json:"policy"`
}

func (b Bootstrap) Normalize() Bootstrap {
	b.Policy = b.Policy.Normalize()
	return b
}

func (b Bootstrap) Validate() error {
	b = b.Normalize()
	if err := b.Policy.Validate(); err != nil {
		return err
	}
	if len(b.Institutions) == 0 {
		return domain.Invalid("at least one institution is required")
	}
	if len(b.Assets) == 0 {
		return domain.Invalid("at least one asset is required")
	}
	if len(b.Accounts) == 0 {
		return domain.Invalid("at least one account is required")
	}
	if len(b.Principals) == 0 {
		return domain.Invalid("at least one principal is required")
	}
	if len(b.Signers) == 0 {
		return domain.Invalid("at least one signer is required")
	}
	seenInstitutions := map[domain.InstitutionID]bool{}
	for _, institution := range b.Institutions {
		if err := institution.Validate(); err != nil {
			return err
		}
		if seenInstitutions[institution.ID] {
			return domain.Invalidf("duplicate institution %s", institution.ID)
		}
		seenInstitutions[institution.ID] = true
	}
	seenAssets := map[domain.AssetID]bool{}
	for _, asset := range b.Assets {
		if err := asset.Validate(); err != nil {
			return err
		}
		if seenAssets[asset.ID] {
			return domain.Invalidf("duplicate asset %s", asset.ID)
		}
		seenAssets[asset.ID] = true
	}
	seenAccounts := map[domain.AccountID]bool{}
	for _, account := range b.Accounts {
		if err := account.Validate(); err != nil {
			return err
		}
		if !seenInstitutions[account.Institution] {
			return domain.Invalidf("account %s references unknown institution %s", account.ID, account.Institution)
		}
		for _, asset := range account.AllowedAssets {
			if !seenAssets[asset] {
				return domain.Invalidf("account %s references unknown asset %s", account.ID, asset)
			}
		}
		if seenAccounts[account.ID] {
			return domain.Invalidf("duplicate account %s", account.ID)
		}
		seenAccounts[account.ID] = true
	}
	seenPrincipals := map[domain.PrincipalID]bool{}
	for _, principal := range b.Principals {
		if err := principal.Validate(); err != nil {
			return err
		}
		if !seenInstitutions[principal.Institution] {
			return domain.Invalidf("principal %s references unknown institution %s", principal.ID, principal.Institution)
		}
		if seenPrincipals[principal.ID] {
			return domain.Invalidf("duplicate principal %s", principal.ID)
		}
		seenPrincipals[principal.ID] = true
	}
	seenSigners := map[domain.SignerID]bool{}
	for _, signer := range b.Signers {
		if err := signer.Validate(); err != nil {
			return err
		}
		if !seenInstitutions[signer.Institution] {
			return domain.Invalidf("signer %s references unknown institution %s", signer.ID, signer.Institution)
		}
		if !seenPrincipals[signer.Principal] {
			return domain.Invalidf("signer %s references unknown principal %s", signer.ID, signer.Principal)
		}
		if seenSigners[signer.ID] {
			return domain.Invalidf("duplicate signer %s", signer.ID)
		}
		seenSigners[signer.ID] = true
	}
	for _, balance := range b.Balances {
		if err := balance.Validate(); err != nil {
			return err
		}
		if !seenAccounts[balance.Account] {
			return domain.Invalidf("balance references unknown account %s", balance.Account)
		}
		if !seenAssets[balance.Asset] {
			return domain.Invalidf("balance references unknown asset %s", balance.Asset)
		}
	}
	for _, cap := range b.ExposureCaps {
		if err := cap.Validate(); err != nil {
			return err
		}
		if !seenInstitutions[cap.Institution] {
			return domain.Invalidf("cap %s references unknown institution %s", cap.ID, cap.Institution)
		}
		if cap.Account != "" && !seenAccounts[cap.Account] {
			return domain.Invalidf("cap %s references unknown account %s", cap.ID, cap.Account)
		}
		if !seenAssets[cap.Asset] {
			return domain.Invalidf("cap %s references unknown asset %s", cap.ID, cap.Asset)
		}
	}
	return nil
}

func sortedInstitutions(values map[domain.InstitutionID]domain.Institution) []domain.Institution {
	out := make([]domain.Institution, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedAssets(values map[domain.AssetID]domain.Asset) []domain.Asset {
	out := make([]domain.Asset, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedAccounts(values map[domain.AccountID]domain.Account) []domain.Account {
	out := make([]domain.Account, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedPrincipals(values map[domain.PrincipalID]domain.Principal) []domain.Principal {
	out := make([]domain.Principal, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].ID < out[j].ID })
	return out
}
