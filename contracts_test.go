package backupcontracts

import (
	"testing"
	"time"
)

func validRequest() FileSnapshotRequest {
	return FileSnapshotRequest{
		Ref:            InstallRef{ProjectID: "p1", ClusterID: "c1", InstallName: "i1"},
		Component:      "files",
		LocalRoot:      "/srv/data/tree",
		DestinationRef: "conn-1/bucket",
		Stamp:          "2026-11-02T0300Z",
		Session: DestinationSession{
			Kind:      KindS3,
			ExpiresAt: time.Now().Add(time.Hour),
			S3:        &S3Session{Bucket: "b", AccessKeyID: "a", SecretKey: "s"},
		},
		RepoKey: WrappedKeyMaterial{Ciphertext: []byte{1}, WrapAlgo: "age", KeyID: "k1"},
	}
}

func TestValidateAcceptsWellFormedRequest(t *testing.T) {
	if err := validRequest().Validate(); err != nil {
		t.Fatalf("well-formed request refused: %v", err)
	}
}

func TestValidateRefusesRelativeLocalRoot(t *testing.T) {
	r := validRequest()
	r.LocalRoot = "srv/data/tree"
	if err := r.Validate(); err == nil {
		t.Fatal("relative LocalRoot accepted")
	}
}

func TestValidateRefusesUnknownSessionKind(t *testing.T) {
	r := validRequest()
	r.Session.Kind = "gcs"
	if err := r.Validate(); err == nil {
		t.Fatal("unknown session kind accepted")
	}
}

func TestValidateRefusesKindPayloadMismatch(t *testing.T) {
	r := validRequest()
	r.Session.S3 = nil
	if err := r.Validate(); err == nil {
		t.Fatal("s3 kind without an s3 session accepted")
	}
}

func TestValidateRefusesExpiredSession(t *testing.T) {
	r := validRequest()
	r.Session.ExpiresAt = time.Now().Add(-time.Minute)
	if err := r.Validate(); err == nil {
		t.Fatal("expired session accepted")
	}
}

func TestValidateRefusesIncompleteKeyMaterial(t *testing.T) {
	r := validRequest()
	r.RepoKey.WrapAlgo = ""
	if err := r.Validate(); err == nil {
		t.Fatal("incomplete wrapped key material accepted")
	}
}

func TestInstallRefStringIsSlashJoined(t *testing.T) {
	ref := InstallRef{"p", "c", "i"}
	if got := ref.String(); got != "p/c/i" {
		t.Fatalf("InstallRef.String() = %q, want %q", got, "p/c/i")
	}
}
