import { describe, expect, it } from 'vitest';
import { evidenceText, newSourceEvidence, sourceEvidenceEdits } from './source-evidence';

describe('sourceEvidenceEdits', () => {
  // The recorded object came through JSON.parse: 9007199254740993 is already
  // rounded here, so it must never be sent back (#311).
  const recorded = { reference: 'issuer notice', ex_date: '2026-01-05', revision: 9007199254740993, nested: { a: 1 } };

  it('sends nothing when the shown fields are unchanged', () => {
    expect(sourceEvidenceEdits(recorded, { reference: ' issuer notice ', ex_date: '2026-01-05' })).toBeUndefined();
  });

  it('sends only changed keys and never the unedited metadata', () => {
    expect(sourceEvidenceEdits(recorded, { reference: 'amended notice', ex_date: '2026-01-05' }))
      .toEqual({ reference: 'amended notice' });
  });

  it('removes a cleared key with null, including the only key', () => {
    expect(sourceEvidenceEdits({ reference: 'x' }, { reference: '  ' })).toEqual({ reference: null });
  });

  it('leaves a non-text recorded value alone until the owner types one', () => {
    expect(sourceEvidenceEdits({ reference: 42 }, { reference: '' })).toBeUndefined();
    expect(sourceEvidenceEdits({ reference: 42 }, { reference: 'r' })).toEqual({ reference: 'r' });
    expect(evidenceText({ reference: 42 }, 'reference')).toBe('');
  });

  it('adds a key the recorded evidence lacked', () => {
    expect(sourceEvidenceEdits(undefined, { ex_date: '2026-02-01' })).toEqual({ ex_date: '2026-02-01' });
  });
});

describe('newSourceEvidence', () => {
  it('keeps trimmed non-empty fields and is undefined when all are empty', () => {
    expect(newSourceEvidence({ reference: ' r ', ex_date: '' })).toEqual({ reference: 'r' });
    expect(newSourceEvidence({ reference: '' })).toBeUndefined();
  });
});
