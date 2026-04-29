/**
 * Execution mock handlers.
 *
 * Mirrors the existing peers / subscribers / scenarios pattern: a
 * single working copy of the fixture data is mutated by every
 * write-side handler so POST / re-run rows survive within the page
 * session. A hard refresh re-seeds from `executionFixtures`.
 *
 * Implements:
 *   GET    /executions                       — list (filterable: state,
 *                                              scenarioId, peerId, batchId,
 *                                              limit, offset)
 *   GET    /executions/:id                   — detail
 *   POST   /executions                       — start (returns
 *                                              StartExecutionResult)
 *   POST   /executions/:id/rerun             — replay an existing run with
 *                                              the same parameters
 *
 * The `peerId` filter is mock-layer-only (OpenAPI v0.2 does not yet
 * define a `peerId` query param on `GET /executions`); see
 * `api/resources/executions.ts` for the corresponding note.
 *
 * (Reuse: refactor — replaced the read-only `executionFixtures` import
 * with a mutable working copy and absorbed the dropped POST stub from
 * `scenariosFakeApi.ts` into a contract-faithful handler.)
 */
import type { components } from '../../api/schema';
import type {
  Execution,
  ExecutionPage,
  ExecutionState,
  ExecutionMode,
  ExecutionSummary,
  StartExecutionInput,
  StartExecutionResult,
} from '../../api/resources/executions';
import { buildExecutionDetail } from '../data/executionDetails';
import {
  executionFixtures,
  FORCE_FAILURE_SCENARIO_PREFIX,
} from '../data/executions';
import { scenarioFixtures } from '../data/scenarios';
import { mock } from '../MockAdapter';

/** RFC 7807 problem shape per OpenAPI v0.2. */
type ProblemBody = components['schemas']['Problem'];

/**
 * Working copy. POST / rerun handlers prepend onto this array so the
 * "newest first" ordering on the list view is naturally produced
 * without sorting client-side.
 */
const executions: ExecutionSummary[] = executionFixtures.map((e) => ({
  ...e,
}));

type FieldErrors = Record<string, string[]>;

function safeParse<T>(data: unknown): T | undefined {
  if (typeof data === 'string') {
    try {
      return JSON.parse(data) as T;
    } catch {
      return undefined;
    }
  }
  return data as T | undefined;
}

function validationProblem(errors: FieldErrors): [number, ProblemBody] {
  return [
    422,
    {
      type: 'about:blank',
      title: 'Validation failed',
      status: 422,
      detail: 'One or more fields are invalid',
      errors,
    },
  ];
}

function notFound(id: string): [number, ProblemBody] {
  return [
    404,
    {
      type: 'about:blank',
      title: 'Execution not found',
      status: 404,
      detail: `No execution with id "${id}"`,
    },
  ];
}

function forcedFailure(scenarioId: string): [number, ProblemBody] {
  return [
    500,
    {
      type: 'about:blank',
      title: 'Forced failure',
      status: 500,
      detail: `Scenario id "${scenarioId}" begins with "${FORCE_FAILURE_SCENARIO_PREFIX}" — mock returns 5xx.`,
    },
  ];
}

const VALID_MODES: ExecutionMode[] = ['interactive', 'continuous'];

