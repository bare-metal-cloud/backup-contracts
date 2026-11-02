# backup-contracts

Public wire contracts for bare-metal-cloud's backup executor: the
file-snapshot dispatch schema spoken between the backup control plane and the
executor beside the data (an agent on the node, or an in-cluster executor
pod).

Dependency-free on purpose (stdlib only): the control plane and any executor
— including open-source distributions — import it without dragging private
modules in.

## Semantics

- A snapshot run is **authorized per job**: the control plane opens an
  authorization window, resolves the destination credentials, and hands the
  executor a short-lived `DestinationSession` plus the `WrappedKeyMaterial`
  for exactly this snapshot. Sessions expire; an executor that starts after
  expiry refuses. An already-started snapshot completes.
- `LocalRoot` is an absolute path on the executor's own filesystem; the
  executor reads only beneath it.
- `FileSnapshotRequest.Validate` is the wire's minimum shape law — executors
  re-validate beyond it.

## Consumed by

- bare-metal-cloud/backend (the backup executor / control plane)
- bare-metal-cloud/agent (the node-side executor)

## License

Apache-2.0
