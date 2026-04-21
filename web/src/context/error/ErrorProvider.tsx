import { useCallback, useEffect, useMemo, useState } from 'react';

import { ErrorContext, type ErrorProviderProps } from './ErrorContext';
import { ErrorScreen } from './ErrorScreen';
import type { ErrorDetails } from './types';

/**
 * Top-level safety net that captures uncaught promise rejections and
 * runtime errors, then renders a recoverable full-screen fallback.
 *
 * TanStack Query errors that are handled in-component (via `isError`,
 * inline retry buttons, etc.) never reach this — only truly uncaught
 * ones do. Think of this as the "this shouldn't have happened" screen.
 */
export function ErrorProvider({ children }: ErrorProviderProps) {
  const [error, setError] = useState<ErrorDetails | null>(null);

  const clear = useCallback(() => setError(null), []);

  useEffect(() => {
    const captureRejection = (event: PromiseRejectionEvent) => {
      const reason = event.reason;
      const err = reason instanceof Error ? reason : new Error(String(reason));
      console.error('Unhandled promise rejection:', err);
      setError({
        message: err.message,
        name: err.name,
        stack: err.stack,
        cause: err.cause,
        time: new Date().toISOString(),
      });
    };

    const captureError = (event: ErrorEvent) => {
      const err = event.error instanceof Error ? event.error : new Error(event.message);
      console.error('Uncaught error:', err);
      setError({
        message: err.message,
        name: err.name,
        stack: err.stack,
        cause: err.cause,
        time: new Date().toISOString(),
      });
    };

    globalThis.addEventListener('unhandledrejection', captureRejection);
    globalThis.addEventListener('error', captureError);

    return () => {
      globalThis.removeEventListener('unhandledrejection', captureRejection);
      globalThis.removeEventListener('error', captureError);
    };
  }, []);

  const ctx = useMemo(
    () => ({ hasError: error !== null, error, clear }),
    [error, clear],
  );

  return (
    <ErrorContext.Provider value={ctx}>
      {error !== null ? <ErrorScreen /> : children}
    </ErrorContext.Provider>
  );
}
