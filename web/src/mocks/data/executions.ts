import type { ExecutionSummary } from '../../api/resources/executions';
import { scenarioFixtures } from './scenarios';

/** Reference "now" — keep fixtures deterministic. */
const NOW = Date.parse('2026-04-21T09:15:00Z');
const sec = 1_000;
const min = 60 * sec;

function iso(msAgo: number): string {
  return new Date(NOW - msAgo).toISOString();
}

/**
 * Stable index into the scenario fixtures so execution rows always
 * reference real scenarios. Cycles through whatever scenarios exist
 * — keeps the sidebar counts honest no matter how many scenarios
 * the fixture file declares.
 */
function scenario(i: number): { id: string; name: string } {
  const s = scenarioFixtures[i % scenarioFixtures.length];
  return { id: s.id, name: s.name };
}

/**
 * 32 recent executions, newest first. 2 are still running — matches the
 * "Active runs: 2" KPI on the dashboard.
 *
 * Coverage: at least one execution per (state × mode) combination the
 * Executions list filter chips and Mode column need to surface — see
 * Task #96 acceptance criteria for the canonical six cells:
 *
 *   running     × interactive    → id 44
 *   running     × continuous     → id 43
 *   success     × interactive    → ids 41, 39
 *   success     × continuous     → ids 42, 38
 *   failure     × interactive    → id 45
 *   failure     × continuous     → id 40
 *   aborted     × continuous     → id 46  (user-stopped run)
 */
export const executionFixtures: ExecutionSummary[] = [
  // 46 — aborted × continuous
  {
    id: '46',
    ...mapName(scenario(0)),
    mode: 'continuous',
    peerId: 'peer-02',
    peerName: 'peer-02',
    state: 'aborted',
    startedAt: iso(45 * sec),
    finishedAt: iso(20 * sec),
  },
  // 45 — failure × interactive
  {
    id: '45',
    ...mapName(scenario(1)),
    mode: 'interactive',
    peerId: 'peer-02',
    peerName: 'peer-02',
    state: 'failure',
    startedAt: iso(2 * min),
    finishedAt: iso(90 * sec),
  },
  // 42 — success × continuous
  {
    id: '42',
    ...mapName(scenario(2)),
    mode: 'continuous',
    peerId: 'peer-01',
    peerName: 'peer-01',
    state: 'success',
    startedAt: iso(12 * sec),
    finishedAt: iso(2 * sec),
  },
  // 41 — success × interactive
  {
    id: '41',
    ...mapName(scenario(3)),
    mode: 'interactive',
    peerId: 'peer-01',
    peerName: 'peer-01',
    state: 'success',
    startedAt: iso(1 * min),
    finishedAt: iso(45 * sec),
  },
  // 40 — failure × continuous
  {
    id: '40',
    ...mapName(scenario(0)),
    mode: 'continuous',
    peerId: 'peer-02',
    peerName: 'peer-02',
    state: 'failure',
    startedAt: iso(5 * min),
    finishedAt: iso(4 * min),
  },
  // 39 — success × interactive
  {
    id: '39',
    ...mapName(scenario(1)),
    mode: 'interactive',
    peerId: 'peer-01',
    peerName: 'peer-01',
    state: 'success',
    startedAt: iso(8 * min),
    finishedAt: iso(7 * min),
  },
  // 38 — success × continuous
  {
    id: '38',
    ...mapName(scenario(4)),
    mode: 'continuous',
    peerId: 'peer-04',
    peerName: 'peer-04',
    state: 'success',
    startedAt: iso(15 * min),
    finishedAt: iso(13 * min),
  },
  // Active runs (no finishedAt) — cover both modes
  // 43 — running × continuous
  {
    id: '43',
    ...mapName(scenario(5)),
    mode: 'continuous',
    peerId: 'peer-04',
    peerName: 'peer-04',
    state: 'running',
    startedAt: iso(30 * sec),
  },
  // 44 — running × interactive
  {
    id: '44',
    ...mapName(scenario(2)),
    mode: 'interactive',
    peerId: 'peer-01',
    peerName: 'peer-01',
    state: 'running',
    startedAt: iso(3 * sec),
  },
  // Older history
  ...Array.from({ length: 23 }, (_, i): ExecutionSummary => {
    const id = String(37 - i);
    const startMin = 20 + i * 3;
    const scn = scenario(i);
    return {
      id,
      scenarioId: scn.id,
      scenarioName: scn.name,
      mode: i % 2 === 0 ? 'continuous' : 'interactive',
      peerId: `peer-0${1 + (i % 4)}`,
      peerName: `peer-0${1 + (i % 4)}`,
      state: i % 7 === 3 ? 'failure' : 'success',
      startedAt: iso(startMin * min),
      finishedAt: iso((startMin - 1) * min),
    };
  }),
];

/** Spread helper to widen `{id,name}` into `{scenarioId, scenarioName}`. */
function mapName(scn: { id: string; name: string }) {
  return { scenarioId: scn.id, scenarioName: scn.name };
}

/**
 * Reserved scenario-id prefix used by `POST /executions` to exercise
 * the failure path. Any start with `scenarioId.startsWith('error-')`
 * returns 5xx — see `executionsFakeApi.ts`.
 */
export const FORCE_FAILURE_SCENARIO_PREFIX = 'error-';
