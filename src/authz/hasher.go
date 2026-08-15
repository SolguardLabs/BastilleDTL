package authz

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

func PartialEconomicHash(operation domain.Operation) string {
	payload := strings.Join([]string{
		"institution=" + operation.Institution.String(),
		"source=" + operation.SourceAccount.String(),
		"asset=" + operation.Asset.String(),
		fmt.Sprintf("amount=%d", operation.Amount),
		"bucket=" + string(operation.EffectiveRiskBucket()),
	}, "|")
	return digest(payload)
}

func FullOperationHash(operation domain.Operation) string {
	parts := []string{
		"id=" + operation.ID.String(),
		"institution=" + operation.Institution.String(),
		"kind=" + string(operation.Kind),
		"source=" + operation.SourceAccount.String(),
		"destination=" + operation.DestinationAccount.String(),
		"externalBeneficiary=" + strings.ToLower(strings.TrimSpace(operation.ExternalBeneficiary)),
		"externalRail=" + strings.ToLower(strings.TrimSpace(operation.ExternalRail)),
		"asset=" + operation.Asset.String(),
		fmt.Sprintf("amount=%d", operation.Amount),
		"requestedBy=" + operation.RequestedBy.String(),
		"memo=" + strings.TrimSpace(operation.Memo),
		"bucket=" + string(operation.EffectiveRiskBucket()),
	}
	return digest(strings.Join(parts, "|"))
}

func ApprovalIntent(operation domain.Operation, signer domain.Signer) string {
	payload := strings.Join([]string{
		PartialEconomicHash(operation),
		FullOperationHash(operation),
		"signer=" + signer.ID.String(),
		"role=" + string(signer.Role),
		fmt.Sprintf("weight=%d", signer.Weight),
	}, "|")
	return digest(payload)
}

func digest(payload string) string {
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
