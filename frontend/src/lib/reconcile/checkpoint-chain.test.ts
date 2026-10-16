import { describe, it, expect } from 'vitest';
import { coveredByLaterActiveCheckpoint } from './checkpoint-chain';

describe('coveredByLaterActiveCheckpoint', () => {
  it('is false for the latest active checkpoint', () => {
    const checkpoints = [
      { id: 1, status: 'active' },
      { id: 2, status: 'active', previous_checkpoint_id: 1 }
    ];
    expect(coveredByLaterActiveCheckpoint(2, checkpoints)).toBe(false);
  });

  it('is true when a later active checkpoint chains directly from it', () => {
    const checkpoints = [
      { id: 1, status: 'active' },
      { id: 2, status: 'active', previous_checkpoint_id: 1 }
    ];
    expect(coveredByLaterActiveCheckpoint(1, checkpoints)).toBe(true);
  });

  it('follows the chain through a voided middle checkpoint', () => {
    const checkpoints = [
      { id: 1, status: 'active' },
      { id: 2, status: 'voided', previous_checkpoint_id: 1 },
      { id: 3, status: 'active', previous_checkpoint_id: 2 }
    ];
    expect(coveredByLaterActiveCheckpoint(1, checkpoints)).toBe(true);
  });

  it('ignores chains that end in a non-active checkpoint', () => {
    const checkpoints = [
      { id: 1, status: 'active' },
      { id: 2, status: 'invalidated', previous_checkpoint_id: 1 }
    ];
    expect(coveredByLaterActiveCheckpoint(1, checkpoints)).toBe(false);
  });

  it('terminates on a cyclic chain', () => {
    const checkpoints = [
      { id: 1, status: 'active', previous_checkpoint_id: 2 },
      { id: 2, status: 'active', previous_checkpoint_id: 1 }
    ];
    expect(coveredByLaterActiveCheckpoint(3, checkpoints)).toBe(false);
  });
});
