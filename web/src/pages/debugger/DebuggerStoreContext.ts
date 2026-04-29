/**
 * The bare React Context object for the Execution Debugger store.
 *
 * Kept in its own file so:
 *   - the provider (`DebuggerStoreProvider.tsx`) is export-only-components
 *     for Fast Refresh, and
 *   - the hooks (`useDebuggerStore.ts`) can read the same context without
 *     creating a circular import through the provider component.
 */
import { createContext } from 'react';

import type { ExecutionStore } from './executionStore';

export const DebuggerStoreContext = createContext<ExecutionStore | null>(null);
