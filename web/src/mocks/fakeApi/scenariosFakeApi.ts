/**
 * Scenario mock handlers.
 *
 * The project does not run MSW; it uses `axios-mock-adapter` against
 * the shared Axios base (see `src/mocks/MockAdapter.ts`). The design
 * plan called for "MSW handlers" — the closest equivalent in this
 * codebase is this set of adapter handlers, registered at the same
 * shape as the existing peer / subscriber / dashboard handlers
 * (Reuse: refactor — extends the project's axios-mock-adapter pattern
 * rather than introducing a parallel mock framework).
 *
 * Implements:
 *   GET    /scenarios                       — list (Summary[])
 *   GET    /scenarios/:id                   — detail (Scenario)
 *   POST   /scenarios                       — create (Scenario)
 *   PUT    /scenarios/:id                   — replace (Scenario)
 *                                             — 5xx for `error-` prefix
 *   DELETE /scenarios/:id                   — delete (204)
 *   POST   /scenarios/:id/duplicate         — duplicate (Scenario)
 *   POST   /executions                      — stub (returns fake id;
 *                                              used by Save & Run, Task 3)
 *
 * The in-memory store survives reloads only within the page session;
 * a hard refresh re-seeds from `initialScenarioStore`.
 */
import { mock } from '../MockAdapter';
import type { Problem, Scenario, ScenarioInput } from '../../pages/scenarios/types';

import {
  FORCE_FAILURE_ID_PREFIX,
  initialScenarioStore,
} from '../data/scenarios';

/** Working copy — never the imported fixtures themselves. */
const scenarios: Scenario[] = initialScenarioStore.map((s) =>
  structuredClone(s),
);

let nextId = 1;
function newScenarioId(): string {
  // Stable, monotonically-increasing id. Skip past existing seeded ids.
  while (scenarios.some((s) => s.id === `scn-user-${String(nextId).padStart(3, '0')}`)) {
    nextId += 1;
  }
  const id = `scn-user-${String(nextId).padStart(3, '0')}`;
  nextId += 1;
  return id;
}

function notFound(id: string): [number, Problem] {
  return [
    404,
    {
      type: 'about:blank',
      title: 'Scenario not found',
      status: 404,
      detail: `No scenario with id "${id}"`,
    },
  ];
}

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

function toSummary(s: Scenario): Omit<Scenario, 'avpTree' | 'services' | 'variables' | 'steps'> {
  // Strips heavy fields for the list view; keeps the row-level shape.
  // Type narrowing: the Scenario type is `ScenarioSummary & {…}`, so
  // dropping the four heavy fields yields a valid ScenarioSummary.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { avpTree, services, variables, steps, ...rest } = s;
  return rest;
}

function applyInput(base: Scenario | null, input: ScenarioInput, idHint?: string): Scenario {
  return {
    id: base?.id ?? idHint ?? '',
    name: input.name,
    description: input.description,
    unitType: input.unitType,
    sessionMode: input.sessionMode,
    serviceModel: input.serviceModel,
    favourite: input.favourite,
    subscriberId: input.subscriberId,
    peerId: input.peerId,
    origin: 'user',
    stepCount: input.steps.length,
    updatedAt: new Date().toISOString(),
    avpTree: input.avpTree,
    services: input.services,
    variables: input.variables,
    steps: input.steps,
  };
}

// ---------------------------------------------------------------------------
// LIST
// ---------------------------------------------------------------------------
mock
  .onGet(/\/scenarios$/)
  .withDelayInMs(150)
  .reply((): [number, ReturnType<typeof toSummary>[]] => [
    200,
    scenarios.map(toSummary),
  ]);

// ---------------------------------------------------------------------------
// DETAIL
// ---------------------------------------------------------------------------
mock.onGet(/\/scenarios\/[^/]+$/).reply((config): [number, Scenario | Problem] => {
  const m = /\/scenarios\/([^/]+)$/.exec(config.url ?? '');
  const id = m ? decodeURIComponent(m[1]) : '';
  const scn = scenarios.find((s) => s.id === id);
  if (!scn) return notFound(id);
  return [200, structuredClone(scn)];
});

// ---------------------------------------------------------------------------
// CREATE
// ---------------------------------------------------------------------------
mock
  .onPost(/\/scenarios$/)
  .withDelayInMs(250)
  .reply((config): [number, Scenario | Problem] => {
    const input = safeParse<ScenarioInput>(config.data);
    if (!input || !input.name) {
      return [
        422,
        {
          type: 'about:blank',
          title: 'Validation failed',
          status: 422,
          detail: 'Scenario name is required',
          errors: { '/name': ['Name is required'] },
        },
      ];
    }
    const id = newScenarioId();
    const created = applyInput(null, input, id);
    scenarios.push(created);
    return [201, structuredClone(created)];
  });

// ---------------------------------------------------------------------------
// DUPLICATE
// ---------------------------------------------------------------------------
mock
  .onPost(/\/scenarios\/[^/]+\/duplicate$/)
  .withDelayInMs(200)
  .reply((config): [number, Scenario | Problem] => {
    const m = /\/scenarios\/([^/]+)\/duplicate$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const src = scenarios.find((s) => s.id === id);
    if (!src) return notFound(id);
    const body = safeParse<{ name?: string }>(config.data) ?? {};
    const name = body.name && body.name.trim().length > 0
      ? body.name
      : `${src.name} (copy)`;
    const newId = newScenarioId();
    const dup: Scenario = {
      ...structuredClone(src),
      id: newId,
      name,
      origin: 'user',
      favourite: false,
      updatedAt: new Date().toISOString(),
    };
    scenarios.push(dup);
    return [201, structuredClone(dup)];
  });

// ---------------------------------------------------------------------------
// UPDATE — 5xx for `error-` prefix exercises the failure path
// ---------------------------------------------------------------------------
mock
  .onPut(/\/scenarios\/[^/]+$/)
  .withDelayInMs(250)
  .reply((config): [number, Scenario | Problem] => {
    const m = /\/scenarios\/([^/]+)$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    if (id.startsWith(FORCE_FAILURE_ID_PREFIX)) {
      return [
        500,
        {
          type: 'about:blank',
          title: 'Forced failure',
          status: 500,
          detail: 'Scenario id begins with `error-` — mock returns 5xx.',
        },
      ];
    }
    const idx = scenarios.findIndex((s) => s.id === id);
    if (idx === -1) return notFound(id);
    const input = safeParse<ScenarioInput>(config.data);
    if (!input) {
      return [
        422,
        {
          type: 'about:blank',
          title: 'Validation failed',
          status: 422,
          detail: 'Body could not be parsed',
        },
      ];
    }
    const updated = applyInput(scenarios[idx], input);
    scenarios[idx] = updated;
    return [200, structuredClone(updated)];
  });

// ---------------------------------------------------------------------------
// DELETE
// ---------------------------------------------------------------------------
mock
  .onDelete(/\/scenarios\/[^/]+$/)
  .withDelayInMs(150)
  .reply((config): [number, void | Problem] => {
    const m = /\/scenarios\/([^/]+)$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const idx = scenarios.findIndex((s) => s.id === id);
    if (idx === -1) return notFound(id);
    scenarios.splice(idx, 1);
    return [204, undefined];
  });

// ---------------------------------------------------------------------------
// EXECUTIONS — stub for Save & Run (Task 3)
// ---------------------------------------------------------------------------
mock
  .onPost(/\/executions$/)
  .withDelayInMs(150)
  .reply((): [number, { batchId?: string; items: { id: string }[] }] => {
    const id = `exec-stub-${Date.now()}`;
    return [201, { items: [{ id }] }];
  });
