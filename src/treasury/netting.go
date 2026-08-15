package treasury

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type Instruction struct {
	ID                  string
	Window              uint64
	Institution         domain.InstitutionID
	Kind                domain.OperationKind
	SourceAccount       domain.AccountID
	DestinationAccount  domain.AccountID
	ExternalBeneficiary string
	Rail                string
	Asset               domain.AssetID
	Amount              domain.Money
	Priority            uint8
	CutoffEpoch         uint64
}

type Limits struct {
	MaxInstructions      int
	MaxAccountsPerAsset  int
	ReserveBufferBps     uint16
	MaxRailShareBps      uint16
	MaxRailGross         map[string]domain.Money
	RailStressBps        map[string]uint16
	DefaultRailStressBps uint16
}

type AccountPosition struct {
	Account     domain.AccountID `json:"account"`
	Asset       domain.AssetID   `json:"asset"`
	GrossDebit  domain.Money     `json:"grossDebit"`
	GrossCredit domain.Money     `json:"grossCredit"`
	NetDebit    domain.Money     `json:"netDebit"`
	NetCredit   domain.Money     `json:"netCredit"`
}

type RailSummary struct {
	Asset          domain.AssetID `json:"asset"`
	Rail           string         `json:"rail"`
	Gross          domain.Money   `json:"gross"`
	ShareBps       uint16         `json:"shareBps"`
	StressBps      uint16         `json:"stressBps"`
	StressAddOn    domain.Money   `json:"stressAddOn"`
	ConfiguredCap  domain.Money   `json:"configuredCap"`
	RemainingCap   domain.Money   `json:"remainingCap"`
	WithinLimit    bool           `json:"withinLimit"`
	WithinShareCap bool           `json:"withinShareCap"`
}

type AssetSummary struct {
	Asset             domain.AssetID `json:"asset"`
	InternalGross     domain.Money   `json:"internalGross"`
	ExternalGross     domain.Money   `json:"externalGross"`
	NetCashOutflow    domain.Money   `json:"netCashOutflow"`
	PeakAccountDebit  domain.Money   `json:"peakAccountDebit"`
	ReserveBase       domain.Money   `json:"reserveBase"`
	ReserveRequired   domain.Money   `json:"reserveRequired"`
	StressedReserve   domain.Money   `json:"stressedReserve"`
	AvailableReserve  domain.Money   `json:"availableReserve"`
	ReserveShortfall  domain.Money   `json:"reserveShortfall"`
	RailHHIBps        uint16         `json:"railHhiBps"`
	AccountCount      int            `json:"accountCount"`
	ExternalRailCount int            `json:"externalRailCount"`
}

type Plan struct {
	Window           uint64            `json:"window"`
	InstructionCount int               `json:"instructionCount"`
	PositionCount    int               `json:"positionCount"`
	Digest           string            `json:"digest"`
	Ready            bool              `json:"ready"`
	Reasons          []string          `json:"reasons,omitempty"`
	Positions        []AccountPosition `json:"positions"`
	Rails            []RailSummary     `json:"rails"`
	Assets           []AssetSummary    `json:"assets"`
}

type Planner struct {
	limits Limits
}

type positionKey struct {
	account domain.AccountID
	asset   domain.AssetID
}

type railKey struct {
	asset domain.AssetID
	rail  string
}

func NewPlanner(limits Limits) (Planner, error) {
	limits = limits.normalize()
	if limits.MaxInstructions <= 0 || limits.MaxAccountsPerAsset < 2 {
		return Planner{}, domain.Invalid("treasury limits must permit a bilateral window")
	}
	if limits.ReserveBufferBps > 10_000 || limits.MaxRailShareBps > 10_000 || limits.DefaultRailStressBps > 10_000 {
		return Planner{}, domain.Invalid("treasury basis points exceed 10000")
	}
	for rail, shock := range limits.RailStressBps {
		if strings.TrimSpace(rail) == "" || shock > 10_000 {
			return Planner{}, domain.Invalid("treasury rail stress configuration is invalid")
		}
	}
	for rail, cap := range limits.MaxRailGross {
		if strings.TrimSpace(rail) == "" || cap <= 0 {
			return Planner{}, domain.Invalid("treasury rail cap configuration is invalid")
		}
	}
	return Planner{limits: limits}, nil
}

func (l Limits) normalize() Limits {
	if l.MaxInstructions == 0 {
		l.MaxInstructions = 512
	}
	if l.MaxAccountsPerAsset == 0 {
		l.MaxAccountsPerAsset = 128
	}
	if l.MaxRailShareBps == 0 {
		l.MaxRailShareBps = 10_000
	}
	if l.DefaultRailStressBps == 0 {
		l.DefaultRailStressBps = 500
	}
	if l.MaxRailGross == nil {
		l.MaxRailGross = map[string]domain.Money{}
	}
	if l.RailStressBps == nil {
		l.RailStressBps = map[string]uint16{}
	}
	return l
}

