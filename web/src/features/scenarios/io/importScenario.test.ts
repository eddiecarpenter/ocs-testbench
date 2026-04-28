/**
 * Tests for the Import flow's parse + zod validation.
 *
 * Covers AC-35 / AC-36 from Feature #77 — valid imports succeed,
 * invalid imports surface field-level errors and are rejected.
 */
import { describe, expect, it } from 'vitest';

import { parseAndValidateScenarioJson } from './importScenario';

const VALID = {
  id: 'scn-test',
  name: 'imported',
  description: '',
  unitType: 'OCTET',
  sessionMode: 'session',
  serviceModel: 'single-mscc',
  origin: 'user',
  favourite: false,
  subscriberId: 'sub-001',
  peerId: 'peer-01',
  stepCount: 1,
  updatedAt: '2026-04-28T10:00:00Z',
  avpTree: [
    { name: 'Origin-Host', code: 264, valueRef: 'ORIGIN_HOST' },
  ],
  services: [
    {
      id: '100',
      ratingGroup: 'RATING_GROUP',
      requestedUnits: 'RSU_TOTAL',
    },
  ],
  variables: [
    {
      name: 'MSISDN',
      source: { kind: 'bound', from: 'subscriber', field: 'msisdn' },
    },
  ],
  steps: [{ kind: 'request', requestType: 'INITIAL' }],
};

describe('parseAndValidateScenarioJson', () => {
  it('accepts a valid scenario JSON', () => {
    const out = parseAndValidateScenarioJson(JSON.stringify(VALID));
    expect(out.ok).toBe(true);
    if (out.ok) {
      expect(out.value.name).toBe('imported');
    }
  });

  it('returns a parse error on malformed JSON', () => {
    const out = parseAndValidateScenarioJson('not json');
    expect(out.ok).toBe(false);
    if (!out.ok) {
      expect(out.error.kind).toBe('parse');
    }
  });

  it('returns schema errors for an invalid unitType', () => {
    const bad = { ...VALID, unitType: 'BYTES' };
    const out = parseAndValidateScenarioJson(JSON.stringify(bad));
    expect(out.ok).toBe(false);
    if (!out.ok && out.error.kind === 'schema') {
      expect(out.error.errors.some((e) => e.path.includes('unitType'))).toBe(true);
    }
  });

  it('rejects a step with an unknown kind', () => {
    const bad = {
      ...VALID,
      steps: [{ kind: 'destroy', requestType: 'INITIAL' }],
    };
    const out = parseAndValidateScenarioJson(JSON.stringify(bad));
    expect(out.ok).toBe(false);
  });

  it('rejects a missing required field (name)', () => {
    const bad = { ...VALID } as Record<string, unknown>;
    delete bad.name;
    const out = parseAndValidateScenarioJson(JSON.stringify(bad));
    expect(out.ok).toBe(false);
  });
});
