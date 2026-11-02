// Package backupcontracts holds the public wire contracts for bare-metal-cloud's
// backup executor: the file-snapshot dispatch schema spoken between the backup
// control plane and the executor beside the data (an agent on the node, or an
// in-cluster executor pod).
//
// The package is intentionally dependency-free (stdlib only) so both sides —
// the control plane and any executor, including open-source distributions —
// can import it without dragging private modules in. Field semantics:
//
//   - A run is authorized per job: the control plane opens an authorization
//     window, resolves the destination credentials, and hands the executor a
//     short-lived DestinationSession plus the WrappedKeyMaterial it needs for
//     exactly this snapshot. Sessions expire; an executor that starts after
//     expiry refuses. An already-started snapshot completes.
//   - LocalRoot is an absolute path on the executor's own filesystem; the
//     executor reads only beneath it.
package backupcontracts

import (
	"fmt"
	"path"
	"time"
)

// Destination kinds. The envelope is a closed vocabulary: unknown kinds refuse
// rather than pass through, so an executor never silently misreads a session.
const (
	KindS3     = "s3"
	KindAzBlob = "azblob"
)

// InstallRef names one install on one cluster.
type InstallRef struct {
	ProjectID   string `json:"project_id"`
	ClusterID   string `json:"cluster_id"`
	InstallName string `json:"install_name"`
}

// String renders the canonical label-alphabet form used to derive
// install-namespaced prefixes on a destination.
func (r InstallRef) String() string {
	return r.ProjectID + "/" + r.ClusterID + "/" + r.InstallName
}

// S3Session is a short-lived S3-compatible destination credential
// (STS-scoped: expiring keys for one bucket prefix).
type S3Session struct {
	Endpoint     string `json:"endpoint,omitempty"`
	Region       string `json:"region,omitempty"`
	Bucket       string `json:"bucket"`
	AccessKeyID  string `json:"access_key_id"`
	SecretKey    string `json:"secret_key"`
	SessionToken string `json:"session_token,omitempty"`
}

// AzBlobSession is a short-lived Azure Blob destination credential
// (container-scoped SAS; account keys never cross this wire).
type AzBlobSession struct {
	Endpoint  string `json:"endpoint,omitempty"`
	Account   string `json:"account"`
	Container string `json:"container"`
	SAS       string `json:"sas"`
}

// DestinationSession is the per-run destination credential envelope. It is
// minted per snapshot dispatch, expires, and must not be persisted by the
// executor beyond the run.
type DestinationSession struct {
	Kind      string         `json:"kind"`
	ExpiresAt time.Time      `json:"expires_at"`
	S3        *S3Session     `json:"s3,omitempty"`
	AzBlob    *AzBlobSession `json:"azblob,omitempty"`
}

// WrappedKeyMaterial is the snapshot's key material wrapped for this run:
// the ciphertext of the content/repo key under a key the executor can open,
// plus the wrap algorithm identifier and key version.
type WrappedKeyMaterial struct {
	Ciphertext []byte `json:"ciphertext"`
	WrapAlgo   string `json:"wrap_algo"`
	KeyID      string `json:"key_id"`
}

// FileSnapshotRequest is one file-tree snapshot dispatch: capture the tree at
// LocalRoot (on the executor's own filesystem) and seal it into the
// install-scoped repository at DestinationRef.
type FileSnapshotRequest struct {
	Ref            InstallRef         `json:"ref"`
	Component      string             `json:"component"`
	LocalRoot      string             `json:"local_root"`
	DestinationRef string             `json:"destination_ref"`
	Stamp          string             `json:"stamp"`
	Session        DestinationSession `json:"session"`
	RepoKey        WrappedKeyMaterial `json:"repo_key"`
}

// Validate enforces the wire's minimum shape law: absolute LocalRoot, known
// session kind, a matching session payload, an unexpired session, and key
// material present. Executors re-validate beyond this; the wire law is the
// floor, not the ceiling.
func (r FileSnapshotRequest) Validate() error {
	if !path.IsAbs(path.Clean(r.LocalRoot)) {
		return fmt.Errorf("backup-contracts: volume %q's root %q is not absolute", r.Component, r.LocalRoot)
	}
	if r.Session.ExpiresAt.IsZero() {
		return fmt.Errorf("backup-contracts: volume %q's session carries no expiry", r.Component)
	}
	if time.Now().After(r.Session.ExpiresAt) {
		return fmt.Errorf("backup-contracts: volume %q's session expired at %s — re-dispatch for a fresh window",
			r.Component, r.Session.ExpiresAt.Format(time.RFC3339))
	}
	switch r.Session.Kind {
	case KindS3:
		if r.Session.S3 == nil {
			return fmt.Errorf("backup-contracts: volume %q declares kind %q without an s3 session", r.Component, KindS3)
		}
	case KindAzBlob:
		if r.Session.AzBlob == nil {
			return fmt.Errorf("backup-contracts: volume %q declares kind %q without an azblob session", r.Component, KindAzBlob)
		}
	default:
		return fmt.Errorf("backup-contracts: volume %q declares unknown session kind %q", r.Component, r.Session.Kind)
	}
	if len(r.RepoKey.Ciphertext) == 0 || r.RepoKey.WrapAlgo == "" || r.RepoKey.KeyID == "" {
		return fmt.Errorf("backup-contracts: volume %q's wrapped key material is incomplete", r.Component)
	}
	return nil
}

// FileSnapshotResult reports a completed snapshot: where the sealed chunk
// manifest object landed, which key version sealed it, the content digest,
// and the transferred size.
type FileSnapshotResult struct {
	ChunkManifestObjectKey string `json:"chunk_manifest_object_key"`
	RepoKeyID              string `json:"repo_key_id"`
	Digest                 string `json:"digest"`
	BytesUploaded          int64  `json:"bytes_uploaded"`
}
