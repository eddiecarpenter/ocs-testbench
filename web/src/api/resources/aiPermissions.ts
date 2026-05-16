import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import ApiService from '../ApiService';
import type { components } from '../schema';

export type AIToolPermission = components['schemas']['AIToolPermission'];
export type AIPermissionInput = components['schemas']['AIPermissionInput'];

export const aiPermissionsKeys = {
  all: ['aiPermissions'] as const,
};

/**
 * Fetch the merged list of MCP tools and their persisted permission decisions.
 * Tools with no stored decision default to "ask".
 */
export const listAIPermissions = (signal?: AbortSignal) =>
  ApiService.get<AIToolPermission[]>('/config/ai/permissions', { signal });

/**
 * Persist a permission decision for a named tool.
 * When decision is "ask" the stored record is deleted (reset to default).
 */
export const setAIPermission = (toolName: string, input: AIPermissionInput) =>
  ApiService.patch<void, AIPermissionInput>(
    `/config/ai/permissions/${encodeURIComponent(toolName)}`,
    input,
  );

/**
 * Reset a tool's stored decision back to "ask" (delete the DB record).
 */
export const deleteAIPermission = (toolName: string) =>
  ApiService.delete<void>(`/config/ai/permissions/${encodeURIComponent(toolName)}`);

/**
 * Read the full permissions list. Returns an empty array when the backend
 * has no entries or the MCP server is unavailable.
 */
export function useAIPermissions() {
  return useQuery({
    queryKey: aiPermissionsKeys.all,
    queryFn: ({ signal }) => listAIPermissions(signal),
  });
}

/**
 * Mutate a single tool's permission decision. Invalidates the permissions
 * cache on success so the list refetches automatically.
 */
export function useSetAIPermission() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ toolName, decision }: { toolName: string; decision: AIPermissionInput['decision'] }) =>
      setAIPermission(toolName, { decision }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: aiPermissionsKeys.all });
    },
  });
}

/**
 * Delete a tool's stored decision (reset to "ask"). Invalidates cache on success.
 */
export function useDeleteAIPermission() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (toolName: string) => deleteAIPermission(toolName),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: aiPermissionsKeys.all });
    },
  });
}
