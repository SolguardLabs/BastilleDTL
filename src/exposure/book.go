package exposure

import (
	"sort"
	"sync"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type Usage struct {
	Cap  domain.ExposureCap
	Used domain.Money
}

type Book struct {
	mu    sync.Mutex
	caps  map[domain.CapID]Usage
	flows []Flow
}

func NewBook() *Book {
	return &Book{
		caps:  make(map[domain.CapID]Usage),
		flows: make([]Flow, 0, 128),
	}
}

func (b *Book) Register(cap domain.ExposureCap) error {
	if err := cap.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.caps[cap.ID]; ok {
		existing.Cap = cap
		b.caps[cap.ID] = existing
		return nil
	}
	b.caps[cap.ID] = Usage{Cap: cap}
	return nil
}

func (b *Book) RegisterMany(caps []domain.ExposureCap) error {
	for _, cap := range caps {
		if err := b.Register(cap); err != nil {
			return err
		}
	}
	return nil
}

func (b *Book) Admit(operation domain.Operation) (Admission, error) {
	if err := operation.Validate(); err != nil {
		return Admission{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	matches := make([]CapAdmission, 0)
	for _, usage := range b.caps {
		if !usage.Cap.Matches(operation) {
			continue
		}
		next := usage.Used + operation.Amount
		capAdmission := CapAdmission{
			CapID:     usage.Cap.ID,
			Limit:     usage.Cap.Limit,
			Current:   usage.Used,
			Projected: next,
			Accepted:  next <= usage.Cap.Limit,
		}
		matches = append(matches, capAdmission)
	}
	sort.Slice(matches, func(i int, j int) bool {
		return matches[i].CapID < matches[j].CapID
	})
	admission := Admission{
		Operation:  operation.ID,
		Accepted:   true,
		CapResults: matches,
	}
	for _, match := range matches {
		if !match.Accepted {
			admission.Accepted = false
			admission.Reason = "exposure cap exceeded"
			return admission, domain.Exposuref("cap %s projected %s exceeds limit %s", match.CapID, match.Projected, match.Limit)
		}
	}
	return admission, nil
}

func (b *Book) Apply(operation domain.Operation, epoch uint64) (Admission, error) {
	admission, err := b.Admit(operation)
	if err != nil {
		return admission, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, usage := range b.caps {
		if !usage.Cap.Matches(operation) {
			continue
		}
		usage.Used += operation.Amount
		b.caps[id] = usage
		b.flows = append(b.flows, Flow{
			Epoch:       epoch,
			CapID:       id,
			Operation:   operation.ID,
			Institution: operation.Institution,
			Account:     operation.SourceAccount,
			Asset:       operation.Asset,
			Kind:        operation.Kind,
			Amount:      operation.Amount,
		})
	}
	return admission, nil
}

func (b *Book) Reverse(operation domain.Operation, epoch uint64, reason string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, usage := range b.caps {
		if !usage.Cap.Matches(operation) {
			continue
		}
		usage.Used -= operation.Amount
		if usage.Used < 0 {
			usage.Used = 0
		}
		b.caps[id] = usage
		b.flows = append(b.flows, Flow{
			Epoch:       epoch,
			CapID:       id,
			Operation:   operation.ID,
			Institution: operation.Institution,
			Account:     operation.SourceAccount,
			Asset:       operation.Asset,
			Kind:        operation.Kind,
			Amount:      -operation.Amount,
			Reason:      reason,
		})
	}
	return nil
}

func (b *Book) Snapshot() []domain.ExposureSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	snapshots := make([]domain.ExposureSnapshot, 0, len(b.caps))
	for _, usage := range b.caps {
		remaining := usage.Cap.Limit - usage.Used
		if remaining < 0 {
			remaining = 0
		}
		snapshots = append(snapshots, domain.ExposureSnapshot{
			CapID:       usage.Cap.ID,
			Institution: usage.Cap.Institution,
			Account:     usage.Cap.Account,
			Asset:       usage.Cap.Asset,
			Kind:        usage.Cap.Kind,
			Limit:       usage.Cap.Limit,
			Used:        usage.Used,
			Remaining:   remaining,
		})
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].CapID < snapshots[j].CapID
	})
	return snapshots
}

func (b *Book) Flows() []Flow {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Flow(nil), b.flows...)
}

func (b *Book) Issues() []domain.AuditIssue {
	b.mu.Lock()
	defer b.mu.Unlock()
	issues := make([]domain.AuditIssue, 0)
	for _, usage := range b.caps {
		if usage.Used > usage.Cap.Limit {
			issues = append(issues, domain.AuditIssue{
				Code:     "exposure_over_cap",
				Severity: "critical",
				Message:  "exposure exceeds configured cap",
				Fields: map[string]string{
					"cap":   usage.Cap.ID.String(),
					"used":  usage.Used.String(),
					"limit": usage.Cap.Limit.String(),
				},
			})
		}
	}
	return issues
}
