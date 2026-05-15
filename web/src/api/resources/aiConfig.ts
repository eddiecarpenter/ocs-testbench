import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type AIConfig = components['schemas']['AIConfig'];
export type AIConfigInput = components['schemas']['AIConfigInput'];

export const aiConfigKeys = {
  all: ['aiConfig'] as const,
  models: () => [...aiConfigKeys.all, 'models'] as const,
};

/**
 * Fetch the current AI assistant configuration from the backend.
 * The API key value is never returned — only apiKeySet (boolean) is set.
 */
export const getAIConfig = (signal?: AbortSignal) =>
  ApiService.get<AIConfig>('/config/ai', { signal });

/**
 * Update the AI assistant configuration. Sends only the fields that have
 * changed. Leave apiKey blank to preserve the existing key.
 */
export const updateAIConfig = (input: AIConfigInput) =>
  ApiService.patch<AIConfig, AIConfigInput>('/config/ai', input);

/**
 * Fetch the list of model IDs available at the given LLM endpoint.
 * The backend proxies GET {endpoint}/v1/models to avoid CORS issues.
 * Pass `endpoint` to query a specific endpoint without saving it first
 * (used when the user is editing the provider field before saving).
 */
export const listAIModels = (endpoint?: string, apiKey?: string, signal?: AbortSignal) => {
  const params: Record<string, string> = {};
  if (endpoint) params.endpoint = endpoint;
  if (apiKey) params.apiKey = apiKey;
  return ApiService.get<string[]>('/config/ai/models', {
    signal,
    params: Object.keys(params).length > 0 ? params : undefined,
  });
};

/**
 * Read the current AI configuration. Returns an empty-field response when
 * the AI assistant was not configured at server startup.
 */
export function useAIConfig() {
  return useQuery({
    queryKey: aiConfigKeys.all,
    queryFn: ({ signal }) => getAIConfig(signal),
  });
}

/**
 * Mutate the AI configuration. Invalidates the config cache on success so
 * any mounted useAIConfig hook refetches the authoritative state.
 */
export function useUpdateAIConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AIConfigInput) => updateAIConfig(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: aiConfigKeys.all });
    },
  });
}

/**
 * Fetch the list of model IDs from the given LLM endpoint.
 *
 * Pass the current endpoint string from the card's local state.
 * An empty string disables the query. The endpoint is included in the
 * React Query cache key so changing the provider immediately fetches
 * the model list for the new endpoint without requiring a manual Refresh.
 */
export function useAIModels(endpoint: string, apiKey?: string) {
  return useQuery({
    queryKey: [...aiConfigKeys.models(), endpoint, !!apiKey],
    queryFn: ({ signal }) => listAIModels(endpoint, apiKey, signal),
    enabled: !!endpoint,
  });
}
