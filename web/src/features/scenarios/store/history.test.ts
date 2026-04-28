/**
 * Tests for the bounded undo / redo history controller.
 *
 * Covers AC-14 / AC-15 / AC-16 from Feature #77 — record, undo, redo,
 * clear-on-save, and the 100-snapshot capacity.
 */
import { describe, expect, it } from 'vitest';

import { HISTORY_CAPACITY, createHistory } from './history';
import type { Scenario } from './types';

function makeScenario(name: string): Scenario {
  return {
    id: 'scn-test',
    name,
    description: '',
    unitType: 'OCTET',
    sessionMode: 'session',
    serviceModel: 'single-mscc',
    origin: 'user',
    favourite: false,
    subscriberId: 'sub-001',
    peerId: 'peer-01',
    stepCount: 0,
    updatedAt: '2026-04-28T10:00:00Z',
    avpTree: [],
    services: [],
    variables: [],
    steps: [],
  };
}

describe('createHistory', () => {
  it('starts with empty stacks and disables both undo and redo', () => {
    const h = createHistory();
    expect(h.canUndo()).toBe(false);
    expect(h.canRedo()).toBe(false);
    expect(h.undo(makeScenario('cur'))).toBeNull();
    expect(h.redo(makeScenario('cur'))).toBeNull();
  });

  it('records snapshots and supports undo to the recorded value', () => {
    const h = createHistory();
    const a = makeScenario('a');
    const b = makeScenario('b');
    h.record(a);
    expect(h.canUndo()).toBe(true);
    const popped = h.undo(b);
    expect(popped?.name).toBe('a');
    // After undo the redo stack must hold the value we held when we undid.
    expect(h.canRedo()).toBe(true);
  });

  it('redo replays the next entry and restores undo availability', () => {
    const h = createHistory();
    const a = makeScenario('a');
    const b = makeScenario('b');
    h.record(a);
    const popped = h.undo(b);
    expect(popped?.name).toBe('a');
    const replayed = h.redo(makeScenario('a'));
    expect(replayed?.name).toBe('b');
  });

  it('clear() drops both stacks (used by the Save flow)', () => {
    const h = createHistory();
    h.record(makeScenario('a'));
    h.record(makeScenario('b'));
    h.undo(makeScenario('cur'));
    expect(h.canUndo()).toBe(true);
    expect(h.canRedo()).toBe(true);
    h.clear();
    expect(h.canUndo()).toBe(false);
    expect(h.canRedo()).toBe(false);
  });

  it('record() after undo() discards the redo stack (one-branch history)', () => {
    const h = createHistory();
    h.record(makeScenario('a'));
    h.undo(makeScenario('cur'));
    expect(h.canRedo()).toBe(true);
    h.record(makeScenario('new-branch'));
    expect(h.canRedo()).toBe(false);
  });

  it('caps the undo stack at HISTORY_CAPACITY', () => {
    const h = createHistory();
    for (let i = 0; i < HISTORY_CAPACITY + 5; i += 1) {
      h.record(makeScenario(`s${i}`));
    }
    const snap = h.snapshot();
    expect(snap.undo).toHaveLength(HISTORY_CAPACITY);
    // Oldest entries are evicted from the bottom.
    expect(snap.undo[0].name).toBe('s5');
  });

  it('snapshots are deep-cloned — mutating after record does not bleed', () => {
    const h = createHistory();
    const a = makeScenario('a');
    h.record(a);
    a.name = 'mutated';
    const popped = h.undo(makeScenario('cur'));
    expect(popped?.name).toBe('a');
  });
});
