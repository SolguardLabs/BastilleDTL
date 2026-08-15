package control

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

const (
	changeDomain = "bastille-control-change-v1"
	voteDomain   = "bastille-control-vote-v1"
)

type ChangeKind string

const (
	ChangeEnginePolicy ChangeKind = "engine_policy"
	ChangeSignerSet    ChangeKind = "signer_set"
	ChangeExposureCap  ChangeKind = "exposure_cap"
	ChangeRailPolicy   ChangeKind = "rail_policy"
	ChangeReserveFloor ChangeKind = "reserve_floor"
	ChangeEmergency    ChangeKind = "emergency_restriction"
)

type VoteDecision string

const (
	VoteApprove VoteDecision = "approve"
	VoteCancel  VoteDecision = "cancel"
)

type Change struct {
	ID           string
	Institution  domain.InstitutionID
	Kind         ChangeKind
	ConfigDigest string
	Proposer     domain.SignerID
	Nonce        uint64
	NotBefore    uint64
	ExpiresAt    uint64
	Reason       string
}

type SignedChange struct {
	Change    Change
	PublicKey ed25519.PublicKey
	Signature []byte
}

type Vote struct {
	ChangeDigest string
	Reviewer     domain.SignerID
	Nonce        uint64
	Decision     VoteDecision
}

type SignedVote struct {
	Vote      Vote
	PublicKey ed25519.PublicKey
	Signature []byte
}

type Reviewer struct {
	ID          domain.SignerID
	Institution domain.InstitutionID
	Role        domain.Role
	Weight      uint16
	PublicKey   ed25519.PublicKey
	Active      bool
}

type PendingChange struct {
	Change             Change                     `json:"change"`
	Digest             string                     `json:"digest"`
	ApprovalWeight     uint16                     `json:"approvalWeight"`
	CancellationWeight uint16                     `json:"cancellationWeight"`
	Approvals          map[domain.SignerID]uint16 `json:"approvals"`
	Cancellations      map[domain.SignerID]uint16 `json:"cancellations"`
	Approved           bool                       `json:"approved"`
}

type VoteOutcome string

const (
	OutcomePending   VoteOutcome = "pending"
	OutcomeApproved  VoteOutcome = "approved"
	OutcomeCancelled VoteOutcome = "cancelled"
)

type Committee struct {
	mu        sync.Mutex
	quorum    uint16
	reviewers map[domain.SignerID]Reviewer
	nonces    map[domain.SignerID]uint64
	pending   map[string]PendingChange
	executed  map[string]Change
	cancelled map[string]Change
}

func NewCommittee(quorum uint16, reviewers []Reviewer) (*Committee, error) {
	if quorum == 0 {
		return nil, domain.Invalid("control quorum must be positive")
	}
	committee := &Committee{
		quorum: quorum, reviewers: map[domain.SignerID]Reviewer{}, nonces: map[domain.SignerID]uint64{},
		pending: map[string]PendingChange{}, executed: map[string]Change{}, cancelled: map[string]Change{},
	}
	for _, reviewer := range reviewers {
		if err := committee.addReviewerLocked(reviewer); err != nil {
			return nil, err
		}
	}
	if committee.activeWeightLocked() < quorum {
		return nil, domain.Invalid("control quorum exceeds active reviewer weight")
	}
	return committee, nil
}

func NewReviewer(id domain.SignerID, institution domain.InstitutionID, role domain.Role, weight uint16, publicKey ed25519.PublicKey) (Reviewer, error) {
	reviewer := Reviewer{ID: id, Institution: institution, Role: role, Weight: weight, PublicKey: append(ed25519.PublicKey(nil), publicKey...), Active: true}
	if err := reviewer.validate(); err != nil {
		return Reviewer{}, err
	}
	return reviewer, nil
}

func SignChange(change Change, privateKey ed25519.PrivateKey) (SignedChange, error) {
	if err := change.validate(); err != nil {
		return SignedChange{}, err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return SignedChange{}, domain.Invalid("control private key has invalid length")
	}
	message := changeMessage(change)
	return SignedChange{
		Change:    change,
		PublicKey: append(ed25519.PublicKey(nil), privateKey.Public().(ed25519.PublicKey)...),
		Signature: ed25519.Sign(privateKey, message),
	}, nil
}

