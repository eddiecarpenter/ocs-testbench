import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import ApiService from '../ApiService';

export interface CustomDictionary {
  id: string;
  name: string;
  description?: string;
  xmlContent: string;
  isActive: boolean;
}

export interface CustomDictionaryInput {
  name: string;
  description?: string;
  xmlContent: string;
  isActive: boolean;
}

export const dictKeys = {
  all: ['dictionaries'] as const,
  list: () => [...dictKeys.all, 'list'] as const,
  detail: (id: string) => [...dictKeys.all, 'detail', id] as const,
};

export const listDictionaries = (signal?: AbortSignal) =>
  ApiService.get<CustomDictionary[]>('/dictionaries', { signal });

export const createDictionary = (input: CustomDictionaryInput) =>
  ApiService.post<CustomDictionary, CustomDictionaryInput>('/dictionaries', input);

export const updateDictionary = (id: string, input: CustomDictionaryInput) =>
  ApiService.put<CustomDictionary, CustomDictionaryInput>(
    `/dictionaries/${encodeURIComponent(id)}`,
    input,
  );

export const deleteDictionary = (id: string) =>
  ApiService.delete<void>(`/dictionaries/${encodeURIComponent(id)}`);

export function useDictionaries() {
  return useQuery({
    queryKey: dictKeys.list(),
    queryFn: ({ signal }) => listDictionaries(signal),
  });
}

export function useCreateDictionary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CustomDictionaryInput) => createDictionary(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: dictKeys.list() }),
  });
}

export function useUpdateDictionary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: CustomDictionaryInput }) =>
      updateDictionary(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: dictKeys.list() }),
  });
}

export function useDeleteDictionary() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteDictionary(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: dictKeys.list() }),
  });
}
