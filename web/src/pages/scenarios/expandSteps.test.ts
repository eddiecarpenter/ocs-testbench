/**
 * Tests for `expandScenarioSteps` — the helper that flattens a
 * scenario's authored step list into the per-round timeline the
 * engine actually executes. UPDATE-with-repeat is the only case
 * that expands; everything else passes through 1-for-1.
 */
import { describe, expect, it } from 'vitest';

import {
  DEFAULT_UNTIL_ROUNDS,
  MAX_SIMULATED_ROUNDS,
  expandScenarioSteps,
  totalRoundsFor,
} from './expandSteps';
import type { Scenario, ScenarioStep, Variable } from './types';

/** A scenario shape just rich enough for the simulator to sample. */
function scenarioWith(variables: Variable[]): Pick<Scenario, 'variables'> {
  return { variables };
}

const literalVar = (name: string, value: number): Variable => ({
  name,
  source: {
    kind: 'generator',
    strategy: 'literal',
    refresh: 'once',
    params: { value },
  },
});

const randomVar = (name: string, min: number, max: number): Variable => ({
  name,
  source: {
    kind: 'generator',
    strategy: 'random-int',
    refresh: 'per-send',
    params: { min, max },
  },
});

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

  it('UPDATE with only `until` and no scenario context falls back to DEFAULT_UNTIL_ROUNDS', () => {
    expect(
      totalRoundsFor({
        ...update,
        repeat: { until: { variable: 'X', op: 'gte', value: 0 } },
      }),
    ).toBe(DEFAULT_UNTIL_ROUNDS);
  });

  it('UPDATE with `until` and scenario context simulates rounds — literal USU', () => {
    // 1 MB literal USU per round; predicate fires when cumulative ≥ 10 MB.
    // Should take exactly 10 rounds.
    const sc = scenarioWith([literalVar('USU_TOTAL', 1_048_576)]);
    expect(
      totalRoundsFor(
        {
          ...update,
          repeat: {
            until: { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
          },
        },
        sc,
      ),
    ).toBe(10);
  });

  it('UPDATE with `until` simulates rounds with a seeded random USU', () => {
    // Random 500 KB..1 MB. Use a deterministic rng that returns 0.5 — that
    // yields the midpoint sample (~786 KB), so 10 MB / 786 KB ≈ 14 rounds.
    const sc = scenarioWith([randomVar('USU_TOTAL', 524_288, 1_048_576)]);
    const rounds = totalRoundsFor(
      {
        ...update,
        repeat: {
          until: { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
        },
      },
      sc,
      { rng: () => 0.5 },
    );
    expect(rounds).toBeGreaterThanOrEqual(13);
    expect(rounds).toBeLessThanOrEqual(15);
  });

  it('simulator falls back to DEFAULT_UNTIL_ROUNDS when the per-round variable is missing', () => {
    const sc = scenarioWith([]);
    expect(
      totalRoundsFor(
        {
          ...update,
          repeat: {
            until: { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
          },
        },
        sc,
      ),
    ).toBe(DEFAULT_UNTIL_ROUNDS);
  });

  it('simulator caps a runaway predicate at MAX_SIMULATED_ROUNDS', () => {
    // Per-round 1 byte, target 10 MB → would take 10 million rounds.
    // Should hit the safety cap.
    const sc = scenarioWith([literalVar('USU_TOTAL', 1)]);
    expect(
      totalRoundsFor(
        {
          ...update,
          repeat: {
            until: { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
          },
        },
        sc,
      ),
    ).toBe(MAX_SIMULATED_ROUNDS);
  });

  it('`times` truncates simulation when both bounds are present', () => {
    // Simulator would naturally do 10 rounds, but `times: 3` caps it.
    const sc = scenarioWith([literalVar('USU_TOTAL', 1_048_576)]);
    expect(
      totalRoundsFor(
        {
          ...update,
          repeat: {
            times: 3,
            until: { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
          },
        },
        sc,
      ),
    ).toBe(3);
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

  it('OR-list predicate fires when ANY sub-comparison fires', () => {
    // First comparison: USED_TOTAL >= 10 MB → fires at round 10.
    // Second comparison: FUI_ACTION (extracted, no per-round source)
    // is inert in the simulator. Net: 10 rounds.
    const sc = scenarioWith([literalVar('USU_TOTAL', 1_048_576)]);
    expect(
      totalRoundsFor(
        {
          ...update,
          repeat: {
            until: {
              any: [
                { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
                { variable: 'FUI_ACTION', op: 'ne', value: 'null' },
              ],
            },
          },
        },
        sc,
      ),
    ).toBe(10);
  });

  it('OR-list picks the earlier-firing comparison', () => {
    // Two thresholds against the same accumulator. `>= 5MB` fires at
    // round 5, well before `>= 10MB` would.
    const sc = scenarioWith([literalVar('USU_TOTAL', 1_048_576)]);
    expect(
      totalRoundsFor(
        {
          ...update,
          repeat: {
            until: {
              any: [
                { variable: 'USED_TOTAL', op: 'gte', value: 10_485_760 },
                { variable: 'USED_TOTAL', op: 'gte', value: 5_242_880 },
              ],
            },
          },
        },
        sc,
      ),
    ).toBe(5);
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
