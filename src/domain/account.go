package domain

type AccountKind string

const (
	AccountOperating  AccountKind = "operating"
	AccountReserve    AccountKind = "reserve"
	AccountSettlement AccountKind = "settlement"
	AccountExternal   AccountKind = "external"
	AccountSuspense   AccountKind = "suspense"
)

type AccountStatus string

const (
	AccountActive   AccountStatus = "active"
	AccountFrozen   AccountStatus = "frozen"
	AccountClosed   AccountStatus = "closed"
	AccountWatch    AccountStatus = "watch"
	AccountDisabled AccountStatus = "disabled"
)

type Account struct {
	ID            AccountID     `json:"id"`
	Institution   InstitutionID `json:"institution"`
	Kind          AccountKind   `json:"kind"`
	Status        AccountStatus `json:"status"`
	AllowedAssets []AssetID     `json:"allowedAssets"`
	Roles         []Role        `json:"roles,omitempty"`
	Owner         string        `json:"owner,omitempty"`
	ExposureClass string        `json:"exposureClass,omitempty"`
	ExternalRail  string        `json:"externalRail,omitempty"`
	CreatedEpoch  uint64        `json:"createdEpoch,omitempty"`
	ClosedEpoch   uint64        `json:"closedEpoch,omitempty"`
}

func (a Account) Validate() error {
	if err := ValidateID("account", a.ID.String()); err != nil {
		return err
	}
	if err := ValidateID("institution", a.Institution.String()); err != nil {
		return err
	}
	if a.Kind == "" {
		return Invalidf("account %s kind is required", a.ID)
	}
	if a.Status == "" {
		return Invalidf("account %s status is required", a.ID)
	}
	if len(a.AllowedAssets) == 0 {
		return Invalidf("account %s requires at least one allowed asset", a.ID)
	}
	return nil
}

func (a Account) Enabled() bool {
	return a.Status == AccountActive || a.Status == AccountWatch
}

func (a Account) CanDebit(asset AssetID) bool {
	if !a.Enabled() {
		return false
	}
	return a.AllowsAsset(asset)
}

func (a Account) CanCredit(asset AssetID) bool {
	if a.Status == AccountClosed || a.Status == AccountDisabled {
		return false
	}
	return a.AllowsAsset(asset)
}

func (a Account) AllowsAsset(asset AssetID) bool {
	for _, allowed := range a.AllowedAssets {
		if allowed == asset {
			return true
		}
	}
	return false
}

func (a Account) HasRole(role Role) bool {
	return RoleSetFromSlice(a.Roles).Has(role)
}

func (a Account) IsInternalTo(institution InstitutionID) bool {
	return a.Institution == institution && a.Kind != AccountExternal
}

type Institution struct {
	ID               InstitutionID `json:"id"`
	Name             string        `json:"name"`
	Enabled          bool          `json:"enabled"`
	HomeJurisdiction string        `json:"homeJurisdiction,omitempty"`
	DefaultAsset     AssetID       `json:"defaultAsset,omitempty"`
	CreatedEpoch     uint64        `json:"createdEpoch,omitempty"`
}

func (i Institution) Validate() error {
	if err := ValidateID("institution", i.ID.String()); err != nil {
		return err
	}
	if i.Name == "" {
		return Invalidf("institution %s name is required", i.ID)
	}
	return nil
}
