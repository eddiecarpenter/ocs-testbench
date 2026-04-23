import type { ExecutionSummary } from '../api/resources/executions';
import type { Peer } from '../api/resources/peers';
import type { EventStreamLike } from '../api/sse/transport';
import { setEventStreamFactory } from '../api/sse/transport';
import { buildExecutionDetail, TOTAL_STEPS } from './data/executionDetails';
import { executionFixtures } from './data/executions';
import { peerFixtures } from './data/peers';
import { scenarioFixtures } from './data/scenarios';
import { subscriberFixtures } from './data/subscribers';

/** Tick cadence — fast enough to see the UI breathe, slow enough to read. */
const EXECUTION_TICK_MS = 2_000;
const PEER_FLIP_MS = 15_000;
const KPI_TICK_MS = 5_000;

/**
 * In-memory `EventSource`-compatible emitter backed by timers. Registered
 * as the SSE transport when `VITE_USE_MOCK_API` is on so the rest of the
 * app can `new EventSource(...)` without caring whether it's real or not.
 */
class MockEventSource implements EventStreamLike {
  private readonly listeners = new Map<string, Set<(ev: MessageEvent) => void>>();
  private _readyState = 0;
  private openTimer: ReturnType<typeof setTimeout> | undefined;
  private readonly detach: () => void;

  constructor() {
    emitter.attach(this);
    this.detach = () => emitter.detach(this);
    // Defer "open" by a tick so listeners can subscribe first.
    this.openTimer = setTimeout(() => {
      this._readyState = 1;
      this.dispatch('open', undefined);
      emitter.sendInitialSnapshot(this);
    }, 0);
  }

  get readyState(): number {
    return this._readyState;
  }

  addEventListener(type: string, listener: (ev: MessageEvent) => void): void {
    let set = this.listeners.get(type);
    if (!set) {
      set = new Set();
      this.listeners.set(type, set);
    }
    set.add(listener);
  }

  removeEventListener(
    type: string,
    listener: (ev: MessageEvent) => void,
  ): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    if (this.openTimer) clearTimeout(this.openTimer);
    this.openTimer = undefined;
    this._readyState = 2;
    this.listeners.clear();
    this.detach();
  }

  /** Called by the emitter to push a typed event to this subscriber. */
  dispatch(type: string, data: unknown): void {
    const set = this.listeners.get(type);
    if (!set || set.size === 0) return;
    const payload = data === undefined ? '' : JSON.stringify(data);
    const ev = new MessageEvent(type, { data: payload });
    for (const listener of set) listener(ev);
  }
}

/**
 * Shared tick/broadcast hub. One instance per tab. Holds the current mock
 * state (peer flip cycle, running execution progress) and fans every
 * generated event out to every connected `MockEventSource`.
 */
class MockSseEmitter {
  private readonly subscribers = new Set<MockEventSource>();
  private peerTimer: ReturnType<typeof setInterval> | undefined;
  private execTimer: ReturnType<typeof setInterval> | undefined;
  private kpiTimer: ReturnType<typeof setInterval> | undefined;
  /** Working copy of peers — mutated as status flips. */
  private peers: Peer[] = peerFixtures.map((p) => ({ ...p }));
  /** Working copy of executions — mutated as runs advance / finish. */
  private executions: ExecutionSummary[] = executionFixtures.map((e) => ({
    ...e,
  }));
  /** Per-running-execution progress counter (completed step count). */
  private execProgress = new Map<string, number>([
    ['43', 4],
    ['44', 1],
  ]);
  /** Reusable toggle so peer-03 oscillates between error / connected. */
  private peerFlipIsError = true;

  attach(source: MockEventSource): void {
    this.subscribers.add(source);
    if (this.subscribers.size === 1) this.startTimers();
  }

  detach(source: MockEventSource): void {
    this.subscribers.delete(source);
    if (this.subscribers.size === 0) this.stopTimers();
  }

  /**
   * Push the current state of everything to a just-connected subscriber so
   * its caches are seeded immediately without waiting for the first tick.
   */
  sendInitialSnapshot(source: MockEventSource): void {
    // No replay needed for MVP — REST already populated the caches. We
    // still send a single `dashboard.kpi` so the "live" status indicator
    // flips to OPEN with something visible in dev tools. Keep minimal.
    source.dispatch('dashboard.kpi', this.computeKpis());
  }

  private startTimers(): void {
    this.peerTimer = setInterval(() => this.flipPeer(), PEER_FLIP_MS);
    this.execTimer = setInterval(() => this.tickExecutions(), EXECUTION_TICK_MS);
    this.kpiTimer = setInterval(() => this.broadcastKpis(), KPI_TICK_MS);
  }

  private stopTimers(): void {
    if (this.peerTimer) clearInterval(this.peerTimer);
    if (this.execTimer) clearInterval(this.execTimer);
    if (this.kpiTimer) clearInterval(this.kpiTimer);
    this.peerTimer = this.execTimer = this.kpiTimer = undefined;
  }

  private broadcast(type: string, data: unknown): void {
    for (const sub of this.subscribers) sub.dispatch(type, data);
  }

  /** Flip peer-03 between error/connected so the dashboard status breathes. */
  private flipPeer(): void {
    const peer = this.peers.find((p) => p.id === 'peer-03');
    if (!peer) return;
    this.peerFlipIsError = !this.peerFlipIsError;
    peer.status = this.peerFlipIsError ? 'error' : 'connected';
    peer.statusDetail = this.peerFlipIsError ? 'CER/CEA timeout' : undefined;
    peer.lastChangeAt = new Date().toISOString();
    this.broadcast('peer.updated', peer);
    this.broadcast('dashboard.kpi', this.computeKpis());
  }

  /** Advance every running execution by one step; terminate at TOTAL_STEPS. */
  private tickExecutions(): void {
    for (const exec of this.executions) {
      if (exec.result !== 'running') continue;
      const soFar = this.execProgress.get(exec.id) ?? 0;
      const next = soFar + 1;

      if (next >= TOTAL_STEPS) {
        // Terminal transition — mark success and emit a final progress
        // event carrying the completed state.
        exec.result = 'success';
        exec.finishedAt = new Date().toISOString();
        this.execProgress.set(exec.id, TOTAL_STEPS);
        const detail = buildExecutionDetail(exec.id, TOTAL_STEPS);
        if (detail) this.broadcast('execution.progress', detail);
        this.broadcast('dashboard.kpi', this.computeKpis());
      } else {
        this.execProgress.set(exec.id, next);
        const detail = buildExecutionDetail(exec.id, next);
        if (detail) this.broadcast('execution.progress', detail);
      }
    }
  }

  private broadcastKpis(): void {
    this.broadcast('dashboard.kpi', this.computeKpis());
  }

  private computeKpis() {
    const connected = this.peers.filter((p) => p.status === 'connected').length;
    const activeRuns = this.executions.filter((e) => e.result === 'running')
      .length;
    return {
      peers: { connected, total: this.peers.length },
      subscribers: subscriberFixtures.length,
      scenarios: scenarioFixtures.length,
      activeRuns,
    };
  }
}

const emitter = new MockSseEmitter();

/** Install the mock SSE factory. Called from `mocks/index.ts`. */
export function installMockSse(): void {
  setEventStreamFactory(() => new MockEventSource());
}
