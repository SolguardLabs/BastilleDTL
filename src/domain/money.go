package domain

import (
	"fmt"
	"math"
)

type Money int64

const MaxMoney Money = Money(math.MaxInt64 / 4)

func (m Money) Int64() int64 {
	return int64(m)
}

func (m Money) IsPositive() bool {
	return m > 0
}

func (m Money) IsNegative() bool {
	return m < 0
}

func (m Money) IsZero() bool {
	return m == 0
}

func (m Money) Add(other Money) (Money, error) {
	if other > 0 && m > MaxMoney-other {
		return 0, Invalid("money addition overflow")
	}
	if other < 0 && m < -MaxMoney-other {
		return 0, Invalid("money addition underflow")
	}
	return m + other, nil
}

func (m Money) Sub(other Money) (Money, error) {
	return m.Add(-other)
}

func (m Money) Mul(n int64) (Money, error) {
	if n == 0 || m == 0 {
		return 0, nil
	}
	if n > 0 && m > MaxMoney/Money(n) {
		return 0, Invalid("money multiplication overflow")
	}
	if n < 0 && m < MaxMoney/Money(n) {
		return 0, Invalid("money multiplication underflow")
	}
	return m * Money(n), nil
}

func (m Money) String() string {
	return fmt.Sprintf("%d", m)
}

func MinMoney(a Money, b Money) Money {
	if a < b {
		return a
	}
	return b
}

func MaxOfMoney(a Money, b Money) Money {
	if a > b {
		return a
	}
	return b
}

func MustPositiveMoney(label string, amount Money) error {
	if amount <= 0 {
		return Invalidf("%s must be positive", label)
	}
	return nil
}

func MustNonNegativeMoney(label string, amount Money) error {
	if amount < 0 {
		return Invalidf("%s must be non-negative", label)
	}
	return nil
}

type Asset struct {
	ID        AssetID `json:"id"`
	Symbol    string  `json:"symbol"`
	Decimals  uint8   `json:"decimals"`
	Stable    bool    `json:"stable"`
	Enabled   bool    `json:"enabled"`
	Network   string  `json:"network,omitempty"`
	RiskClass string  `json:"riskClass,omitempty"`
}

func (a Asset) Validate() error {
	if err := ValidateID("asset", a.ID.String()); err != nil {
		return err
	}
	if a.Symbol == "" {
		return Invalidf("asset %s symbol is required", a.ID)
	}
	if a.Decimals > 18 {
		return Invalidf("asset %s decimals exceed 18", a.ID)
	}
	return nil
}

type BalanceSeed struct {
	Account   AccountID `json:"account"`
	Asset     AssetID   `json:"asset"`
	Available Money     `json:"available"`
	Reserved  Money     `json:"reserved,omitempty"`
}

func (b BalanceSeed) Validate() error {
	if err := ValidateID("account", b.Account.String()); err != nil {
		return err
	}
	if err := ValidateID("asset", b.Asset.String()); err != nil {
		return err
	}
	if err := MustNonNegativeMoney("available", b.Available); err != nil {
		return err
	}
	if err := MustNonNegativeMoney("reserved", b.Reserved); err != nil {
		return err
	}
	return nil
}
