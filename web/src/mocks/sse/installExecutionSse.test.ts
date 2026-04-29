/**
 * Per-execution mock SSE driver — scheduler tests.
 *
 * These tests pin the contract Task 7 will lean on: the order and
 * cadence of events, pause/resume continuity (no drift), dispose
 * cancelling pending events, and continuous mode emitting batchStep
 * increments.
 */
import { describe, expect, it, vi } from 'vitest';

import { scenarioFixtures } from '../data/scenarios';

import {
  buildPlan,
  installExecutionSse,
  type DriverEvent,
  type SchedulerLike,
} from './installExecutionSse';

/**
 * Imperatively-controlled scheduler. Tests advance it by calling
 * `advance(ms)`; the next due timeout fires whenever the cumulative
 * "now" passes its scheduled fire time.
 *
 * One `setTimeout` is supported at a time (which matches the driver's
 * usage — it never has more than one tick scheduled).
 */
function makeFakeScheduler(): SchedulerLike & {
  advance(ms: number): void;
  pending(): boolean;
  currentNow(): number;
} {
  let now = 0;
  let timer:
    | { fireAt: number; handler: () => void; cancelled: boolean }
    | null = null;

  return {
    setTimeout(handler, delay) {
      if (timer) {
        throw new Error('fake scheduler: only one timer at a time');
      }
      timer = { fireAt: now + delay, handler, cancelled: false };
      return timer;
    },
    clearTimeout(handle) {
      if (timer && handle === timer) {
        timer.cancelled = true;
        timer = null;
      }
    },
    now() {
      return now;
    },
    advance(ms: number) {
      const target = now + ms;
      // Process any timer whose fireAt is within the advance window.
      // The handler itself may call setTimeout to schedule the next
      // tick; loop until no more fire within target.
      while (timer && !timer.cancelled && timer.fireAt <= target) {
        const t = timer;
        // Step time forward to the firing instant first so handlers
        // see a consistent `now()`.
        now = t.fireAt;
        timer = null;
        t.handler();
      }
      now = target;
    },
    pending() {
      return Boolean(timer && !timer.cancelled);
    },
    currentNow() {
      return now;
    },
  };
}

const interactiveScenario = scenarioFixtures.find(
  (s) => s.serviceModel === 'single-mscc' && s.unitType === 'OCTET',
)!;

describe('buildPlan', () => {
  it('interactive: started → N×(sending,responded) → completed', () => {
    const plan = buildPlan({
      executionId: 'exec-1',
      scenario: interactiveScenario,
      mode: 'interactive',
      outcome: 'success',
      startedAtMs: 0,
      batchTotal: 3,
      tickMs: 1,
    });
    const types = plan.map((e: DriverEvent) => e.type);
    const stepCount = interactiveScenario.steps.length;
    expect(types[0]).toBe('execution.started');
    // Per step: sending then responded
    for (let i = 0; i < stepCount; i++) {
      expect(types[1 + 2 * i]).toBe('step.sending');
      expect(types[2 + 2 * i]).toBe('step.responded');
    }
    expect(types[1 + 2 * stepCount]).toBe('execution.completed');
  });

  it('interactive: failure outcome emits execution.failed terminal', () => {
    const plan = buildPlan({
      executionId: 'exec-1',
      scenario: interactiveScenario,
      mode: 'interactive',
      outcome: 'failure',
      startedAtMs: 0,
      batchTotal: 3,
      tickMs: 1,
    });
    expect(plan[plan.length - 1].type).toBe('execution.failed');
  });

  it('continuous: started → N × batchStep → completed', () => {
    const plan = buildPlan({
      executionId: 'exec-1',
      scenario: interactiveScenario,
      mode: 'continuous',
      outcome: 'success',
      startedAtMs: 0,
      batchTotal: 4,
      tickMs: 1,
    });
    const types = plan.map((e: DriverEvent) => e.type);
    expect(types[0]).toBe('execution.started');
    // Four batchStep events, then completed.
    expect(types.slice(1, 5)).toEqual([
      'execution.batchStep',
      'execution.batchStep',
      'execution.batchStep',
      'execution.batchStep',
    ]);
    expect(types[5]).toBe('execution.completed');
  });
});