func SignVote(vote Vote, privateKey ed25519.PrivateKey) (SignedVote, error) {
	if err := vote.validate(); err != nil {
		return SignedVote{}, err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return SignedVote{}, domain.Invalid("control private key has invalid length")
	}
	return SignedVote{
		Vote:      vote,
		PublicKey: append(ed25519.PublicKey(nil), privateKey.Public().(ed25519.PublicKey)...),
		Signature: ed25519.Sign(privateKey, voteMessage(vote)),
	}, nil
}

func (c *Committee) Submit(signed SignedChange) (string, VoteOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := signed.Change.validate(); err != nil {
		return "", "", err
	}
	reviewer, err := c.verifyChangeLocked(signed)
	if err != nil {
		return "", "", err
	}
	if signed.Change.Institution != reviewer.Institution {
		return "", "", domain.Permission("control proposer belongs to another institution")
	}
	if err := c.consumeNonceLocked(reviewer.ID, signed.Change.Nonce); err != nil {
		return "", "", err
	}
	digest := ChangeDigest(signed.Change)
	if _, exists := c.pending[digest]; exists {
		return "", "", domain.Invalid("control change digest is already pending")
	}
	if _, exists := c.executed[digest]; exists {
		return "", "", domain.Invalid("control change digest was already executed")
	}
	if _, exists := c.cancelled[digest]; exists {
		return "", "", domain.Invalid("control change digest was already cancelled")
	}
	pending := PendingChange{
		Change: signed.Change, Digest: digest, ApprovalWeight: reviewer.Weight,
		Approvals:     map[domain.SignerID]uint16{reviewer.ID: reviewer.Weight},
		Cancellations: map[domain.SignerID]uint16{},
	}
	pending.Approved = pending.ApprovalWeight >= c.quorum
	c.pending[digest] = pending
	if pending.Approved {
		return digest, OutcomeApproved, nil
	}
	return digest, OutcomePending, nil
}

func (c *Committee) Vote(signed SignedVote) (VoteOutcome, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := signed.Vote.validate(); err != nil {
		return "", err
	}
	reviewer, err := c.verifyVoteLocked(signed)
	if err != nil {
		return "", err
	}
	pending, exists := c.pending[signed.Vote.ChangeDigest]
	if !exists {
		return "", domain.NotFound("control change is not pending")
	}
	if pending.Change.Institution != reviewer.Institution {
		return "", domain.Permission("control reviewer belongs to another institution")
	}
	if _, voted := pending.Approvals[reviewer.ID]; voted {
		return "", domain.SignerConflictf("reviewer %s already voted", reviewer.ID)
	}
	if _, voted := pending.Cancellations[reviewer.ID]; voted {
		return "", domain.SignerConflictf("reviewer %s already voted", reviewer.ID)
	}
	if err := c.consumeNonceLocked(reviewer.ID, signed.Vote.Nonce); err != nil {
		return "", err
	}
	switch signed.Vote.Decision {
	case VoteApprove:
		pending.Approvals[reviewer.ID] = reviewer.Weight
		pending.ApprovalWeight = saturatingAdd(pending.ApprovalWeight, reviewer.Weight)
		pending.Approved = pending.ApprovalWeight >= c.quorum
		c.pending[pending.Digest] = pending
		if pending.Approved {
			return OutcomeApproved, nil
		}
	case VoteCancel:
		pending.Cancellations[reviewer.ID] = reviewer.Weight
		pending.CancellationWeight = saturatingAdd(pending.CancellationWeight, reviewer.Weight)
		if pending.CancellationWeight >= c.quorum {
			delete(c.pending, pending.Digest)
			c.cancelled[pending.Digest] = pending.Change
			return OutcomeCancelled, nil
		}
		c.pending[pending.Digest] = pending
	}
	return OutcomePending, nil
}

func (c *Committee) Execute(digest string, epoch uint64) (Change, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pending, exists := c.pending[digest]
	if !exists {
		return Change{}, domain.NotFound("control change is not pending")
	}
	if !pending.Approved || pending.ApprovalWeight < c.quorum {
		return Change{}, domain.Permission("control change has not reached quorum")
	}
	if epoch < pending.Change.NotBefore {
		return Change{}, domain.NewError(domain.CodeRotationWindowClosed, "control timelock is active")
	}
	if epoch > pending.Change.ExpiresAt {
		return Change{}, domain.NewError(domain.CodeRotationWindowClosed, "control execution window expired")
	}
	delete(c.pending, digest)
	c.executed[digest] = pending.Change
	return pending.Change, nil
}