/** Validate a `StartExecutionInput` payload. Returns `null` if all fields are OK. */
function validateStart(
  input: Partial<StartExecutionInput> | undefined,
): FieldErrors | null {
  const errors: FieldErrors = {};
  const scenarioId = input?.scenarioId?.trim();
  const mode = input?.mode;
  const concurrency = input?.concurrency;
  const repeats = input?.repeats;

  if (!scenarioId) errors['/scenarioId'] = ['Scenario id is required'];

  if (!mode || !VALID_MODES.includes(mode)) {
    errors['/mode'] = ['Mode must be "interactive" or "continuous"'];
  }

  // Concurrency
  if (typeof concurrency !== 'number' || !Number.isInteger(concurrency)) {
    errors['/concurrency'] = ['Concurrency is required'];
  } else if (concurrency < 1 || concurrency > 10) {
    errors['/concurrency'] = ['Concurrency must be between 1 and 10'];
  }

  // Repeats
  if (typeof repeats !== 'number' || !Number.isInteger(repeats)) {
    errors['/repeats'] = ['Repeats is required'];
  } else if (repeats < 1 || repeats > 1000) {
    errors['/repeats'] = ['Repeats must be between 1 and 1000'];
  }

  // Mode/multiplier compatibility — interactive mode forbids batched runs
  // per the OpenAPI v0.2 description on `StartExecutionInput.concurrency`.
  if (
    mode === 'interactive' &&
    ((typeof concurrency === 'number' && concurrency > 1) ||
      (typeof repeats === 'number' && repeats > 1))
  ) {
    errors['/mode'] = [
      'Interactive mode requires concurrency = 1 and repeats = 1',
    ];
  }

  return Object.keys(errors).length > 0 ? errors : null;
}

/** Build a fresh `ExecutionSummary` from a validated start input. */
function makeExecution(
  input: StartExecutionInput,
  index: number,
): ExecutionSummary {
  const scenario = scenarioFixtures.find((s) => s.id === input.scenarioId);
  const peerId = input.overrides?.peerId ?? scenario?.peerId;
  const subscriberId =
    input.overrides?.subscriberIds?.[index % Math.max(1, input.overrides?.subscriberIds?.length ?? 1)] ??
    scenario?.subscriberId;

  const summary: ExecutionSummary = {
    id: `exec-${Date.now()}-${index}`,
    scenarioId: input.scenarioId,
    scenarioName: scenario?.name ?? input.scenarioId,
    mode: input.mode,
    state: 'running',
    startedAt: new Date().toISOString(),
  };
  if (peerId) {
    summary.peerId = peerId;
    summary.peerName = peerId;
  }
  if (subscriberId) {
    summary.subscriberId = subscriberId;
  }
  return summary;
}

/**
 * Construct a `StartExecutionResult` from an input. Spawns
 * `concurrency * repeats` rows in `continuous` mode, exactly one in
 * `interactive` mode (validation already enforces the multipliers).
 */
function startNew(input: StartExecutionInput): StartExecutionResult {
  const total =
    input.mode === 'continuous'
      ? Math.max(1, input.concurrency) * Math.max(1, input.repeats)
      : 1;
  const items: ExecutionSummary[] = [];
  for (let i = 0; i < total; i++) {
    items.push(makeExecution(input, i));
  }
  // Newest first — prepend each row.
  for (const it of items) executions.unshift(it);

  return total > 1 ? { batchId: `batch-${Date.now()}`, items } : { items };
}

// ---------------------------------------------------------------------------
// LIST — supports state, scenarioId, peerId, batchId, limit, offset.
// Regex anchors so /executions/:id and /executions/:id/rerun never match.
// ---------------------------------------------------------------------------
mock
  .onGet(/\/executions(\?|$)/)
  .withDelayInMs(250)
  .reply((config): [number, ExecutionPage] => {
    const params = (config.params ?? {}) as Record<string, unknown>;
    const stateFilter = params.state as ExecutionState | undefined;
    const scenarioFilter = params.scenarioId as string | undefined;
    const peerFilter = params.peerId as string | undefined;
    const batchFilter = params.batchId as string | undefined;
    const limit = Math.min(500, Math.max(1, Number(params.limit ?? 50)));
    const offset = Math.max(0, Number(params.offset ?? 0));

    const filtered = executions.filter((e) => {
      if (stateFilter && e.state !== stateFilter) return false;
      if (scenarioFilter && e.scenarioId !== scenarioFilter) return false;
      if (peerFilter && e.peerId !== peerFilter) return false;
      if (batchFilter && e.batchId !== batchFilter) return false;
      return true;
    });

    const items = filtered.slice(offset, offset + limit);
    return [
      200,
      {
        items,
        page: { total: filtered.length, limit, offset },
      },
    ];
  });