func (p Planner) Preview(window uint64, instructions []Instruction, reserves map[domain.AssetID]domain.Money) (Plan, error) {
	if window == 0 {
		return Plan{}, domain.Invalid("treasury window must be positive")
	}
	if len(instructions) == 0 || len(instructions) > p.limits.MaxInstructions {
		return Plan{}, domain.Limit("treasury instruction count is outside configured limits")
	}

	debits := map[positionKey]domain.Money{}
	credits := map[positionKey]domain.Money{}
	accounts := map[domain.AssetID]map[domain.AccountID]bool{}
	internalGross := map[domain.AssetID]domain.Money{}
	externalGross := map[domain.AssetID]domain.Money{}
	railGross := map[railKey]domain.Money{}
	seen := map[string]bool{}

	for _, instruction := range instructions {
		if err := instruction.validate(window); err != nil {
			return Plan{}, err
		}
		if seen[instruction.ID] {
			return Plan{}, domain.Invalidf("treasury instruction %s is duplicated", instruction.ID)
		}
		seen[instruction.ID] = true
		if accounts[instruction.Asset] == nil {
			accounts[instruction.Asset] = map[domain.AccountID]bool{}
		}
		accounts[instruction.Asset][instruction.SourceAccount] = true
		if err := addMoney(debits, positionKey{instruction.SourceAccount, instruction.Asset}, instruction.Amount); err != nil {
			return Plan{}, err
		}
		if instruction.Kind == domain.OperationInternal {
			accounts[instruction.Asset][instruction.DestinationAccount] = true
			if err := addMoney(credits, positionKey{instruction.DestinationAccount, instruction.Asset}, instruction.Amount); err != nil {
				return Plan{}, err
			}
			if err := addMoney(internalGross, instruction.Asset, instruction.Amount); err != nil {
				return Plan{}, err
			}
		} else {
			if err := addMoney(externalGross, instruction.Asset, instruction.Amount); err != nil {
				return Plan{}, err
			}
			key := railKey{instruction.Asset, strings.ToLower(strings.TrimSpace(instruction.Rail))}
			if err := addMoney(railGross, key, instruction.Amount); err != nil {
				return Plan{}, err
			}
		}
	}

	for asset, values := range accounts {
		if len(values) > p.limits.MaxAccountsPerAsset {
			return Plan{}, domain.Limitf("asset %s exceeds treasury account limit", asset)
		}
	}

	positions, err := buildPositions(debits, credits)
	if err != nil {
		return Plan{}, err
	}
	rails, railStress, railReasons, err := p.buildRails(railGross, externalGross)
	if err != nil {
		return Plan{}, err
	}
	assets, assetReasons, err := p.buildAssets(accounts, positions, internalGross, externalGross, railGross, railStress, reserves)
	if err != nil {
		return Plan{}, err
	}
	reasons := append(railReasons, assetReasons...)
	sort.Strings(reasons)
	plan := Plan{
		Window:           window,
		InstructionCount: len(instructions),
		PositionCount:    len(positions),
		Ready:            len(reasons) == 0,
		Reasons:          reasons,
		Positions:        positions,
		Rails:            rails,
		Assets:           assets,
	}
	plan.Digest = digestPlan(plan)
	return plan, nil
}

func (i Instruction) validate(window uint64) error {
	if err := domain.ValidateID("treasury instruction", i.ID); err != nil {
		return err
	}
	if i.Window != window {
		return domain.Invalidf("treasury instruction %s belongs to another window", i.ID)
	}
	if err := domain.ValidateID("institution", i.Institution.String()); err != nil {
		return err
	}
	if err := domain.ValidateID("source account", i.SourceAccount.String()); err != nil {
		return err
	}
	if err := domain.ValidateID("asset", i.Asset.String()); err != nil {
		return err
	}
	if err := domain.MustPositiveMoney("treasury amount", i.Amount); err != nil {
		return err
	}
	if i.Priority > 9 {
		return domain.Invalidf("treasury instruction %s priority exceeds 9", i.ID)
	}
	if i.CutoffEpoch > 0 && i.CutoffEpoch < i.Window {
		return domain.Invalidf("treasury instruction %s cutoff precedes its window", i.ID)
	}
	switch i.Kind {
	case domain.OperationInternal:
		if err := domain.ValidateID("destination account", i.DestinationAccount.String()); err != nil {
			return err
		}
		if i.SourceAccount == i.DestinationAccount {
			return domain.Invalidf("treasury instruction %s has identical accounts", i.ID)
		}
	case domain.OperationWithdrawal:
		if strings.TrimSpace(i.ExternalBeneficiary) == "" || strings.TrimSpace(i.Rail) == "" {
			return domain.Invalidf("treasury instruction %s requires beneficiary and rail", i.ID)
		}
	default:
		return domain.Invalidf("treasury instruction %s kind is unsupported", i.ID)
	}
	return nil
}

