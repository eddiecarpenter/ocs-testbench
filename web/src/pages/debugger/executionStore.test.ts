/**
 * State-machine tests for the page-scoped Execution Debugger store.
 *
 * The reducer is the load-bearing bit (it pins the engine's lifecycle
 * matrix); transient edit state and snapshot ingestion get a couple
 * of smoke checks.
 */
import { describe, expect, it, vi } from 'vitest';

import type { Execution } from '../../api/resources/executions';

import {
  canTransition,
  createExecutionStore,
  reduceTransition,
} from './executionStore';

describe('canTransition', () => {
  it('accepts every transition mandated by docs/ARCHITECTURE.md §6', () => {
    // pending → {running, paused, aborted, error}
    expect(canTransition('pending', 'running')).toBe(true);
    expect(canTransition('pending', 'paused')).toBe(true);
    expect(canTransition('pending', 'aborted')).toBe(true);
    expect(canTransition('pending', 'error')).toBe(true);

    // running ⇄ paused
    expect(canTransition('running', 'paused')).toBe(true);
    expect(canTransition('paused', 'running')).toBe(true);

    // running → terminal
    expect(canTransition('running', 'success')).toBe(true);
    expect(canTransition('running', 'failure')).toBe(true);
    expect(canTransition('running', 'aborted')).toBe(true);
    expect(canTransition('running', 'error')).toBe(true);

    // paused → terminal (only abort/error per the matrix; success/failure
    // must run through `running`)
    expect(canTransition('paused', 'aborted')).toBe(true);
    expect(canTransition('paused', 'error')).toBe(true);
  });

  it('rejects illegal transitions', () => {
    // Terminal states have no outbound edges
    expect(canTransition('success', 'running')).toBe(false);
    expect(canTransition('failure', 'pending')).toBe(false);
    expect(canTransition('aborted', 'paused')).toBe(false);
    expect(canTransition('error', 'success')).toBe(false);

    // paused cannot directly land on success / failure
    expect(canTransition('paused', 'success')).toBe(false);
    expect(canTransition('paused', 'failure')).toBe(false);

    // pending cannot jump to a terminal success/failure
    expect(canTransition('pending', 'success')).toBe(false);
    expect(canTransition('pending', 'failure')).toBe(false);
  });

  it('treats same-state as a permitted no-op', () => {
    // re-applying an unchanged snapshot must not warn
    expect(canTransition('running', 'running')).toBe(true);
    expect(canTransition('success', 'success')).toBe(true);
  });
});

describe('reduceTransition', () => {
  it('returns the next state for legal transitions', () => {
    const warn = vi.fn();
    expect(reduceTransition('running', 'paused', warn)).toBe('paused');
    expect(warn).not.toHaveBeenCalled();
  });

  it('returns the current state and warns on illegal transitions', () => {
    const warn = vi.fn();
    expect(reduceTransition('success', 'running', warn)).toBe('success');
    expect(warn).toHaveBeenCalledTimes(1);
    expect(warn.mock.calls[0][0]).toContain('success → running');
  });

  it('does not warn on same-state transitions', () => {
    const warn = vi.fn();
    expect(reduceTransition('paused', 'paused', warn)).toBe('paused');
    expect(warn).not.toHaveBeenCalled();
  });
});

describe('createExecutionStore', () => {
  function buildSnapshot(overrides: Partial<Execution> = {}): Execution {
    return {
      id: 'exec-1',
      scenarioId: 'scen-1',
      scenarioName: 'CCR-I happy path',
      mode: 'interactive',
      state: 'paused',
      startedAt: '2026-04-29T07:00:00Z',
      currentStep: 2,
      totalSteps: 5,
      steps: [],
      context: { system: {}, user: {}, extracted: {} },
      ...overrides,
    } as Execution;
  }

  it('seeds initial state from the executionId', () => {
    const store = createExecutionStore('exec-42');
    const s = store.getState();
    expect(s.executionId).toBe('exec-42');
    expect(s.state).toBe('pending');
    expect(s.cursor).toBe(0);
    expect(s.steps).toEqual([]);
    expect(s.historicalIndex).toBeNull();
  });

  it('ingestSnapshot drives the state machine and copies fields', () => {
    const store = createExecutionStore('exec-1');
    const snapshot = buildSnapshot();
    store.getState().ingestSnapshot(snapshot);
    const s = store.getState();
    // pending → paused is legal
    expect(s.state).toBe('paused');
    expect(s.cursor).toBe(2);
    expect(s.totalSteps).toBe(5);
  });

  it('ingestSnapshot ignores invalid transitions and keeps the prior state', () => {
    const store = createExecutionStore('exec-1');
    // Drive through running → success (legal path).
    store.getState().ingestSnapshot(buildSnapshot({ state: 'running' }));
    store.getState().ingestSnapshot(buildSnapshot({ state: 'success' }));
    expect(store.getState().state).toBe('success');

    // Subsequent snapshot trying to leave a terminal state must no-op.
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    store.getState().ingestSnapshot(buildSnapshot({ state: 'running' }));
    expect(store.getState().state).toBe('success');
    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });

  it('toggleService flips edit.servicesEnabled and sets dirty', () => {
    const store = createExecutionStore('exec-1');
    expect(store.getState().edit.dirty).toBe(false);

    store.getState().toggleService('mscc-1');
    expect(store.getState().edit.servicesEnabled.has('mscc-1')).toBe(true);
    expect(store.getState().edit.dirty).toBe(true);

    store.getState().toggleService('mscc-1');
    expect(store.getState().edit.servicesEnabled.has('mscc-1')).toBe(false);
  });

  it('viewHistorical sets the cursor pointer', () => {
    const store = createExecutionStore('exec-1');
    store.getState().viewHistorical(3);
    expect(store.getState().historicalIndex).toBe(3);
    store.getState().viewHistorical(null);
    expect(store.getState().historicalIndex).toBeNull();
  });
});
