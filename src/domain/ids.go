package domain

import (
	"fmt"
	"strings"
)

type InstitutionID string
type AccountID string
type AssetID string
type PrincipalID string
type SignerID string
type OperationID string
type ApprovalID string
type EventID string
type LedgerEntryID string
type CapID string

func (id InstitutionID) String() string { return string(id) }
func (id AccountID) String() string     { return string(id) }
func (id AssetID) String() string       { return string(id) }
func (id PrincipalID) String() string   { return string(id) }
func (id SignerID) String() string      { return string(id) }
func (id OperationID) String() string   { return string(id) }
func (id ApprovalID) String() string    { return string(id) }
func (id EventID) String() string       { return string(id) }
func (id LedgerEntryID) String() string { return string(id) }
func (id CapID) String() string         { return string(id) }

func NewOperationID(epoch uint64, seq uint64) OperationID {
	return OperationID(fmt.Sprintf("op:%06d:%06d", epoch, seq))
}

func NewApprovalID(epoch uint64, seq uint64) ApprovalID {
	return ApprovalID(fmt.Sprintf("appr:%06d:%06d", epoch, seq))
}

func NewEventID(epoch uint64, seq uint64) EventID {
	return EventID(fmt.Sprintf("evt:%06d:%06d", epoch, seq))
}

func NewLedgerEntryID(epoch uint64, seq uint64) LedgerEntryID {
	return LedgerEntryID(fmt.Sprintf("je:%06d:%06d", epoch, seq))
}

func ValidateID(kind string, value string) error {
	if strings.TrimSpace(value) == "" {
		return Invalidf("%s id is required", kind)
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return Invalidf("%s id must not contain whitespace", kind)
	}
	return nil
}

func EmptyAccount(id AccountID) bool {
	return strings.TrimSpace(id.String()) == ""
}

func EmptyAsset(id AssetID) bool {
	return strings.TrimSpace(id.String()) == ""
}

func EmptyInstitution(id InstitutionID) bool {
	return strings.TrimSpace(id.String()) == ""
}

func EmptySigner(id SignerID) bool {
	return strings.TrimSpace(id.String()) == ""
}

func EmptyOperation(id OperationID) bool {
	return strings.TrimSpace(id.String()) == ""
}

func EmptyApproval(id ApprovalID) bool {
	return strings.TrimSpace(id.String()) == ""
}

func NormalizeID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func JoinScope(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, ":")
}
