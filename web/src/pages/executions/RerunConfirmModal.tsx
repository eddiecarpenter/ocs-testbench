/**
 * Confirm modal for re-running a Continuous-mode execution.
 *
 * Interactive sources re-run silently from the kebab; Continuous
 * sources hit this modal first so the operator can see the source
 * parameters before re-firing what may be a sizable batched run.
 *
 * Reuse: refactor — copies the centered modal pattern used by the
 * peers-page delete-confirm modal (`pages/peers/PeersPage.tsx`):
 * `closeOnClickOutside={!isPending}`, `closeOnEscape={!isPending}`.
 */
import { Button, Group, Modal, Stack, Text } from '@mantine/core';

import type { ExecutionSummary } from '../../api/resources/executions';

interface RerunConfirmModalProps {
  /** The source row being replayed; `null` keeps the modal closed. */
  source: ExecutionSummary | null;
  isPending: boolean;
  onClose(): void;
  onConfirm(): void;
}

export function RerunConfirmModal({
  source,
  isPending,
  onClose,
  onConfirm,
}: RerunConfirmModalProps) {
  return (
    <Modal
      opened={Boolean(source)}
      onClose={onClose}
      centered
      title="Re-run execution"
      closeOnClickOutside={!isPending}
      closeOnEscape={!isPending}
      data-testid="executions-rerun-confirm"
    >
      {source && (
        <Stack gap="md">
          <Text size="sm">
            Re-run <Text span fw={600}>#{source.id}</Text> for
            <Text span fw={600}> {source.scenarioName}</Text>?
          </Text>
          <Stack gap={2}>
            <Text size="xs" c="dimmed">
              Mode: continuous
            </Text>
            <Text size="xs" c="dimmed">
              Peer: {source.peerName ?? source.peerId ?? '—'}
            </Text>
            {source.subscriberId && (
              <Text size="xs" c="dimmed">
                Subscriber: {source.subscriberMsisdn ?? source.subscriberId}
              </Text>
            )}
          </Stack>
          <Group justify="flex-end" gap="sm">
            <Button
              variant="default"
              onClick={onClose}
              disabled={isPending}
              data-testid="executions-rerun-cancel"
            >
              Cancel
            </Button>
            <Button
              onClick={onConfirm}
              loading={isPending}
              data-testid="executions-rerun-submit"
            >
              Re-run
            </Button>
          </Group>
        </Stack>
      )}
    </Modal>
  );
}