func (c *Committee) AddReviewer(reviewer Reviewer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addReviewerLocked(reviewer)
}

func (c *Committee) RemoveReviewer(id domain.SignerID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	reviewer, exists := c.reviewers[id]
	if !exists || !reviewer.Active {
		return domain.NotFoundf("control reviewer %s is not active", id)
	}
	if c.activeWeightLocked()-reviewer.Weight < c.quorum {
		return domain.Limit("control reviewer removal would violate quorum")
	}
	for _, pending := range c.pending {
		if _, voted := pending.Approvals[id]; voted {
			return domain.SignerConflictf("reviewer %s has a pending approval", id)
		}
		if _, voted := pending.Cancellations[id]; voted {
			return domain.SignerConflictf("reviewer %s has a pending cancellation", id)
		}
	}
	reviewer.Active = false
	c.reviewers[id] = reviewer
	return nil
}

func (c *Committee) SetQuorum(quorum uint16) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if quorum == 0 || quorum > c.activeWeightLocked() {
		return domain.Invalid("control quorum is outside active reviewer weight")
	}
	c.quorum = quorum
	return nil
}

func (c *Committee) Pending(digest string) (PendingChange, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pending, ok := c.pending[digest]
	if !ok {
		return PendingChange{}, false
	}
	return clonePending(pending), true
}

func (c *Committee) Nonce(id domain.SignerID) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.nonces[id]
	if !ok {
		return 0, domain.NotFoundf("control reviewer %s is not registered", id)
	}
	return value, nil
}

func (c *Committee) Executed(digest string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.executed[digest]
	return ok
}

func (c *Committee) Reviewers() []Reviewer {
	c.mu.Lock()
	defer c.mu.Unlock()
	values := make([]Reviewer, 0, len(c.reviewers))
	for _, reviewer := range c.reviewers {
		clone := reviewer
		clone.PublicKey = append(ed25519.PublicKey(nil), reviewer.PublicKey...)
		values = append(values, clone)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values
}

func ChangeDigest(change Change) string {
	sum := sha256.Sum256(changeMessage(change))
	return hex.EncodeToString(sum[:])
}

func (r Reviewer) validate() error {
	if err := domain.ValidateID("control reviewer", r.ID.String()); err != nil {
		return err
	}
	if err := domain.ValidateID("institution", r.Institution.String()); err != nil {
		return err
	}
	if !domain.RoleCanApprove(r.Role) {
		return domain.Permissionf("role %s cannot review control changes", r.Role)
	}
	if r.Weight == 0 {
		return domain.Invalid("control reviewer weight must be positive")
	}
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return domain.Invalid("control reviewer public key has invalid length")
	}
	if !r.Active {
		return domain.Invalid("control reviewer must be active when registered")
	}
	return nil
}

func (c Change) validate() error {
	if err := domain.ValidateID("control change", c.ID); err != nil {
		return err
	}
	if err := domain.ValidateID("institution", c.Institution.String()); err != nil {
		return err
	}
	if err := domain.ValidateID("proposer", c.Proposer.String()); err != nil {
		return err
	}
	switch c.Kind {
	case ChangeEnginePolicy, ChangeSignerSet, ChangeExposureCap, ChangeRailPolicy, ChangeReserveFloor, ChangeEmergency:
	default:
		return domain.Invalid("control change kind is unsupported")
	}
	decoded, err := hex.DecodeString(c.ConfigDigest)
	if err != nil || len(decoded) != sha256.Size {
		return domain.Invalid("control config digest must be 32-byte hexadecimal")
	}
	if c.NotBefore >= c.ExpiresAt {
		return domain.Invalid("control execution window is invalid")
	}
	if strings.TrimSpace(c.Reason) == "" {
		return domain.Invalid("control change reason is required")
	}
	return nil
}

func (v Vote) validate() error {
	decoded, err := hex.DecodeString(v.ChangeDigest)
	if err != nil || len(decoded) != sha256.Size {
		return domain.Invalid("control vote digest must be 32-byte hexadecimal")
	}
	if err := domain.ValidateID("control reviewer", v.Reviewer.String()); err != nil {
		return err
	}
	if v.Decision != VoteApprove && v.Decision != VoteCancel {
		return domain.Invalid("control vote decision is unsupported")
	}
	return nil
}

