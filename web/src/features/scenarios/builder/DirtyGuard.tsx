/**
 * Dirty-state guard. Two layers, both required:
 *
 * 1. **Router blocker** — `useBlocker` intercepts in-app navigation.
 *    When the draft is dirty, presents a confirm modal; the user can
 *    proceed or cancel.
 * 2. **`beforeunload` listener** — covers refresh, close-tab, and
 *    out-of-app navigation (browsers ignore custom strings now and
 *    show their own confirm; setting `e.returnValue = ''` is enough
 *    to trigger it).
 *
 * The `silenced` ref breaks the lock during a programmatic reset
 * (e.g. after a successful Save) — without it, the `markSaved` effect
 * would race the navigate call and the blocker would still be armed.
 */
import { Button, Group, Modal, Text } from '@mantine/core';
import { useEffect, useRef } from 'react';
import { useBlocker } from 'react-router';

interface DirtyGuardProps {
  dirty: boolean;
}

export function DirtyGuard({ dirty }: DirtyGuardProps) {
  const silenced = useRef(false);

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirty &&
      !silenced.current &&
      currentLocation.pathname !== nextLocation.pathname,
  );

  useEffect(() => {
    if (!dirty) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      // Modern browsers ignore the message and show their own; the
      // assignment is what triggers the prompt.
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  if (blocker.state !== 'blocked') return null;

  return (
    <Modal
      opened
      onClose={() => blocker.reset?.()}
      title="Discard unsaved changes?"
      centered
    >
      <Text size="sm" mb="md">
        You have unsaved changes in this scenario. Leaving this page will
        discard them.
      </Text>
      <Group justify="flex-end">
        <Button variant="subtle" onClick={() => blocker.reset?.()}>
          Stay
        </Button>
        <Button color="red" onClick={() => blocker.proceed?.()}>
          Discard and leave
        </Button>
      </Group>
    </Modal>
  );
}
