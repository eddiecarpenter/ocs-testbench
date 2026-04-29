/**
 * Tests for the Export-run JSON shape.
 *
 * Pin the contract: a future change to the schema must surface here
 * (and bump `exportVersion`) — silent shape drift would corrupt
 * downstream consumers (analysts, CI checks, etc.).
 */
import { describe, expect, it } from 'vitest';

import type {
  Execution,
  StepRecord,
} from '../../api/resources/executions';

import {
  buildExportPayload,
  exportFilename,
  exportToString,
} from './exportRun';

function step(
  i: number,
  state: StepRecord['state'],
  extras: Partial<StepRecord> = {},
): StepRecord {
  return {
    n: i + 1,
    kind: 'request',
    state,
    durationMs: 100,
    response: { resultCode: 2001 },
    ...extras,
  };
}

function snapshot(overrides: Partial<Execution> = {}): Execution {
  return {
    id: 'exec-1',
    scenarioId: 'scn-1',
    scenarioName: 'CCR happy path',
    mode: 'interactive',
    state: 'success',
    startedAt: '2026-04-29T07:00:00Z',
    finishedAt: '2026-04-29T07:00:30Z',
    currentStep: 3,
    totalSteps: 3,
    steps: [
      step(0, 'success'),
      step(1, 'success'),
      step(2, 'success', { response: { resultCode: 2001, extractions: { X: 'y' } } }),
    ],
    context: {
      system: { SESSION_ID: 'abc' },
      user: { MSISDN: '27821234567' },
      extracted: { GRANTED_TOTAL: 1024 },
    },
    ...overrides,
  } as Execution;
}

describe('buildExportPayload', () => {
  it('produces the v1 export shape', () => {
    const payload = buildExportPayload(snapshot());
    expect(payload.exportVersion).toBe(1);
    expect(payload.execution.id).toBe('exec-1');
    expect(payload.execution.scenarioName).toBe('CCR happy path');
    expect(payload.execution.state).toBe('success');
    expect(payload.steps).toHaveLength(3);
    expect(payload.responses).toHaveLength(3);
    expect(payload.extractedVariables).toEqual({ GRANTED_TOTAL: 1024 });
  });

  it('totals roll up step counts and durations', () => {
    const payload = buildExportPayload(
      snapshot({
        steps: [
          step(0, 'success'),
          step(1, 'failure'),
          step(2, 'skipped', { durationMs: 50 }),
        ],
      }),
    );
    expect(payload.totals).toEqual({
      steps: 3,
      passed: 1,
      failed: 1,
      skipped: 1,
      durationMs: 100 + 100 + 50,
    });
  });

  it('strips optional fields when absent', () => {
    const payload = buildExportPayload(
      snapshot({
        finishedAt: undefined,
        peerName: undefined,
        subscriberMsisdn: undefined,
      }),
    );
    expect(payload.execution.finishedAt).toBeUndefined();
    expect(payload.execution.peerName).toBeUndefined();
  });

  it('responses preserve per-step payloads keyed by step n', () => {
    const payload = buildExportPayload(snapshot());
    expect(payload.responses[2]).toEqual({
      stepN: 3,
      response: { resultCode: 2001, extractions: { X: 'y' } },
    });
  });
});

describe('exportFilename', () => {
  it('renders execution-<id>.json', () => {
    expect(exportFilename('exec-42')).toBe('execution-exec-42.json');
  });
});

describe('exportToString', () => {
  it('produces valid JSON parseable round-trip', () => {
    const payload = buildExportPayload(snapshot());
    const json = exportToString(payload);
    expect(() => JSON.parse(json)).not.toThrow();
    const parsed = JSON.parse(json);
    expect(parsed.execution.id).toBe('exec-1');
  });

  it('uses 2-space indent for readability', () => {
    const payload = buildExportPayload(snapshot());
    const json = exportToString(payload);
    expect(json).toContain('\n  "execution"');
  });
});
