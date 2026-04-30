package diameter

// This file is the shared-types home for the diameter package. Task 1
// of feature #17 only ships the dictionary loader and therefore needs
// no exported types here yet — the file is created so the package
// shape declared by the design plan exists from day one.
//
// Subsequent tasks of the feature add to this file:
//
//   - PeerConfig              — task 2 (per-peer connection lifecycle)
//   - ConnectionState, StateEvent — task 2 (per-peer state machine)
//   - CCR, CCA                — task 4 (Sender interface)
//   - SessionTerminationEvent — task 5 (protocol behaviour)
//   - Package-level error sentinels — added incrementally per task
//
// Keeping the file in source from task 1 means later commits only have
// to grow it, not create it, which keeps the per-task diffs scoped to
// the feature they actually add.
