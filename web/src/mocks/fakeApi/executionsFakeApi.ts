import type { components } from '../../api/schema';
import type {
  Execution,
  ExecutionPage,
  ExecutionState,
} from '../../api/resources/executions';
import { buildExecutionDetail } from '../data/executionDetails';
import { executionFixtures } from '../data/executions';
import { mock } from '../MockAdapter';

/** RFC 7807 problem shape per OpenAPI v0.2. */
type ProblemBody = components['schemas']['Problem'];

// List endpoint — note regex anchors so /executions/:id doesn't match here.
mock
  .onGet(/\/executions(\?|$)/)
  .withDelayInMs(250)
  .reply((config): [number, ExecutionPage] => {
    const stateFilter = config.params?.state as ExecutionState | undefined;
    const limit = Math.min(500, Math.max(1, Number(config.params?.limit ?? 50)));
    const offset = Math.max(0, Number(config.params?.offset ?? 0));

    const filtered = stateFilter
      ? executionFixtures.filter((e) => e.state === stateFilter)
      : executionFixtures;

    const items = filtered.slice(offset, offset + limit);
    return [
      200,
      {
        items,
        page: { total: filtered.length, limit, offset },
      },
    ];
  });

// Detail endpoint — /executions/{id}
mock
  .onGet(/\/executions\/[^/]+$/)
  .withDelayInMs(200)
  .reply((config): [number, Execution | ProblemBody] => {
    const url = config.url ?? '';
    const id = decodeURIComponent(url.split('/').pop() ?? '');
    const detail = buildExecutionDetail(id);
    if (!detail) {
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
    return [200, detail];
  });
