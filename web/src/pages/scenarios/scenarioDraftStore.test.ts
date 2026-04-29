/**
 * Tests for the scenario draft store — focused on the dirty-flag /
 * history interaction, which has had subtle bugs:
 *
 *   - undo back to the loaded baseline must clear `dirty`
 *   - redo always re-introduces a step ahead of saved → `dirty` true
 *   - load / markSaved reset both the dirty flag and history
 */
import { beforeEach, describe, expect, it } from 'vitest';

import { useScenarioDraftStore } from './scenarioDraftStore';
import type { Scenario } from './types';

function makeScenario(name = 'baseline'): Scenario {
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

describe('scenarioDraftStore — dirty-flag lifecycle', () => {
  beforeEach(() => {
    // Reset to a clean store between tests.
    useScenarioDraftStore.getState().reset();
  });

  it('load() seeds the draft with dirty=false and empty history', () => {
    const s = useScenarioDraftStore.getState();
    s.load(makeScenario());
    const after = useScenarioDraftStore.getState();
    expect(after.dirty).toBe(false);
    expect(after.canUndo()).toBe(false);
    expect(after.canRedo()).toBe(false);
  });

  it('a single mutation sets dirty=true and enables undo', () => {
    const s = useScenarioDraftStore.getState();
    s.load(makeScenario());
    s.setName('renamed');
    const after = useScenarioDraftStore.getState();
    expect(after.dirty).toBe(true);
    expect(after.draft?.name).toBe('renamed');
    expect(after.canUndo()).toBe(true);
  });

  it('undoing every mutation clears the dirty flag', () => {
    const s = useScenarioDraftStore.getState();
    s.load(makeScenario());
    s.setName('first edit');
    s.setName('second edit');
    s.setName('third edit');
    expect(useScenarioDraftStore.getState().dirty).toBe(true);

    // After the first undo we're still dirty (two more ahead of saved).
    s.undo();
    expect(useScenarioDraftStore.getState().dirty).toBe(true);
    s.undo();
    expect(useScenarioDraftStore.getState().dirty).toBe(true);

    // Final undo lands us on the baseline → dirty must clear.
    s.undo();
    const final = useScenarioDraftStore.getState();
    expect(final.dirty).toBe(false);
    expect(final.draft?.name).toBe('baseline');
    expect(final.canUndo()).toBe(false);
    expect(final.canRedo()).toBe(true);
  });

  it('redo re-introduces dirty=true', () => {
    const s = useScenarioDraftStore.getState();
    s.load(makeScenario());
    s.setName('edit 1');
    s.undo();
    expect(useScenarioDraftStore.getState().dirty).toBe(false);

    s.redo();
    const after = useScenarioDraftStore.getState();
    expect(after.dirty).toBe(true);
    expect(after.draft?.name).toBe('edit 1');
    expect(after.canRedo()).toBe(false);
  });

  it('markSaved clears dirty + history (both stacks)', () => {
    const s = useScenarioDraftStore.getState();
    s.load(makeScenario());
    s.setName('edit 1');
    s.setName('edit 2');
    s.undo(); // some redo entry exists
    s.markSaved({ ...makeScenario(), name: 'edit 1', id: 'scn-test' });

    const after = useScenarioDraftStore.getState();
    expect(after.dirty).toBe(false);
    expect(after.canUndo()).toBe(false);
    expect(after.canRedo()).toBe(false);
    expect(after.draft?.name).toBe('edit 1');
  });
});