func buildPositions(debits, credits map[positionKey]domain.Money) ([]AccountPosition, error) {
	keys := map[positionKey]bool{}
	for key := range debits {
		keys[key] = true
	}
	for key := range credits {
		keys[key] = true
	}
	positions := make([]AccountPosition, 0, len(keys))
	for key := range keys {
		debit := debits[key]
		credit := credits[key]
		position := AccountPosition{Account: key.account, Asset: key.asset, GrossDebit: debit, GrossCredit: credit}
		if debit >= credit {
			value, err := debit.Sub(credit)
			if err != nil {
				return nil, err
			}
			position.NetDebit = value
		} else {
			value, err := credit.Sub(debit)
			if err != nil {
				return nil, err
			}
			position.NetCredit = value
		}
		positions = append(positions, position)
	}
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].Asset == positions[j].Asset {
			return positions[i].Account < positions[j].Account
		}
		return positions[i].Asset < positions[j].Asset
	})
	return positions, nil
}

func (p Planner) buildRails(gross map[railKey]domain.Money, external map[domain.AssetID]domain.Money) ([]RailSummary, map[domain.AssetID]domain.Money, []string, error) {
	keys := make([]railKey, 0, len(gross))
	for key := range gross {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].asset == keys[j].asset {
			return keys[i].rail < keys[j].rail
		}
		return keys[i].asset < keys[j].asset
	})
	rows := make([]RailSummary, 0, len(keys))
	stressByAsset := map[domain.AssetID]domain.Money{}
	reasons := make([]string, 0)
	for _, key := range keys {
		amount := gross[key]
		share := ratioBps(amount, external[key.asset])
		shock := p.limits.DefaultRailStressBps
		if configured, ok := p.limits.RailStressBps[key.rail]; ok {
			shock = configured
		}
		stress, err := scaleBps(amount, shock, true)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := addMoney(stressByAsset, key.asset, stress); err != nil {
			return nil, nil, nil, err
		}
		capValue, hasCap := p.limits.MaxRailGross[key.rail]
		withinLimit := !hasCap || amount <= capValue
		remaining := domain.Money(0)
		if hasCap && withinLimit {
			remaining, err = capValue.Sub(amount)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		withinShare := share <= p.limits.MaxRailShareBps
		if !withinLimit {
			reasons = append(reasons, fmt.Sprintf("rail %s gross cap exceeded for %s", key.rail, key.asset))
		}
		if !withinShare {
			reasons = append(reasons, fmt.Sprintf("rail %s concentration exceeded for %s", key.rail, key.asset))
		}
		rows = append(rows, RailSummary{
			Asset: key.asset, Rail: key.rail, Gross: amount, ShareBps: share, StressBps: shock,
			StressAddOn: stress, ConfiguredCap: capValue, RemainingCap: remaining,
			WithinLimit: withinLimit, WithinShareCap: withinShare,
		})
	}
	return rows, stressByAsset, reasons, nil
}

func (p Planner) buildAssets(accounts map[domain.AssetID]map[domain.AccountID]bool, positions []AccountPosition, internal, external map[domain.AssetID]domain.Money, railGross map[railKey]domain.Money, railStress, reserves map[domain.AssetID]domain.Money) ([]AssetSummary, []string, error) {
	assetSet := map[domain.AssetID]bool{}
	for asset := range internal {
		assetSet[asset] = true
	}
	for asset := range external {
		assetSet[asset] = true
	}
	assetIDs := make([]domain.AssetID, 0, len(assetSet))
	for asset := range assetSet {
		assetIDs = append(assetIDs, asset)
	}
	sort.Slice(assetIDs, func(i, j int) bool { return assetIDs[i] < assetIDs[j] })
	rows := make([]AssetSummary, 0, len(assetIDs))
	reasons := make([]string, 0)
	for _, asset := range assetIDs {
		peak := domain.Money(0)
		for _, position := range positions {
			if position.Asset == asset && position.NetDebit > peak {
				peak = position.NetDebit
			}
		}
		base := domain.MaxOfMoney(external[asset], peak)
		buffer, err := scaleBps(base, p.limits.ReserveBufferBps, true)
		if err != nil {
			return nil, nil, err
		}
		required, err := base.Add(buffer)
		if err != nil {
			return nil, nil, err
		}
		stressed, err := required.Add(railStress[asset])
		if err != nil {
			return nil, nil, err
		}
		available := reserves[asset]
		if available < 0 {
			return nil, nil, domain.Invalidf("asset %s reserve is negative", asset)
		}
		shortfall := domain.Money(0)
		if stressed > available {
			shortfall, err = stressed.Sub(available)
			if err != nil {
				return nil, nil, err
			}
			reasons = append(reasons, fmt.Sprintf("asset %s reserve shortfall is %s", asset, shortfall))
		}
		rails := make([]domain.Money, 0)
		for key, value := range railGross {
			if key.asset == asset {
				rails = append(rails, value)
			}
		}
		rows = append(rows, AssetSummary{
			Asset: asset, InternalGross: internal[asset], ExternalGross: external[asset],
			NetCashOutflow: external[asset], PeakAccountDebit: peak, ReserveBase: base,
			ReserveRequired: required, StressedReserve: stressed, AvailableReserve: available,
			ReserveShortfall: shortfall, RailHHIBps: concentrationHHI(rails),
			AccountCount: len(accounts[asset]), ExternalRailCount: len(rails),
		})
	}
	return rows, reasons, nil
}

