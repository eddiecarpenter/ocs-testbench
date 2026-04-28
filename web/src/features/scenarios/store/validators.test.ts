/**
 * Tests for `serviceModel × unitType` matrix enforcement and the
 * `sessionMode × requestType` step validator.
 *
 * Covers AC-25 / AC-26 / AC-19 from Feature #77.
 */
import { describe, expect, it } from 'vitest';

import { matrix, validateScenario } from './validators';

describe('matrix(unit, model)', () => {
  it('disallows OCTET × root and surfaces a hint', () => {
    const cell = matrix('OCTET', 'root');
    expect(cell.allowed).toBe(false);
    expect(cell.hint).toMatch(/MSCC/);
  });

  it('disallows multi-mscc for TIME and UNITS', () => {
    expect(matrix('TIME', 'multi-mscc').allowed).toBe(false);
    expect(matrix('UNITS', 'multi-mscc').allowed).toBe(false);
  });

  it('allows OCTET × single-mscc, OCTET × multi-mscc', () => {
    expect(matrix('OCTET', 'single-mscc').allowed).toBe(true);
    expect(matrix('OCTET', 'multi-mscc').allowed).toBe(true);
  });

  it('allows TIME × root, TIME × single-mscc, UNITS × root, UNITS × single-mscc', () => {
    expect(matrix('TIME', 'root').allowed).toBe(true);
    expect(matrix('TIME', 'single-mscc').allowed).toBe(true);
    expect(matrix('UNITS', 'root').allowed).toBe(true);
    expect(matrix('UNITS', 'single-mscc').allowed).toBe(true);
  });
});

describe('validateScenario', () => {
  it('rejects EVENT under sessionMode=session', () => {
    const issues = validateScenario({
      unitType: 'OCTET',
      serviceModel: 'single-mscc',
      sessionMode: 'session',
      steps: [{ kind: 'request', requestType: 'EVENT' }],
    });
    expect(issues).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: '/steps/0/requestType',
          message: expect.stringContaining('EVENT'),
        }),
      ]),
    );
  });

  it('rejects INITIAL/UPDATE/TERMINATE under sessionMode=event', () => {
    const issues = validateScenario({
      unitType: 'UNITS',
      serviceModel: 'single-mscc',
      sessionMode: 'event',
      steps: [{ kind: 'request', requestType: 'UPDATE' }],
    });
    expect(issues.some((i) => i.path.endsWith('/requestType'))).toBe(true);
  });

  it('flags an invalid serviceModel × unitType cell as a top-level issue', () => {
    const issues = validateScenario({
      unitType: 'OCTET',
      serviceModel: 'root',
      sessionMode: 'session',
      steps: [],
    });
    expect(issues).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ path: '/serviceModel' }),
      ]),
    );
  });

  it('returns no issues for a fully-valid scenario', () => {
    const issues = validateScenario({
      unitType: 'OCTET',
      serviceModel: 'single-mscc',
      sessionMode: 'session',
      steps: [
        { kind: 'request', requestType: 'INITIAL' },
        { kind: 'request', requestType: 'TERMINATE' },
      ],
    });
    expect(issues).toEqual([]);
  });
});
