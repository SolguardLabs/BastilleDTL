package scenario

import (
	"github.com/solguardlabs/bastilledtl/src/domain"
	"github.com/solguardlabs/bastilledtl/src/engine"
)

type Definition struct {
	Name      string           `json:"name"`
	Bootstrap engine.Bootstrap `json:"bootstrap"`
	Actions   []Action         `json:"actions"`
}

type Action struct {
	Type         string                `json:"type"`
	Label        string                `json:"label,omitempty"`
	Operation    domain.Operation      `json:"operation,omitempty"`
	Approval     ApprovalAction        `json:"approval,omitempty"`
	Rotation     domain.SignerRotation `json:"rotation,omitempty"`
	Delta        uint64                `json:"delta,omitempty"`
	ApprovalRefs []string              `json:"approvalRefs,omitempty"`
	ExpectError  domain.Code           `json:"expectError,omitempty"`
	Metadata     map[string]string     `json:"metadata,omitempty"`
}

type ApprovalAction struct {
	Operation    domain.Operation        `json:"operation"`
	SignerID     domain.SignerID         `json:"signerId"`
	Decision     domain.ApprovalDecision `json:"decision"`
	Reason       string                  `json:"reason,omitempty"`
	ApprovalRefs []string                `json:"approvalRefs,omitempty"`
}

type Result struct {
	Name     string                `json:"name"`
	Results  []ActionResult        `json:"results"`
	Snapshot domain.SystemSnapshot `json:"snapshot"`
	Report   engine.TreasuryReport `json:"report"`
}

type ActionResult struct {
	Type      string                   `json:"type"`
	Label     string                   `json:"label,omitempty"`
	Approval  *domain.Approval         `json:"approval,omitempty"`
	Execution *engine.ExecutionReceipt `json:"execution,omitempty"`
	Rotation  *engine.RotationReceipt  `json:"rotation,omitempty"`
	Snapshot  *domain.SystemSnapshot   `json:"snapshot,omitempty"`
	Report    *engine.TreasuryReport   `json:"report,omitempty"`
	Epoch     *uint64                  `json:"epoch,omitempty"`
	Error     *domain.Error            `json:"error,omitempty"`
}