func addMoney[Key comparable](values map[Key]domain.Money, key Key, amount domain.Money) error {
	next, err := values[key].Add(amount)
	if err != nil {
		return err
	}
	values[key] = next
	return nil
}

func scaleBps(value domain.Money, bps uint16, roundUp bool) (domain.Money, error) {
	if value < 0 || bps > 10_000 {
		return 0, domain.Invalid("basis point calculation received invalid input")
	}
	numerator := new(big.Int).Mul(big.NewInt(value.Int64()), big.NewInt(int64(bps)))
	if roundUp && numerator.Sign() > 0 {
		numerator.Add(numerator, big.NewInt(9_999))
	}
	result := numerator.Quo(numerator, big.NewInt(10_000))
	if !result.IsInt64() || result.Int64() > domain.MaxMoney.Int64() {
		return 0, domain.Invalid("basis point calculation overflow")
	}
	return domain.Money(result.Int64()), nil
}

func ratioBps(part, total domain.Money) uint16 {
	if part <= 0 || total <= 0 {
		return 0
	}
	numerator := new(big.Int).Mul(big.NewInt(part.Int64()), big.NewInt(10_000))
	value := numerator.Quo(numerator, big.NewInt(total.Int64())).Int64()
	if value > 10_000 {
		value = 10_000
	}
	return uint16(value)
}

func concentrationHHI(values []domain.Money) uint16 {
	total := big.NewInt(0)
	squares := big.NewInt(0)
	for _, value := range values {
		amount := big.NewInt(value.Int64())
		total.Add(total, amount)
		squares.Add(squares, new(big.Int).Mul(amount, amount))
	}
	if total.Sign() == 0 {
		return 0
	}
	numerator := new(big.Int).Mul(squares, big.NewInt(10_000))
	denominator := new(big.Int).Mul(total, total)
	value := numerator.Quo(numerator, denominator).Int64()
	if value > 10_000 {
		value = 10_000
	}
	return uint16(value)
}

func digestPlan(plan Plan) string {
	var payload strings.Builder
	fmt.Fprintf(&payload, "bastille-treasury-plan-v1|window=%d|instructions=%d|positions=%d", plan.Window, plan.InstructionCount, plan.PositionCount)
	for _, position := range plan.Positions {
		fmt.Fprintf(&payload, "|p=%s,%s,%d,%d,%d,%d", position.Account, position.Asset, position.GrossDebit, position.GrossCredit, position.NetDebit, position.NetCredit)
	}
	for _, rail := range plan.Rails {
		fmt.Fprintf(&payload, "|r=%s,%s,%d,%d,%d,%d,%d,%t,%t", rail.Asset, rail.Rail, rail.Gross, rail.ShareBps, rail.StressBps, rail.StressAddOn, rail.ConfiguredCap, rail.WithinLimit, rail.WithinShareCap)
	}
	for _, asset := range plan.Assets {
		fmt.Fprintf(&payload, "|a=%s,%d,%d,%d,%d,%d,%d,%d,%d,%d", asset.Asset, asset.InternalGross, asset.ExternalGross, asset.PeakAccountDebit, asset.ReserveRequired, asset.StressedReserve, asset.AvailableReserve, asset.ReserveShortfall, asset.RailHHIBps, asset.AccountCount)
	}
	sum := sha256.Sum256([]byte(payload.String()))
	return hex.EncodeToString(sum[:])
}

func (p Plan) Asset(asset domain.AssetID) (AssetSummary, bool) {
	for _, summary := range p.Assets {
		if summary.Asset == asset {
			return summary, true
		}
	}
	return AssetSummary{}, false
}
