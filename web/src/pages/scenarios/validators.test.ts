/**
 * Tests for `serviceModel × serviceType` matrix enforcement, the
 * `sessionMode × requestType` step validator, and the Rating-Group
 * presence check for MSCC scenarios.
 *
 * Covers AC-25 / AC-26 / AC-19 from Feature #77.
 */
import { describe, expect, it } from 'vitest';

import type { Service } from './types';
import { matrix, validateScenario } from './validators';

/** Minimal valid service with a rating group set. */
const svcWithRG: Service = {
  id: '0',
  ratingGroup: 'RATING_GROUP',
  requestedUnits: 'RSU',
  usedUnits: 'USU',
};

/** Service missing the rating group — the case we are guarding against. */
const svcNoRG: Service = {
  id: '0',
  ratingGroup: '',
  requestedUnits: 'RSU',
  usedUnits: 'USU',
};

describe('matrix(unit, model)', () => {
  it('disallows VOLUME × root and surfaces a hint', () => {
    const cell = matrix('VOLUME', 'root');
    expect(cell.allowed).toBe(false);
    expect(cell.hint).toMatch(/MSCC/);
  });

  it('disallows multi-mscc for TIME and EVENT', () => {
    expect(matrix('TIME', 'multi-mscc').allowed).toBe(false);
    expect(matrix('EVENT', 'multi-mscc').allowed).toBe(false);
  });

  it('allows VOLUME × single-mscc, VOLUME × multi-mscc', () => {
    expect(matrix('VOLUME', 'single-mscc').allowed).toBe(true);
    expect(matrix('VOLUME', 'multi-mscc').allowed).toBe(true);
  });

  it('allows TIME × root, TIME × single-mscc, EVENT × root, EVENT × single-mscc', () => {
    expect(matrix('TIME', 'root').allowed).toBe(true);
    expect(matrix('TIME', 'single-mscc').allowed).toBe(true);
    expect(matrix('EVENT', 'root').allowed).toBe(true);
    expect(matrix('EVENT', 'single-mscc').allowed).toBe(true);
  });
});

describe('validateScenario', () => {
  it('rejects EVENT under sessionMode=session', () => {
    const issues = validateScenario({
      serviceType: 'DATA',
      serviceModel: 'single-mscc',
      sessionMode: 'session',
      services: [svcWithRG],
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
      serviceType: 'SMS',
      serviceModel: 'single-mscc',
      sessionMode: 'event',
      services: [svcWithRG],
      steps: [{ kind: 'request', requestType: 'UPDATE' }],
    });
    expect(issues.some((i) => i.path.endsWith('/requestType'))).toBe(true);
  });

  it('flags an invalid serviceModel × serviceType cell as a top-level issue', () => {
    const issues = validateScenario({
      serviceType: 'DATA',
      serviceModel: 'root',
      sessionMode: 'session',
      services: [],
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
      serviceType: 'DATA',
      serviceModel: 'single-mscc',
      sessionMode: 'session',
      services: [svcWithRG],
      steps: [
        { kind: 'request', requestType: 'INITIAL' },
        { kind: 'request', requestType: 'TERMINATE' },
      ],
    });
    expect(issues).toEqual([]);
  });

  // ── Rating-Group validation ──────────────────────────────────────────────

  it('blocks save on single-mscc when a service has no Rating-Group', () => {
    const issues = validateScenario({
      serviceType: 'VOICE',
      serviceModel: 'single-mscc',
      sessionMode: 'session',
      services: [svcNoRG],
      steps: [],
    });
    expect(issues).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          path: '/services/0/ratingGroup',
          message: expect.stringContaining('Rating-Group'),
        }),
      ]),
    );
  });

  it('blocks save on multi-mscc when any service has no Rating-Group', () => {
    const issues = validateScenario({
      serviceType: 'DATA',
      serviceModel: 'multi-mscc',
      sessionMode: 'session',
      services: [svcWithRG, svcNoRG],
      steps: [],
    });
    const rgIssues = issues.filter((i) => i.path.includes('ratingGroup'));
    expect(rgIssues).toHaveLength(1);
    expect(rgIssues[0].path).toBe('/services/1/ratingGroup');
  });

  it('does not require Rating-Group for root service model', () => {
    const issues = validateScenario({
      serviceType: 'VOICE',
      serviceModel: 'root',
      sessionMode: 'session',
      services: [svcNoRG],
      steps: [],
    });
    expect(issues.some((i) => i.path.includes('ratingGroup'))).toBe(false);
  });
});
