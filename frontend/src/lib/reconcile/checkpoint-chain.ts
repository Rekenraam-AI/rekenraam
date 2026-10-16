type ChainLink = { id: number; status: string; previous_checkpoint_id?: number };

// Voiding a checkpoint does not cascade (#240): an active checkpoint whose
// previous_checkpoint_id chain passes through the voided one still covers its
// postings, because its statement balance includes them. This mirrors the
// backend's notCoveredByCheckpointSQL so the void confirmation can say whether
// the postings will actually be offered again.
export function coveredByLaterActiveCheckpoint(checkpointID: number, checkpoints: ChainLink[]): boolean {
  const byID = new Map(checkpoints.map((c) => [c.id, c]));
  for (const candidate of checkpoints) {
    if (candidate.status !== 'active' || candidate.id === checkpointID) continue;
    const seen = new Set<number>([candidate.id]);
    let previous = candidate.previous_checkpoint_id;
    while (previous !== undefined && !seen.has(previous)) {
      if (previous === checkpointID) return true;
      seen.add(previous);
      previous = byID.get(previous)?.previous_checkpoint_id;
    }
  }
  return false;
}
