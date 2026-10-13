import { describe, it, expect } from 'vitest';
import { basisSourceDeclaration, basisSourceFields } from './basis-provenance';

describe('basis source declarations', () => {
  it('requires meaningful references and an explicit source kind', () => {
    expect(basisSourceDeclaration('', 'statement', '', '')).toBeNull();
    expect(basisSourceDeclaration('statement', '  ', '', '')).toBeNull();
    expect(basisSourceDeclaration('provider', ' synthetic row 42 ', '', '')).toEqual({
      kind: 'provider', reference: 'synthetic row 42'
    });
  });

  it('requires a rationale and knowledge date for an owner assertion', () => {
    expect(basisSourceDeclaration('owner_assertion', '', '', '2026-10-13')).toBeNull();
    expect(basisSourceDeclaration('owner_assertion', '', 'records', '')).toBeNull();
    expect(basisSourceDeclaration('owner_assertion', '', ' records ', '2026-10-13')).toEqual({
      kind: 'owner_assertion', rationale: 'records', known_on: '2026-10-13'
    });
  });

  it('prefills historical references without claiming a typed declaration', () => {
    expect(basisSourceFields({ statement: 'legacy statement' })).toEqual({
      kind: '', reference: 'legacy statement', rationale: '', knownOn: ''
    });
    expect(basisSourceFields({ kind: 'owner_assertion', rationale: 'records', known_on: '2026-10-13' })).toEqual({
      kind: 'owner_assertion', reference: '', rationale: 'records', knownOn: '2026-10-13'
    });
    expect(basisSourceFields({ reference: 42 }).reference).toBe('');
  });
});
