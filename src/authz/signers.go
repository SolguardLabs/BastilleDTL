package authz

import (
	"sort"
	"sync"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type SignerRegistry struct {
	mu      sync.Mutex
	signers map[domain.SignerID]domain.Signer
}

func NewSignerRegistry(signers []domain.Signer) (*SignerRegistry, error) {
	registry := &SignerRegistry{signers: make(map[domain.SignerID]domain.Signer)}
	for _, signer := range signers {
		if err := registry.Register(signer); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *SignerRegistry) Register(signer domain.Signer) error {
	if err := signer.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.signers[signer.ID]; exists {
		return domain.Invalidf("signer %s already exists", signer.ID)
	}
	r.signers[signer.ID] = signer
	return nil
}

func (r *SignerRegistry) Get(id domain.SignerID) (domain.Signer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	signer, ok := r.signers[id]
	if !ok {
		return domain.Signer{}, domain.NotFoundf("signer %s not found", id)
	}
	return signer, nil
}

func (r *SignerRegistry) Active(id domain.SignerID, epoch uint64) (domain.Signer, error) {
	signer, err := r.Get(id)
	if err != nil {
		return domain.Signer{}, err
	}
	if !signer.ActiveAt(epoch) {
		return domain.Signer{}, domain.SignerInactivef("signer %s is not active at epoch %d", id, epoch)
	}
	return signer, nil
}

func (r *SignerRegistry) Rotate(retire domain.SignerID, activate domain.Signer, actor domain.Principal, epoch uint64, policy domain.EnginePolicy) (domain.Signer, domain.Signer, error) {
	if !actor.CanManageSigners() {
		return domain.Signer{}, domain.Signer{}, domain.Permissionf("principal %s cannot rotate signers", actor.ID)
	}
	if err := activate.Validate(); err != nil {
		return domain.Signer{}, domain.Signer{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.signers[retire]
	if !ok {
		return domain.Signer{}, domain.Signer{}, domain.NotFoundf("signer %s not found", retire)
	}
	if old.Institution != activate.Institution {
		return domain.Signer{}, domain.Signer{}, domain.Invalid("rotated signers must belong to the same institution")
	}
	if policy.RotationDelayEpochs > 0 && old.ActivatedEpoch+policy.RotationDelayEpochs > epoch {
		return domain.Signer{}, domain.Signer{}, domain.NewError(domain.CodeRotationWindowClosed, "rotation delay window is still open")
	}
	old.Status = domain.SignerRetired
	old.RetiredEpoch = epoch
	if activate.ActivatedEpoch == 0 {
		activate.ActivatedEpoch = epoch
	}
	activate.Status = domain.SignerActive
	r.signers[old.ID] = old
	r.signers[activate.ID] = activate
	return old, activate, nil
}

func (r *SignerRegistry) Suspend(id domain.SignerID, actor domain.Principal, epoch uint64, reason string) (domain.Signer, error) {
	if !actor.CanManageSigners() {
		return domain.Signer{}, domain.Permissionf("principal %s cannot suspend signers", actor.ID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	signer, ok := r.signers[id]
	if !ok {
		return domain.Signer{}, domain.NotFoundf("signer %s not found", id)
	}
	signer.Status = domain.SignerSuspended
	signer.RetiredEpoch = epoch
	signer.Notes = reason
	r.signers[id] = signer
	return signer, nil
}

func (r *SignerRegistry) List() []domain.Signer {
	r.mu.Lock()
	defer r.mu.Unlock()
	signers := make([]domain.Signer, 0, len(r.signers))
	for _, signer := range r.signers {
		signers = append(signers, signer)
	}
	sort.Slice(signers, func(i int, j int) bool {
		return signers[i].ID < signers[j].ID
	})
	return signers
}

func (r *SignerRegistry) CountActive(institution domain.InstitutionID, epoch uint64) int {
	count := 0
	for _, signer := range r.List() {
		if signer.Institution == institution && signer.ActiveAt(epoch) {
			count++
		}
	}
	return count
}
