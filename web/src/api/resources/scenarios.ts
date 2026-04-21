import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type ScenarioSummary = components['schemas']['ScenarioSummary'];

export const scenarioKeys = {
  all: ['scenarios'] as const,
  list: () => [...scenarioKeys.all, 'list'] as const,
};

export const listScenarios = (signal?: AbortSignal) =>
  ApiService.get<ScenarioSummary[]>('/scenarios', { signal });

export function useScenarios() {
  return useQuery({
    queryKey: scenarioKeys.list(),
    queryFn: ({ signal }) => listScenarios(signal),
  });
}
