package treasury

import (
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type ReserveAssessment struct {
	Institution      domain.InstitutionID `json:"institution"`
	Asset            domain.AssetID       `json:"asset"`
	Epoch            uint64               `json:"epoch"`
	Gross            domain.Money         `json:"gross"`
	RequiredReserve  domain.Money         `json:"requiredReserve"`
	AvailableReserve domain.Money         `json:"availableReserve"`
	Accepted         bool                 `json:"accepted"`
	Rails            []RailExposure       `json:"rails"`
}

type RailExposure struct {
	Rail      string       `json:"rail"`
	Gross     domain.Money `json:"gross"`
	StressBps uint16       `json:"stressBps"`
}

type reserveKey struct {
	institution domain.InstitutionID
	asset       domain.AssetID
}

type ReserveGate struct {
	mu     sync.Mutex
	epoch  uint64
	flows  map[reserveKey]map[string]domain.Money
	stress map[string]uint16
}

func NewReserveGate(policy domain.EnginePolicy) *ReserveGate {
	stress := make(map[string]uint16, len(policy.RailStressBps))
	for rail, bps := range policy.RailStressBps {
		stress[strings.ToLower(strings.TrimSpace(rail))] = bps
	}
	return &ReserveGate{flows: make(map[reserveKey]map[string]domain.Money), stress: stress}
}

func (g *ReserveGate) Preview(operation domain.Operation, epoch uint64, available domain.Money) (ReserveAssessment, error) {
	if operation.Kind != domain.OperationWithdrawal {
		return ReserveAssessment{}, domain.Invalid("reserve preview requires a withdrawal")
	}
	if available < 0 {
		return ReserveAssessment{}, domain.Invalid("reserve liquidity is negative")
	}
	key := reserveKey{institution: operation.Institution, asset: operation.Asset}
	rail := strings.ToLower(strings.TrimSpace(operation.ExternalRail))
	g.mu.Lock()
	projected := make(map[string]domain.Money)
	if g.epoch == epoch {
		for name, gross := range g.flows[key] {
			projected[name] = gross
		}
	}
	g.mu.Unlock()
	next, err := projected[rail].Add(operation.Amount)
	if err != nil {
		return ReserveAssessment{}, err
	}
	projected[rail] = next
	assessment := ReserveAssessment{
		Institution: operation.Institution, Asset: operation.Asset, Epoch: epoch,
		AvailableReserve: available, Rails: make([]RailExposure, 0, len(projected)),
	}
	var rateTotal int64
	for name, gross := range projected {
		bps, ok := g.stress[name]
		if !ok {
			return ReserveAssessment{}, domain.Invalidf("rail %s has no stress configuration", name)
		}
		assessment.Gross, err = assessment.Gross.Add(gross)
		if err != nil {
			return ReserveAssessment{}, err
		}
		rateTotal += int64(bps)
		assessment.Rails = append(assessment.Rails, RailExposure{Rail: name, Gross: gross, StressBps: bps})
	}
	sort.Slice(assessment.Rails, func(i, j int) bool { return assessment.Rails[i].Rail < assessment.Rails[j].Rail })
	denominator := big.NewInt(int64(len(assessment.Rails)) * 10_000)
	numerator := new(big.Int).Mul(big.NewInt(assessment.Gross.Int64()), big.NewInt(rateTotal))
	if numerator.Sign() > 0 {
		numerator.Add(numerator, new(big.Int).Sub(denominator, big.NewInt(1)))
	}
	numerator.Quo(numerator, denominator)
	if !numerator.IsInt64() || numerator.Int64() > domain.MaxMoney.Int64() {
		return ReserveAssessment{}, domain.Invalid("reserve stress calculation exceeds supported amount")
	}
	assessment.RequiredReserve = domain.Money(numerator.Int64())
	assessment.Accepted = assessment.RequiredReserve <= available
	if !assessment.Accepted {
		return assessment, domain.Exposuref("treasury reserve requirement %s exceeds available %s", assessment.RequiredReserve, available)
	}
	return assessment, nil
}

func (g *ReserveGate) Commit(assessment ReserveAssessment) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.epoch != assessment.Epoch {
		g.epoch = assessment.Epoch
		g.flows = make(map[reserveKey]map[string]domain.Money)
	}
	key := reserveKey{institution: assessment.Institution, asset: assessment.Asset}
	flows := make(map[string]domain.Money, len(assessment.Rails))
	for _, rail := range assessment.Rails {
		flows[rail.Rail] = rail.Gross
	}
	g.flows[key] = flows
}
