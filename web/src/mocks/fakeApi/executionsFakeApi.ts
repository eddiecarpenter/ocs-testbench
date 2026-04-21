import type {
  Execution,
  ExecutionPage,
  ExecutionResult,
} from '../../api/resources/executions';
import { buildExecutionDetail } from '../data/executionDetails';
import { executionFixtures } from '../data/executions';
import { mock } from '../MockAdapter';

// List endpoint — note regex anchors so /executions/:id doesn't match here.
mock
  .onGet(/\/executions(\?|$)/)
  .withDelayInMs(250)
  .reply((config): [number, ExecutionPage] => {
    const statusFilter = config.params?.status as ExecutionResult | undefined;
    const limit = Math.min(500, Math.max(1, Number(config.params?.limit ?? 50)));
    const offset = Math.max(0, Number(config.params?.offset ?? 0));

    const filtered = statusFilter
      ? executionFixtures.filter((e) => e.result === statusFilter)
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
  .reply((config): [number, Execution | { title: string; status: number }] => {
    const url = config.url ?? '';
    const id = decodeURIComponent(url.split('/').pop() ?? '');
    const detail = buildExecutionDetail(id);
    if (!detail) {
      return [
        404,
        {
          title: 'Execution not found',
          status: 404,
        },
      ];
    }
    return [200, detail];
  });
