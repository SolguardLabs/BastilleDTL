package ledger

import "github.com/solguardlabs/bastilledtl/src/domain"

type EntryType string

const (
	EntryCredit     EntryType = "credit"
	EntryDebit      EntryType = "debit"
	EntryReserve    EntryType = "reserve"
	EntryRelease    EntryType = "release"
	EntryTransfer   EntryType = "transfer"
	EntryWithdrawal EntryType = "withdrawal"
)

type Entry struct {
	ID          domain.LedgerEntryID `json:"id"`
	Type        EntryType            `json:"type"`
	Epoch       uint64               `json:"epoch"`
	Source      domain.AccountID     `json:"source,omitempty"`
	Destination domain.AccountID     `json:"destination,omitempty"`
	Asset       domain.AssetID       `json:"asset"`
	Amount      domain.Money         `json:"amount"`
	Operation   domain.OperationID   `json:"operation,omitempty"`
	Memo        string               `json:"memo,omitempty"`
}

func (e Entry) SignedAmount(account domain.AccountID) domain.Money {
	switch e.Type {
	case EntryCredit:
		if e.Destination == account {
			return e.Amount
		}
	case EntryDebit, EntryWithdrawal:
		if e.Source == account {
			return -e.Amount
		}
	case EntryTransfer:
		if e.Source == account {
			return -e.Amount
		}
		if e.Destination == account {
			return e.Amount
		}
	case EntryReserve:
		if e.Source == account {
			return 0
		}
	case EntryRelease:
		if e.Source == account {
			return 0
		}
	}
	return 0
}

func (e Entry) Involves(account domain.AccountID) bool {
	return e.Source == account || e.Destination == account
}

func FilterByOperation(entries []Entry, operation domain.OperationID) []Entry {
	filtered := make([]Entry, 0)
	for _, entry := range entries {
		if entry.Operation == operation {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func FilterByAccount(entries []Entry, account domain.AccountID) []Entry {
	filtered := make([]Entry, 0)
	for _, entry := range entries {
		if entry.Involves(account) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func SumEntries(entries []Entry, account domain.AccountID, asset domain.AssetID) domain.Money {
	var total domain.Money
	for _, entry := range entries {
		if entry.Asset != asset {
			continue
		}
		total += entry.SignedAmount(account)
	}
	return total
}
