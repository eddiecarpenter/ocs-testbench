/**
 * Tests for the Steps-tab UX-layer validation. Pins the rolled-up
 * problems the Save flow surfaces before the schema's contractual
 * constraints fire.
 */
import { describe, expect, it } from 'vitest';

import type { ScenarioStep } from '../types';

import { validateStep, validateSteps } from './stepsValidation';

const updateStep = (
  repeat?: ScenarioStep extends { kind: 'request'; requestType: 'UPDATE' }
    ? Parameters<typeof Object>[0]
    : never,
): ScenarioStep => ({
  kind: 'request',
  requestType: 'UPDATE',
  ...(repeat ? { repeat } : {}),
});

describe('validateStep — repeat policy', () => {
  it('UPDATE without repeat is fine', () => {
    expect(validateStep(updateStep(), 'session')).toEqual([]);
  });

  it('UPDATE with `times` only is fine', () => {
    expect(
      validateStep(updateStep({ times: 3 }), 'session'),
    ).toEqual([]);
  });

  it('UPDATE with `until` only is fine', () => {
    expect(
      validateStep(
        updateStep({ until: { variable: 'X', op: 'gte', value: 0 } }),
        'session',
      ),
    ).toEqual([]);
  });

  it('UPDATE with empty repeat object reports the missing-bound problem', () => {
    const out = validateStep(updateStep({}), 'session');
    expect(out).toHaveLength(1);
    expect(out[0]).toMatch(/unbounded loops are not allowed/);
  });

  it('UPDATE with `until` missing a variable is rejected', () => {
    const out = validateStep(
      updateStep({ until: { variable: '', op: 'gte', value: 0 } }),
      'session',
    );
    expect(out.some((p) => p.includes('missing a variable'))).toBe(true);
  });

  it('Repeat on a non-UPDATE request step is rejected', () => {
    const step: ScenarioStep = {
      kind: 'request',
      requestType: 'INITIAL',
      // @ts-expect-error — repeat is UPDATE-only
      repeat: { times: 2 },
    };
    const out = validateStep(step, 'session');
    expect(out.some((p) => p.includes('only valid on UPDATE'))).toBe(true);
  });

  it('Illegal request type for the session mode is reported', () => {
    const step: ScenarioStep = { kind: 'request', requestType: 'EVENT' };
    const out = validateStep(step, 'session');
    expect(
      out.some((p) => p.includes('not legal under sessionMode session')),
    ).toBe(true);
  });
});

describe('validateSteps', () => {
  it('aggregates per-step problems with positional context', () => {
    const out = validateSteps(
      [
        { kind: 'request', requestType: 'INITIAL' },
        updateStep({}), // unbounded loop
        { kind: 'request', requestType: 'TERMINATE' },
      ],
      'session',
    );
    expect(out).toHaveLength(1);
    expect(out[0]).toMatch(/^Step 2:/);
  });

  it('returns empty when every step is valid', () => {
    const out = validateSteps(
      [
        { kind: 'request', requestType: 'INITIAL' },
        updateStep({ times: 3 }),
        { kind: 'request', requestType: 'TERMINATE' },
      ],
      'session',
    );
    expect(out).toEqual([]);
  });
});
