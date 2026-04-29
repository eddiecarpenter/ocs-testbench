/**
 * Tests for `expandScenarioSteps` — the helper that flattens a
 * scenario's authored step list into the per-round timeline the
 * engine actually executes. UPDATE-with-repeat is the only case
 * that expands; everything else passes through 1-for-1.
 */
import { describe, expect, it } from 'vitest';

import {
  DEFAULT_UNTIL_ROUNDS,
  expandScenarioSteps,
  totalRoundsFor,
} from './expandSteps';
import type { ScenarioStep } from './types';

const initial: ScenarioStep = { kind: 'request', requestType: 'INITIAL' };
const update: ScenarioStep = { kind: 'request', requestType: 'UPDATE' };
const terminate: ScenarioStep = { kind: 'request', requestType: 'TERMINATE' };
const event: ScenarioStep = { kind: 'request', requestType: 'EVENT' };

describe('totalRoundsFor', () => {
  it('non-request steps are 1', () => {
    expect(totalRoundsFor({ kind: 'wait', durationMs: 100 })).toBe(1);
    expect(totalRoundsFor({ kind: 'pause' })).toBe(1);
  });

  it('non-UPDATE request steps are 1, even with `repeat` shape attached', () => {
    expect(totalRoundsFor(initial)).toBe(1);
    expect(totalRoundsFor(terminate)).toBe(1);
    expect(totalRoundsFor(event)).toBe(1);
  });

  it('UPDATE without repeat is 1', () => {
    expect(totalRoundsFor(update)).toBe(1);
  });

  it('UPDATE with `times` returns the count', () => {
    expect(
      totalRoundsFor({ ...update, repeat: { times: 5 } }),
    ).toBe(5);
  });

  it('UPDATE with only `until` falls back to DEFAULT_UNTIL_ROUNDS', () => {
    expect(
      totalRoundsFor({
        ...update,
        repeat: { until: { variable: 'X', op: 'gte', value: 0 } },
      }),
    ).toBe(DEFAULT_UNTIL_ROUNDS);
  });

  it('UPDATE with both honours `times` (the hard cap)', () => {
    expect(
      totalRoundsFor({
        ...update,
        repeat: {
          times: 3,
          until: { variable: 'X', op: 'gte', value: 0 },
        },
      }),
    ).toBe(3);
  });
});

describe('expandScenarioSteps', () => {
  it('passes non-repeating steps through 1-for-1', () => {
    const out = expandScenarioSteps([initial, update, terminate]);
    expect(out).toHaveLength(3);
    expect(out.map((s) => s.sourceIndex)).toEqual([0, 1, 2]);
    expect(out.every((s) => s.roundIndex === undefined)).toBe(true);
  });

  it('expands an UPDATE with `times: 4` into 4 entries sharing sourceIndex', () => {
    const out = expandScenarioSteps([
      initial,
      { ...update, repeat: { times: 4, delayMs: 60_000 } },
      terminate,
    ]);
    expect(out).toHaveLength(6);
    // Sources: I (0), U×4 (1,1,1,1), T (2)
    expect(out.map((s) => s.sourceIndex)).toEqual([0, 1, 1, 1, 1, 2]);
    // Round indices on the expanded UPDATE entries
    expect(out.slice(1, 5).map((s) => s.roundIndex)).toEqual([1, 2, 3, 4]);
    expect(out.slice(1, 5).map((s) => s.totalRounds)).toEqual([4, 4, 4, 4]);
    expect(out.slice(1, 5).map((s) => s.delayMs)).toEqual([
      60_000,
      60_000,
      60_000,
      60_000,
    ]);
  });

  it('expands an UPDATE with only `until` to DEFAULT_UNTIL_ROUNDS entries', () => {
    const out = expandScenarioSteps([
      { ...update, repeat: { until: { variable: 'X', op: 'gte', value: 1 } } },
    ]);
    expect(out).toHaveLength(DEFAULT_UNTIL_ROUNDS);
    expect(out.every((s) => s.totalRounds === DEFAULT_UNTIL_ROUNDS)).toBe(true);
  });

  it('does not expand a non-UPDATE step even when given a stray repeat-shape', () => {
    // Defensive — even if a malformed scenario lands a `repeat` on a
    // non-UPDATE request step, expansion treats it as 1-for-1. The
    // schema's if/then/else rejects this at the contract layer.
    const out = expandScenarioSteps([
      // Cast — the type narrowing forbids this in legal code.
      {
        kind: 'request',
        requestType: 'INITIAL',
        // @ts-expect-error — repeat is UPDATE-only per the schema
        repeat: { times: 5 },
      },
    ]);
    expect(out).toHaveLength(1);
  });
});
