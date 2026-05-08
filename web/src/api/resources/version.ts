import { useQuery } from '@tanstack/react-query';

import ApiService from '../ApiService';

export interface VersionInfo {
  version: string;
}

export const versionKeys = {
  all: ['version'] as const,
};

export const getVersion = (signal?: AbortSignal) =>
  ApiService.get<VersionInfo>('/version', { signal });

export function useVersion() {
  return useQuery({
    queryKey: versionKeys.all,
    queryFn: ({ signal }) => getVersion(signal),
    // Version never changes at runtime — cache indefinitely.
    staleTime: Infinity,
  });
}