describe('installExecutionSse', () => {
  it('interactive: fires execution.started on the first tick, then paces step events', () => {
    const sched = makeFakeScheduler();
    const seen: string[] = [];
    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      {
        tickMs: 100,
        scheduler: sched,
      },
    );
    handle.subscribe((e) => seen.push(e.type));

    // execution.started is scheduled on the next tick (delay 0) so
    // subscribers attached immediately after install() reliably see
    // it. Without this, the started event would fire inside install
    // before subscribe() could register, and the store would never
    // get its session-local startedAt stamp.
    expect(seen).toEqual([]);

    sched.advance(0);
    expect(seen).toEqual(['execution.started']);

    sched.advance(100);
    expect(seen[1]).toBe('step.sending');
    sched.advance(100);
    expect(seen[2]).toBe('step.responded');

    handle.dispose();
  });

  it('subscriber attached after install reliably receives execution.started', () => {
    const sched = makeFakeScheduler();
    const seen: string[] = [];

    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      {
        tickMs: 50,
        scheduler: sched,
      },
    );
    // Subscribe AFTER install — the contract guarantees we still see
    // execution.started because the driver schedules it on the first
    // tick rather than firing it synchronously inside install.
    handle.subscribe((e) => seen.push(e.type));

    sched.advance(0);
    expect(seen[0]).toBe('execution.started');

    sched.advance(50);
    expect(seen[1]).toBe('step.sending');

    handle.dispose();
  });

  it('pause then resume continues from where it left off (no drift)', () => {
    const sched = makeFakeScheduler();
    const seen: string[] = [];
    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      {
        tickMs: 100,
        scheduler: sched,
      },
    );
    handle.subscribe((e) => seen.push(e.type));

    // Drain the leading execution.started (scheduled on tick 0).
    sched.advance(0);
    expect(seen).toEqual(['execution.started']);
    seen.length = 0;

    // 30 ms in (70 ms remaining on the first paced tick), pause.
    sched.advance(30);
    expect(seen).toEqual([]);
    expect(sched.pending()).toBe(true);
    handle.pause();
    expect(sched.pending()).toBe(false);

    // Time passes while paused — no events fire.
    sched.advance(500);
    expect(seen).toEqual([]);

    // Resume; the leftover 70 ms should fire the first event, then
    // the next 100 ms tick fires the second.
    handle.resume();
    sched.advance(70);
    expect(seen).toEqual(['step.sending']);
    sched.advance(100);
    expect(seen).toEqual(['step.sending', 'step.responded']);

    handle.dispose();
  });

  it('dispose mid-run cancels remaining events', () => {
    const sched = makeFakeScheduler();
    const seen: string[] = [];
    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      {
        tickMs: 100,
        scheduler: sched,
      },
    );
    handle.subscribe((e) => seen.push(e.type));

    sched.advance(0);
    expect(seen).toEqual(['execution.started']);
    seen.length = 0;

    sched.advance(100);
    expect(seen).toEqual(['step.sending']);

    handle.dispose();
    sched.advance(10_000);
    // No new events emitted after dispose.
    expect(seen).toEqual(['step.sending']);
  });

  it('continuous mode emits batchStep until repeat count', () => {
    const sched = makeFakeScheduler();
    const seen: DriverEvent[] = [];
    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'continuous',
      {
        tickMs: 50,
        batchTotal: 3,
        scheduler: sched,
      },
    );
    handle.subscribe((e) => seen.push(e));

    // Drain leading execution.started.
    sched.advance(0);
    expect(seen[0].type).toBe('execution.started');
    seen.length = 0;

    // Fire all 3 batchStep events.
    sched.advance(50);
    sched.advance(50);
    sched.advance(50);
    expect(seen.map((e) => e.type)).toEqual([
      'execution.batchStep',
      'execution.batchStep',
      'execution.batchStep',
    ]);
    // Then completed.
    sched.advance(50);
    expect(seen[seen.length - 1].type).toBe('execution.completed');

    handle.dispose();
  });

  it('multiple subscribers receive the same events', () => {
    const sched = makeFakeScheduler();
    const a: string[] = [];
    const b: string[] = [];
    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      {
        tickMs: 10,
        scheduler: sched,
      },
    );
    handle.subscribe((e) => a.push(e.type));
    handle.subscribe((e) => b.push(e.type));

    sched.advance(0); // drain started
    a.length = 0;
    b.length = 0;

    sched.advance(10);
    expect(a).toEqual(['step.sending']);
    expect(b).toEqual(['step.sending']);

    handle.dispose();
  });

  it('subscribe returns an unsubscribe fn', () => {
    const sched = makeFakeScheduler();
    const seen: string[] = [];
    const handle = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      {
        tickMs: 10,
        scheduler: sched,
      },
    );
    const off = handle.subscribe((e) => seen.push(e.type));
    sched.advance(0); // drain started
    seen.length = 0;
    sched.advance(10);
    expect(seen.length).toBe(1);
    off();
    sched.advance(10);
    expect(seen.length).toBe(1); // no further pushes after off()

    handle.dispose();
  });

  it('pause flag is scoped to the driver instance — pausing one does not affect another', () => {
    const sched1 = makeFakeScheduler();
    const sched2 = makeFakeScheduler();
    const seen1: string[] = [];
    const seen2: string[] = [];

    const h1 = installExecutionSse(
      'exec-1',
      interactiveScenario,
      'interactive',
      { tickMs: 10, scheduler: sched1 },
    );
    const h2 = installExecutionSse(
      'exec-2',
      interactiveScenario,
      'interactive',
      { tickMs: 10, scheduler: sched2 },
    );
    h1.subscribe((e) => seen1.push(e.type));
    h2.subscribe((e) => seen2.push(e.type));

    h1.pause();
    sched1.advance(100);
    sched2.advance(0); // drain started on h2
    sched2.advance(10);
    seen2.splice(0, seen2.indexOf('step.sending')); // drop leading started
    expect(seen1).toEqual([]); // h1 paused before its first tick fired
    expect(seen2).toEqual(['step.sending']); // h2 still running

    // Vitest will catch leaked timers
    h1.dispose();
    h2.dispose();
  });

  it('using vi.useFakeTimers() integration smoke test', () => {
    vi.useFakeTimers();
    try {
      const seen: string[] = [];
      const handle = installExecutionSse(
        'exec-1',
        interactiveScenario,
        'interactive',
        { tickMs: 100 },
      );
      handle.subscribe((e) => seen.push(e.type));
      vi.advanceTimersByTime(100);
      expect(seen).toEqual(['execution.started', 'step.sending']);
      handle.dispose();
    } finally {
      vi.useRealTimers();
    }
  });
});