// ---------------------------------------------------------------------------
// DETAIL — /executions/{id}
// ---------------------------------------------------------------------------
mock
  .onGet(/\/executions\/[^/]+$/)
  .withDelayInMs(200)
  .reply((config): [number, Execution | ProblemBody] => {
    const url = config.url ?? '';
    const id = decodeURIComponent(url.split('/').pop() ?? '');
    const detail = buildExecutionDetail(id);
    if (!detail) return notFound(id);
    return [200, detail];
  });

// ---------------------------------------------------------------------------
// START — POST /executions
// ---------------------------------------------------------------------------
mock
  .onPost(/\/executions$/)
  .withDelayInMs(250)
  .reply((config): [number, StartExecutionResult | ProblemBody] => {
    const input = safeParse<StartExecutionInput>(config.data);
    if (!input) {
      return [
        422,
        {
          type: 'about:blank',
          title: 'Validation failed',
          status: 422,
          detail: 'Body could not be parsed as StartExecutionInput',
        },
      ];
    }

    if (input.scenarioId?.startsWith(FORCE_FAILURE_SCENARIO_PREFIX)) {
      return forcedFailure(input.scenarioId);
    }

    const errors = validateStart(input);
    if (errors) return validationProblem(errors);

    return [201, startNew(input as StartExecutionInput)];
  });

// ---------------------------------------------------------------------------
// RERUN — POST /executions/:id/rerun
// ---------------------------------------------------------------------------
mock
  .onPost(/\/executions\/[^/]+\/rerun$/)
  .withDelayInMs(250)
  .reply((config): [number, StartExecutionResult | ProblemBody] => {
    const m = /\/executions\/([^/]+)\/rerun$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const source = executions.find((e) => e.id === id);
    if (!source) return notFound(id);

    if (source.scenarioId.startsWith(FORCE_FAILURE_SCENARIO_PREFIX)) {
      return forcedFailure(source.scenarioId);
    }

    // Re-runs adopt the source's mode and peer/subscriber but always
    // start at concurrency=1, repeats=1 — the rationale calls for a
    // "silent" re-run of the source's parameters, and the mock has no
    // record of the original concurrency / repeats inputs.
    const input: StartExecutionInput = {
      scenarioId: source.scenarioId,
      mode: source.mode,
      concurrency: 1,
      repeats: 1,
      ...(source.peerId || source.subscriberId
        ? {
            overrides: {
              ...(source.peerId ? { peerId: source.peerId } : {}),
              ...(source.subscriberId
                ? { subscriberIds: [source.subscriberId] }
                : {}),
            },
          }
        : {}),
    };

    return [201, startNew(input)];
  });

// ---------------------------------------------------------------------------
// ABORT — POST /executions/:id/abort
// ---------------------------------------------------------------------------
mock
  .onPost(/\/executions\/[^/]+\/abort$/)
  .withDelayInMs(150)
  .reply((config): [number, void | ProblemBody] => {
    const m = /\/executions\/([^/]+)\/abort$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const idx = executions.findIndex((e) => e.id === id);
    if (idx === -1) return notFound(id);
    // Flip the row to `aborted` and stamp `finishedAt` if not already
    // terminal — mirrors the engine's transition contract.
    const next: ExecutionSummary = {
      ...executions[idx],
      state: 'aborted',
      finishedAt: executions[idx].finishedAt ?? new Date().toISOString(),
    };
    executions[idx] = next;
    return [204, undefined];
  });

// ---------------------------------------------------------------------------
// Test seam — exposed only to the unit tests so they can reseed the
// working copy between cases. Must NOT be imported from production code.
// ---------------------------------------------------------------------------
export const __test__ = {
  reset(): void {
    executions.splice(
      0,
      executions.length,
      ...executionFixtures.map((e) => ({ ...e })),
    );
  },
  get state(): ReadonlyArray<ExecutionSummary> {
    return executions;
  },
};
