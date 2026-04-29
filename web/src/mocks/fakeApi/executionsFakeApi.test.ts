/**
 * Unit tests for the executions mock layer.
 *
 * Exercises the contract surface: list filters, force-failure path,
 * validation errors, batched-run multiplier semantics, and the rerun
 * shortcut. Runs in node — no DOM, no real network.
 */
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import type {
  ExecutionPage,
  StartExecutionInput,
  StartExecutionResult,
} from '../../api/resources/executions';
import ApiService from '../../api/ApiService';
import { mock } from '../MockAdapter';

import { __test__ } from './executionsFakeApi';

// Drop the global pass-through so unmatched routes 404 cleanly during
// tests instead of escaping out to a (non-existent) network. Without
// this, an erroneous URL would silently hang on `passThrough()`.
beforeAll(() => {
  mock.onAny().reply(404, { type: 'about:blank', title: 'Not mocked' });
});

afterEach(() => {
  __test__.reset();
});

describe('GET /executions — list filters', () => {
  it('returns every fixture by default', async () => {
    const page = await ApiService.get<ExecutionPage>('/executions');
    expect(page.items.length).toBeGreaterThan(0);
    expect(page.page.total).toBe(page.items.length);
  });

  it('filters by state (running)', async () => {
    const page = await ApiService.get<ExecutionPage>('/executions', {
      params: { state: 'running' },
    });
    expect(page.items.length).toBeGreaterThan(0);
    expect(page.items.every((e) => e.state === 'running')).toBe(true);
  });

  it('filters by scenarioId', async () => {
    const page = await ApiService.get<ExecutionPage>('/executions', {
      params: { scenarioId: 'scn-001' },
    });
    expect(page.items.every((e) => e.scenarioId === 'scn-001')).toBe(true);
  });

  it('filters by peerId', async () => {
    const page = await ApiService.get<ExecutionPage>('/executions', {
      params: { peerId: 'peer-04' },
    });
    expect(page.items.length).toBeGreaterThan(0);
    expect(page.items.every((e) => e.peerId === 'peer-04')).toBe(true);
  });

  it('combines filters with AND semantics', async () => {
    const page = await ApiService.get<ExecutionPage>('/executions', {
      params: { state: 'running', peerId: 'peer-01' },
    });
    expect(page.items.every((e) => e.state === 'running' && e.peerId === 'peer-01')).toBe(true);
  });

  it('honours limit and offset', async () => {
    const all = await ApiService.get<ExecutionPage>('/executions');
    const paged = await ApiService.get<ExecutionPage>('/executions', {
      params: { limit: 5, offset: 2 },
    });
    expect(paged.items.length).toBeLessThanOrEqual(5);
    expect(paged.items[0]?.id).toBe(all.items[2]?.id);
  });

  it('covers every (state × mode) cell required by the spec', async () => {
    const page = await ApiService.get<ExecutionPage>('/executions', {
      params: { limit: 500 },
    });
    const cell = (state: string, mode: string) =>
      page.items.some((e) => e.state === state && e.mode === mode);
    expect(cell('running', 'interactive')).toBe(true);
    expect(cell('running', 'continuous')).toBe(true);
    expect(cell('success', 'interactive')).toBe(true);
    expect(cell('success', 'continuous')).toBe(true);
    expect(cell('failure', 'interactive')).toBe(true);
    expect(cell('aborted', 'continuous')).toBe(true);
  });
});

