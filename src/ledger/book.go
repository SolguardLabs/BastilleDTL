package ledger

import (
	"sort"
	"sync"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type Balance struct {
	Account   domain.AccountID
	Asset     domain.AssetID
	Available domain.Money
	Reserved  domain.Money
}

func (b Balance) Snapshot() domain.BalanceSnapshot {
	return domain.BalanceSnapshot{
		Account:   b.Account,
		Asset:     b.Asset,
		Available: b.Available,
		Reserved:  b.Reserved,
	}
}

type key struct {
	account domain.AccountID
	asset   domain.AssetID
}

type Book struct {
	mu       sync.Mutex
	balances map[key]Balance
	journal  []Entry
	seq      uint64
}

func NewBook() *Book {
	return &Book{
		balances: make(map[key]Balance),
		journal:  make([]Entry, 0, 128),
	}
}

func (b *Book) ApplySeeds(seeds []domain.BalanceSeed) error {
	for _, seed := range seeds {
		if err := seed.Validate(); err != nil {
			return err
		}
		if err := b.Set(seed.Account, seed.Asset, seed.Available, seed.Reserved); err != nil {
			return err
		}
	}
	return nil
}

func (b *Book) Set(account domain.AccountID, asset domain.AssetID, available domain.Money, reserved domain.Money) error {
	if available < 0 || reserved < 0 {
		return domain.Invalid("ledger balances must be non-negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.balances[key{account: account, asset: asset}] = Balance{
		Account:   account,
		Asset:     asset,
		Available: available,
		Reserved:  reserved,
	}
	return nil
}

func (b *Book) Balance(account domain.AccountID, asset domain.AssetID) Balance {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.balanceLocked(account, asset)
}

func (b *Book) Available(account domain.AccountID, asset domain.AssetID) domain.Money {
	return b.Balance(account, asset).Available
}

func (b *Book) Reserved(account domain.AccountID, asset domain.AssetID) domain.Money {
	return b.Balance(account, asset).Reserved
}

func (b *Book) Reserve(account domain.AccountID, asset domain.AssetID, amount domain.Money, reason string, epoch uint64) (Entry, error) {
	if amount <= 0 {
		return Entry{}, domain.Invalid("reserve amount must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	bal := b.balanceLocked(account, asset)
	if bal.Available < amount {
		return Entry{}, domain.Fundsf("account %s has %s available, need %s", account, bal.Available, amount)
	}
	bal.Available -= amount
	bal.Reserved += amount
	b.balances[key{account: account, asset: asset}] = bal
	entry := b.postLocked(epoch, EntryReserve, account, "", asset, amount, reason, "")
	return entry, nil
}

func (b *Book) Release(account domain.AccountID, asset domain.AssetID, amount domain.Money, reason string, epoch uint64) (Entry, error) {
	if amount <= 0 {
		return Entry{}, domain.Invalid("release amount must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	bal := b.balanceLocked(account, asset)
	if bal.Reserved < amount {
		return Entry{}, domain.Fundsf("account %s has %s reserved, need %s", account, bal.Reserved, amount)
	}
	bal.Reserved -= amount
	bal.Available += amount
	b.balances[key{account: account, asset: asset}] = bal
	entry := b.postLocked(epoch, EntryRelease, account, "", asset, amount, reason, "")
	return entry, nil
}

func (b *Book) Transfer(source domain.AccountID, destination domain.AccountID, asset domain.AssetID, amount domain.Money, operation domain.OperationID, memo string, epoch uint64) (Entry, error) {
	if amount <= 0 {
		return Entry{}, domain.Invalid("transfer amount must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	src := b.balanceLocked(source, asset)
	if src.Available < amount {
		return Entry{}, domain.Fundsf("account %s has %s available, need %s", source, src.Available, amount)
	}
	dst := b.balanceLocked(destination, asset)
	src.Available -= amount
	dst.Available += amount
	b.balances[key{account: source, asset: asset}] = src
	b.balances[key{account: destination, asset: asset}] = dst
	entry := b.postLocked(epoch, EntryTransfer, source, destination, asset, amount, memo, operation)
	return entry, nil
}

func (b *Book) Withdraw(source domain.AccountID, asset domain.AssetID, amount domain.Money, operation domain.OperationID, beneficiary string, rail string, epoch uint64) (Entry, error) {
	if amount <= 0 {
		return Entry{}, domain.Invalid("withdraw amount must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	src := b.balanceLocked(source, asset)
	if src.Available < amount {
		return Entry{}, domain.Fundsf("account %s has %s available, need %s", source, src.Available, amount)
	}
	src.Available -= amount
	b.balances[key{account: source, asset: asset}] = src
	note := "withdraw:" + rail + ":" + beneficiary
	entry := b.postLocked(epoch, EntryWithdrawal, source, "", asset, amount, note, operation)
	return entry, nil
}

func (b *Book) Credit(account domain.AccountID, asset domain.AssetID, amount domain.Money, reason string, epoch uint64) (Entry, error) {
	if amount <= 0 {
		return Entry{}, domain.Invalid("credit amount must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	bal := b.balanceLocked(account, asset)
	bal.Available += amount
	b.balances[key{account: account, asset: asset}] = bal
	entry := b.postLocked(epoch, EntryCredit, "", account, asset, amount, reason, "")
	return entry, nil
}

func (b *Book) Debit(account domain.AccountID, asset domain.AssetID, amount domain.Money, reason string, epoch uint64) (Entry, error) {
	if amount <= 0 {
		return Entry{}, domain.Invalid("debit amount must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	bal := b.balanceLocked(account, asset)
	if bal.Available < amount {
		return Entry{}, domain.Fundsf("account %s has %s available, need %s", account, bal.Available, amount)
	}
	bal.Available -= amount
	b.balances[key{account: account, asset: asset}] = bal
	entry := b.postLocked(epoch, EntryDebit, account, "", asset, amount, reason, "")
	return entry, nil
}

func (b *Book) Snapshots() []domain.BalanceSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	snapshots := make([]domain.BalanceSnapshot, 0, len(b.balances))
	for _, bal := range b.balances {
		snapshots = append(snapshots, bal.Snapshot())
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].Account == snapshots[j].Account {
			return snapshots[i].Asset < snapshots[j].Asset
		}
		return snapshots[i].Account < snapshots[j].Account
	})
	return snapshots
}

func (b *Book) Journal() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	journal := append([]Entry(nil), b.journal...)
	return journal
}

func (b *Book) AssertSolvent() []domain.AuditIssue {
	b.mu.Lock()
	defer b.mu.Unlock()
	issues := make([]domain.AuditIssue, 0)
	for _, bal := range b.balances {
		if bal.Available < 0 || bal.Reserved < 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "negative_balance",
				Severity: "critical",
				Message:  "ledger balance is negative",
				Fields: map[string]string{
					"account": bal.Account.String(),
					"asset":   bal.Asset.String(),
				},
			})
		}
	}
	return issues
}

func (b *Book) balanceLocked(account domain.AccountID, asset domain.AssetID) Balance {
	bal, ok := b.balances[key{account: account, asset: asset}]
	if !ok {
		return Balance{Account: account, Asset: asset}
	}
	return bal
}

func (b *Book) postLocked(epoch uint64, typ EntryType, source domain.AccountID, destination domain.AccountID, asset domain.AssetID, amount domain.Money, memo string, operation domain.OperationID) Entry {
	b.seq++
	entry := Entry{
		ID:          domain.NewLedgerEntryID(epoch, b.seq),
		Type:        typ,
		Epoch:       epoch,
		Source:      source,
		Destination: destination,
		Asset:       asset,
		Amount:      amount,
		Operation:   operation,
		Memo:        memo,
	}
	b.journal = append(b.journal, entry)
	return entry
}
