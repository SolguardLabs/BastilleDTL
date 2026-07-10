package domain

type EventType string

const (
	EventServiceStarted    EventType = "service_started"
	EventOperationProposed EventType = "operation_proposed"
	EventApprovalCreated   EventType = "approval_created"
	EventApprovalRejected  EventType = "approval_rejected"
	EventOperationApproved EventType = "operation_approved"
	EventOperationExecuted EventType = "operation_executed"
	EventOperationRejected EventType = "operation_rejected"
	EventSignerRotated     EventType = "signer_rotated"
	EventEpochAdvanced     EventType = "epoch_advanced"
	EventLedgerPosted      EventType = "ledger_posted"
	EventExposureApplied   EventType = "exposure_applied"
	EventAuditFlag         EventType = "audit_flag"
)

type Event struct {
	ID          EventID           `json:"id"`
	Type        EventType         `json:"type"`
	Epoch       uint64            `json:"epoch"`
	Institution InstitutionID     `json:"institution,omitempty"`
	Operation   OperationID       `json:"operation,omitempty"`
	Approval    ApprovalID        `json:"approval,omitempty"`
	Signer      SignerID          `json:"signer,omitempty"`
	Account     AccountID         `json:"account,omitempty"`
	Asset       AssetID           `json:"asset,omitempty"`
	Amount      Money             `json:"amount,omitempty"`
	Message     string            `json:"message,omitempty"`
	Fields      map[string]string `json:"fields,omitempty"`
}

func NewEvent(id EventID, eventType EventType, epoch uint64, message string) Event {
	return Event{ID: id, Type: eventType, Epoch: epoch, Message: message}
}

func (e Event) WithOperation(id OperationID) Event {
	e.Operation = id
	return e
}

func (e Event) WithApproval(id ApprovalID) Event {
	e.Approval = id
	return e
}

func (e Event) WithSigner(id SignerID) Event {
	e.Signer = id
	return e
}

func (e Event) WithAmount(asset AssetID, amount Money) Event {
	e.Asset = asset
	e.Amount = amount
	return e
}

func (e Event) WithInstitution(id InstitutionID) Event {
	e.Institution = id
	return e
}

func (e Event) WithField(key string, value string) Event {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[key] = value
	return e
}
