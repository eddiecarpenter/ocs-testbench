/**
 * Dirty-state guard — `beforeunload` listener only.
 *
 * Covers refresh, close-tab, and out-of-app navigation (browsers
 * ignore custom strings now and show their own confirm; setting
 * `e.returnValue = ''` is enough to trigger it).
 *
 * In-app router navigation is NOT guarded here. The original design
 * used `useBlocker`, but that hook requires React Router's data
 * router (`createBrowserRouter` + `RouterProvider`); the app uses
 * the legacy `<BrowserRouter>` + `<Routes>` form, so `useBlocker`
 * throws on render. Restoring the in-app guard requires migrating
 * the app to the data router.
 */
import { useEffect } from 'react';

interface DirtyGuardProps {
  dirty: boolean;
}

export function DirtyGuard({ dirty }: DirtyGuardProps) {
  useEffect(() => {
    if (!dirty) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  return null;
}
