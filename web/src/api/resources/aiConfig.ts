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
 * Fetch the list of model IDs available at the configured LLM endpoint.
 * The backend proxies GET {endpoint}/v1/models to avoid CORS issues.
 */
export const listAIModels = (signal?: AbortSignal) =>
  ApiService.get<string[]>('/config/ai/models', { signal });

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
 * Fetch the list of model IDs from the configured LLM endpoint.
 *
 * @param enabled - Set to true only when a valid endpoint is configured.
 *                  Pass `!!endpoint` from the card's local state so the
 *                  fetch is skipped until the user has entered an endpoint.
 */
export function useAIModels(enabled: boolean) {
  return useQuery({
    queryKey: aiConfigKeys.models(),
    queryFn: ({ signal }) => listAIModels(signal),
    enabled,
    staleTime: 60_000, // model lists change rarely; reuse for 1 minute
  });
}
