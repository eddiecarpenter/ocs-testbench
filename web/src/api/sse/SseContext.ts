import { createContext, useContext } from 'react';

import type { SseStatus } from './SseClient';

export interface SseContextValue {
  status: SseStatus;
}

export const SseContext = createContext<SseContextValue>({ status: 'idle' });

export function useSseStatus(): SseStatus {
  return useContext(SseContext).status;
}
