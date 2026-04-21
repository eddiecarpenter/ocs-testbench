import type {
  ExecutionPage,
  ExecutionResult,
} from '../../api/resources/executions';
import { executionFixtures } from '../data/executions';
import { mock } from '../MockAdapter';

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
