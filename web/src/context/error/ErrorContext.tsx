import { createContext, type ReactNode } from 'react';

import type { ErrorDetails } from './types';

export interface ErrorContextValue {
  hasError: boolean;
  error: ErrorDetails | null;
  /** Clear the captured error and return to normal rendering. */
  clear: () => void;
}

export const ErrorContext = createContext<ErrorContextValue | null>(null);

export interface ErrorProviderProps {
  children: ReactNode;
}
