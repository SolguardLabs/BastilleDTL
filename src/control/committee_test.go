package control

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/solguardlabs/bastilledtl/src/domain"
)

type reviewerKey struct {
	reviewer Reviewer
	private  ed25519.PrivateKey
}

func key(seed byte, id domain.SignerID, weight uint16) reviewerKey {
	private := ed25519.NewKeyFromSeed(bytesOf(seed, ed25519.SeedSize))
	reviewer, err := NewReviewer(id, "inst:alpha", domain.RoleRisk, weight, private.Public().(ed25519.PublicKey))
	if err != nil {
		panic(err)
	}
	return reviewerKey{reviewer: reviewer, private: private}
}

func bytesOf(value byte, length int) []byte {
	result := make([]byte, length)
	for i := range result {
		result[i] = value
	}
	return result
}

func configDigest(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

func signedChange(t *testing.T, signer reviewerKey, nonce uint64, id string) SignedChange {
	t.Helper()
	signed, err := SignChange(Change{
		ID: id, Institution: "inst:alpha", Kind: ChangeExposureCap, ConfigDigest: configDigest(id),
		Proposer: signer.reviewer.ID, Nonce: nonce, NotBefore: 100, ExpiresAt: 120, Reason: "scheduled risk calibration",
	}, signer.private)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func signedVote(t *testing.T, signer reviewerKey, digest string, nonce uint64, decision VoteDecision) SignedVote {
	t.Helper()
	signed, err := SignVote(Vote{ChangeDigest: digest, Reviewer: signer.reviewer.ID, Nonce: nonce, Decision: decision}, signer.private)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func committee(t *testing.T) (*Committee, reviewerKey, reviewerKey, reviewerKey) {
	t.Helper()
	first := key(11, "signer:first", 1)
	second := key(22, "signer:second", 1)
	third := key(33, "signer:third", 1)
	value, err := NewCommittee(2, []Reviewer{first.reviewer, second.reviewer, third.reviewer})
	if err != nil {
		t.Fatal(err)
	}
	return value, first, second, third
}

func TestSignedQuorumAndTimelockGateExecution(t *testing.T) {
	value, first, second, _ := committee(t)
	digest, outcome, err := value.Submit(signedChange(t, first, 0, "change:limits:1"))
	if err != nil || outcome != OutcomePending {
		t.Fatalf("submit outcome=%s err=%v", outcome, err)
	}
	outcome, err = value.Vote(signedVote(t, second, digest, 0, VoteApprove))
	if err != nil || outcome != OutcomeApproved {
		t.Fatalf("vote outcome=%s err=%v", outcome, err)
	}
	if _, err := value.Execute(digest, 99); err == nil {
		t.Fatal("expected timelock rejection")
	}
	change, err := value.Execute(digest, 100)
	if err != nil || change.Kind != ChangeExposureCap || !value.Executed(digest) {
		t.Fatalf("execute change=%+v err=%v", change, err)
	}
}

func TestSignaturesAndNoncesRejectMutationAndReplay(t *testing.T) {
	value, first, _, _ := committee(t)
	tampered := signedChange(t, first, 0, "change:limits:2")
	tampered.Change.NotBefore = 101
	if _, _, err := value.Submit(tampered); err == nil {
		t.Fatal("expected signature mutation rejection")
	}
	valid := signedChange(t, first, 0, "change:limits:3")
	if _, _, err := value.Submit(valid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := value.Submit(signedChange(t, first, 0, "change:limits:4")); err == nil {
		t.Fatal("expected nonce replay rejection")
	}
}

func TestCancellationUsesIndependentQuorum(t *testing.T) {
	value, first, second, third := committee(t)
	digest, _, err := value.Submit(signedChange(t, first, 0, "change:limits:5"))
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := value.Vote(signedVote(t, second, digest, 0, VoteCancel))
	if err != nil || outcome != OutcomePending {
		t.Fatalf("first cancellation outcome=%s err=%v", outcome, err)
	}
	outcome, err = value.Vote(signedVote(t, third, digest, 0, VoteCancel))
	if err != nil || outcome != OutcomeCancelled {
		t.Fatalf("second cancellation outcome=%s err=%v", outcome, err)
	}
	if _, err := value.Execute(digest, 100); err == nil {
		t.Fatal("cancelled change must not execute")
	}
}

func TestReviewerCannotVoteTwice(t *testing.T) {
	value, first, second, _ := committee(t)
	digest, _, _ := value.Submit(signedChange(t, first, 0, "change:limits:6"))
	vote := signedVote(t, second, digest, 0, VoteApprove)
	if _, err := value.Vote(vote); err != nil {
		t.Fatal(err)
	}
	if _, err := value.Vote(vote); err == nil {
		t.Fatal("expected duplicate vote rejection")
	}
}

func TestReviewerRemovalProtectsQuorumAndPendingVotes(t *testing.T) {
	value, first, second, third := committee(t)
	digest, _, err := value.Submit(signedChange(t, first, 0, "change:limits:7"))
	if err != nil {
		t.Fatal(err)
	}
	if err := value.RemoveReviewer(first.reviewer.ID); err == nil {
		t.Fatal("expected pending proposer removal rejection")
	}
	if err := value.RemoveReviewer(third.reviewer.ID); err != nil {
		t.Fatalf("remove non-voting reviewer: %v", err)
	}
	if err := value.RemoveReviewer(second.reviewer.ID); err == nil {
		t.Fatal("expected quorum protection")
	}
	if _, ok := value.Pending(digest); !ok {
		t.Fatal("pending change unexpectedly removed")
	}
}