func (c *Committee) addReviewerLocked(reviewer Reviewer) error {
	if err := reviewer.validate(); err != nil {
		return err
	}
	if _, exists := c.reviewers[reviewer.ID]; exists {
		return domain.Invalidf("control reviewer %s already exists", reviewer.ID)
	}
	reviewer.PublicKey = append(ed25519.PublicKey(nil), reviewer.PublicKey...)
	c.reviewers[reviewer.ID] = reviewer
	c.nonces[reviewer.ID] = 0
	return nil
}

func (c *Committee) verifyChangeLocked(signed SignedChange) (Reviewer, error) {
	reviewer, err := c.activeReviewerLocked(signed.Change.Proposer)
	if err != nil {
		return Reviewer{}, err
	}
	if !sameKey(reviewer.PublicKey, signed.PublicKey) || !ed25519.Verify(reviewer.PublicKey, changeMessage(signed.Change), signed.Signature) {
		return Reviewer{}, domain.Permission("control change signature is invalid")
	}
	return reviewer, nil
}

func (c *Committee) verifyVoteLocked(signed SignedVote) (Reviewer, error) {
	reviewer, err := c.activeReviewerLocked(signed.Vote.Reviewer)
	if err != nil {
		return Reviewer{}, err
	}
	if !sameKey(reviewer.PublicKey, signed.PublicKey) || !ed25519.Verify(reviewer.PublicKey, voteMessage(signed.Vote), signed.Signature) {
		return Reviewer{}, domain.Permission("control vote signature is invalid")
	}
	return reviewer, nil
}

func (c *Committee) activeReviewerLocked(id domain.SignerID) (Reviewer, error) {
	reviewer, exists := c.reviewers[id]
	if !exists || !reviewer.Active {
		return Reviewer{}, domain.Permissionf("control reviewer %s is not active", id)
	}
	return reviewer, nil
}

func (c *Committee) consumeNonceLocked(id domain.SignerID, received uint64) error {
	expected, ok := c.nonces[id]
	if !ok {
		return domain.NotFoundf("control reviewer %s is not registered", id)
	}
	if received != expected {
		return domain.Invalidf("control reviewer %s nonce %d does not match %d", id, received, expected)
	}
	if expected == ^uint64(0) {
		return domain.Limit("control reviewer nonce overflow")
	}
	c.nonces[id] = expected + 1
	return nil
}

func (c *Committee) activeWeightLocked() uint16 {
	var total uint16
	for _, reviewer := range c.reviewers {
		if reviewer.Active {
			total = saturatingAdd(total, reviewer.Weight)
		}
	}
	return total
}

func changeMessage(change Change) []byte {
	payload := fmt.Sprintf("%s|id=%s|institution=%s|kind=%s|config=%s|proposer=%s|nonce=%d|notBefore=%d|expires=%d|reason=%s",
		changeDomain, change.ID, change.Institution, change.Kind, strings.ToLower(change.ConfigDigest), change.Proposer,
		change.Nonce, change.NotBefore, change.ExpiresAt, strings.TrimSpace(change.Reason))
	return []byte(payload)
}

func voteMessage(vote Vote) []byte {
	return []byte(fmt.Sprintf("%s|change=%s|reviewer=%s|nonce=%d|decision=%s",
		voteDomain, strings.ToLower(vote.ChangeDigest), vote.Reviewer, vote.Nonce, vote.Decision))
}

func sameKey(expected, received ed25519.PublicKey) bool {
	if len(expected) != len(received) || len(expected) != ed25519.PublicKeySize {
		return false
	}
	return subtle.ConstantTimeCompare(expected, received) == 1
}

func saturatingAdd(left, right uint16) uint16 {
	if ^uint16(0)-left < right {
		return ^uint16(0)
	}
	return left + right
}

func clonePending(value PendingChange) PendingChange {
	clone := value
	clone.Approvals = make(map[domain.SignerID]uint16, len(value.Approvals))
	for id, weight := range value.Approvals {
		clone.Approvals[id] = weight
	}
	clone.Cancellations = make(map[domain.SignerID]uint16, len(value.Cancellations))
	for id, weight := range value.Cancellations {
		clone.Cancellations[id] = weight
	}
	return clone
}