describe('POST /executions — start', () => {
  const validInteractive: StartExecutionInput = {
    scenarioId: 'scn-octet-single-001',
    mode: 'interactive',
    concurrency: 1,
    repeats: 1,
  };

  it('returns a fresh row in `running` state, prepended onto the list', async () => {
    const before = await ApiService.get<ExecutionPage>('/executions');
    const res = await ApiService.post<StartExecutionResult, StartExecutionInput>(
      '/executions',
      validInteractive,
    );
    expect(res.items).toHaveLength(1);
    const created = res.items[0];
    expect(created.state).toBe('running');
    expect(created.scenarioId).toBe(validInteractive.scenarioId);
    expect(created.mode).toBe('interactive');
    expect(typeof created.startedAt).toBe('string');

    const after = await ApiService.get<ExecutionPage>('/executions');
    expect(after.page.total).toBe(before.page.total + 1);
    expect(after.items[0].id).toBe(created.id);
  });

  it('copies the scenario name from the scenario fixtures', async () => {
    const res = await ApiService.post<StartExecutionResult, StartExecutionInput>(
      '/executions',
      validInteractive,
    );
    expect(res.items[0].scenarioName).toBe('OCTET × single-MSCC — data session baseline');
  });

  it('honours peer / subscriber overrides', async () => {
    const res = await ApiService.post<StartExecutionResult, StartExecutionInput>(
      '/executions',
      {
        ...validInteractive,
        overrides: { peerId: 'peer-99', subscriberIds: ['sub-99'] },
      },
    );
    expect(res.items[0].peerId).toBe('peer-99');
    expect(res.items[0].subscriberId).toBe('sub-99');
  });

  it('spawns concurrency × repeats rows on continuous mode and emits a batchId', async () => {
    const res = await ApiService.post<StartExecutionResult, StartExecutionInput>(
      '/executions',
      {
        scenarioId: 'scn-octet-single-001',
        mode: 'continuous',
        concurrency: 3,
        repeats: 2,
      },
    );
    expect(res.items).toHaveLength(6);
    expect(res.batchId).toMatch(/^batch-/);
    expect(res.items.every((e) => e.state === 'running')).toBe(true);
  });

  it('rejects interactive + concurrency > 1 with 422', async () => {
    await expect(
      ApiService.post<StartExecutionResult, StartExecutionInput>('/executions', {
        scenarioId: 'scn-octet-single-001',
        mode: 'interactive',
        concurrency: 2,
        repeats: 1,
      }),
    ).rejects.toMatchObject({ status: 422 });
  });

  it('rejects missing scenarioId with 422', async () => {
    await expect(
      ApiService.post<StartExecutionResult, Partial<StartExecutionInput>>(
        '/executions',
        { mode: 'continuous', concurrency: 1, repeats: 1 },
      ),
    ).rejects.toMatchObject({ status: 422 });
  });

  it('rejects out-of-range concurrency with 422', async () => {
    await expect(
      ApiService.post<StartExecutionResult, StartExecutionInput>('/executions', {
        scenarioId: 'scn-octet-single-001',
        mode: 'continuous',
        concurrency: 11,
        repeats: 1,
      }),
    ).rejects.toMatchObject({ status: 422 });
  });

  it('returns 5xx for the force-failure scenarioId prefix', async () => {
    await expect(
      ApiService.post<StartExecutionResult, StartExecutionInput>('/executions', {
        scenarioId: 'error-force-fail',
        mode: 'continuous',
        concurrency: 1,
        repeats: 1,
      }),
    ).rejects.toMatchObject({ status: 500 });
  });
});

describe('POST /executions/:id/rerun', () => {
  it('replays an existing execution as a brand-new running row', async () => {
    const before = await ApiService.get<ExecutionPage>('/executions');
    const sourceId = '42';

    const res = await ApiService.post<StartExecutionResult>(
      `/executions/${sourceId}/rerun`,
    );

    expect(res.items).toHaveLength(1);
    const created = res.items[0];
    expect(created.id).not.toBe(sourceId);
    expect(created.state).toBe('running');

    // Inherits source's scenario / peer / mode.
    const source = before.items.find((e) => e.id === sourceId)!;
    expect(created.scenarioId).toBe(source.scenarioId);
    expect(created.mode).toBe(source.mode);
    expect(created.peerId).toBe(source.peerId);
  });

  it('returns 404 for an unknown source id', async () => {
    await expect(
      ApiService.post<StartExecutionResult>('/executions/does-not-exist/rerun'),
    ).rejects.toMatchObject({ status: 404 });
  });

  it('returns 5xx when the source row references a force-failure scenario', async () => {
    // Plant a row, then mutate the in-memory copy (not the response
    // clone — axios-mock-adapter serialises through JSON) so the
    // rerun-side `scenarioId.startsWith('error-')` guard fires.
    __test__.reset();
    const stub: StartExecutionResult = await ApiService.post<
      StartExecutionResult
    >('/executions', {
      scenarioId: 'scn-octet-single-001',
      mode: 'continuous',
      concurrency: 1,
      repeats: 1,
    } as StartExecutionInput);
    const stubId = stub.items[0].id;
    const inMemory = __test__.state.find((e) => e.id === stubId)!;
    expect(inMemory).toBeDefined();
    (inMemory as { scenarioId: string }).scenarioId = 'error-force-fail';

    await expect(
      ApiService.post<StartExecutionResult>(`/executions/${stubId}/rerun`),
    ).rejects.toMatchObject({ status: 500 });
  });
});
