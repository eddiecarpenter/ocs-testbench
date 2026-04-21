import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type TemplateSummary = components['schemas']['TemplateSummary'];

export const templateKeys = {
  all: ['templates'] as const,
  list: () => [...templateKeys.all, 'list'] as const,
};

export const listTemplates = (signal?: AbortSignal) =>
  ApiService.get<TemplateSummary[]>('/templates', { signal });

export function useTemplates() {
  return useQuery({
    queryKey: templateKeys.list(),
    queryFn: ({ signal }) => listTemplates(signal),
  });
}
