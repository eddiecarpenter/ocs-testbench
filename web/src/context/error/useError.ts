import { useContext } from 'react';

import { ErrorContext, type ErrorContextValue } from './ErrorContext';

export function useError(): ErrorContextValue {
  const ctx = useContext(ErrorContext);
  if (!ctx) {
    throw new Error('useError must be used inside <ErrorProvider>');
  }
  return ctx;
}
