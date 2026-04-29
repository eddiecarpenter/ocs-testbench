/**
 * Tests for the Start-Run dialog's input-builder.
 *
 * Covers AC-13 / AC-14 / AC-15 — the contract-shape rules the
 * payload must satisfy before hitting POST /executions.
 */
import { describe, expect, it } from 'vitest';

import { buildStartExecutionInput } from './buildStartExecutionInput';

const baseForm = {
  scenarioId: 'scn-1',
  peerId: null,
  subscriberId: null,
  concurrency: 5,
  repeats: 50,
} as const;

describe('buildStartExecutionInput', () => {
  it('forces concurrency = 1 and repeats = 1 in interactive mode', () => {
    const out = buildStartExecutionInput({
      ...baseForm,
      mode: 'interactive',
    });
    expect(out).toEqual({
      scenarioId: 'scn-1',
      mode: 'interactive',
      concurrency: 1,
      repeats: 1,
    });
  });

  it('keeps concurrency / repeats in continuous mode', () => {
    const out = buildStartExecutionInput({
      ...baseForm,
      mode: 'continuous',
    });
    expect(out.concurrency).toBe(5);
    expect(out.repeats).toBe(50);
  });

  it('clamps concurrency to [1, 10]', () => {
    expect(
      buildStartExecutionInput({
        ...baseForm,
        mode: 'continuous',
        concurrency: 0,
      }).concurrency,
    ).toBe(1);
    expect(
      buildStartExecutionInput({
        ...baseForm,
        mode: 'continuous',
        concurrency: 99,
      }).concurrency,
    ).toBe(10);
  });

  it('clamps repeats to [1, 1000]', () => {
    expect(
      buildStartExecutionInput({
        ...baseForm,
        mode: 'continuous',
        repeats: 0,
      }).repeats,
    ).toBe(1);
    expect(
      buildStartExecutionInput({
        ...baseForm,
        mode: 'continuous',
        repeats: 5_000,
      }).repeats,
    ).toBe(1000);
  });

  it('omits `overrides` when both peer and subscriber are null', () => {
    const out = buildStartExecutionInput({
      ...baseForm,
      mode: 'interactive',
    });
    expect(out.overrides).toBeUndefined();
  });

  it('includes peerId when set', () => {
    const out = buildStartExecutionInput({
      ...baseForm,
      mode: 'interactive',
      peerId: 'peer-99',
    });
    expect(out.overrides).toEqual({ peerId: 'peer-99' });
  });

  it('wraps subscriberId in subscriberIds[]', () => {
    const out = buildStartExecutionInput({
      ...baseForm,
      mode: 'interactive',
      subscriberId: 'sub-77',
    });
    expect(out.overrides).toEqual({ subscriberIds: ['sub-77'] });
  });

  it('combines peer and subscriber overrides', () => {
    const out = buildStartExecutionInput({
      ...baseForm,
      mode: 'continuous',
      peerId: 'peer-99',
      subscriberId: 'sub-77',
    });
    expect(out.overrides).toEqual({
      peerId: 'peer-99',
      subscriberIds: ['sub-77'],
    });
  });
});
