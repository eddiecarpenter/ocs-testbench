import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Group,
  Modal,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import {
  IconAlertTriangle,
  IconPencil,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import { useState } from 'react';

import { ApiError } from '../../api/errors';
import type { Peer, PeerInput, PeerStatus } from '../../api/resources/peers';
import {
  useCreatePeer,
  useDeletePeer,
  usePeers,
  useUpdatePeer,
} from '../../api/resources/peers';
import { PeerForm } from './PeerForm';

const statusColor: Record<PeerStatus, string> = {
  connected: 'teal',
  disconnected: 'gray',
  error: 'red',
  connecting: 'yellow',
};

type DialogState =
  | { kind: 'closed' }
  | { kind: 'create' }
  | { kind: 'edit'; peer: Peer }
  | { kind: 'delete'; peer: Peer };

export function PeersPage() {
  const peers = usePeers();
  const [dialog, setDialog] = useState<DialogState>({ kind: 'closed' });

  return (
    <Stack gap="lg" p="md">
      <Group justify="space-between" align="center">
        <Stack gap={4}>
          <Title order={2} fw={600}>
            Peers
          </Title>
          <Text c="dimmed" size="sm">
            Diameter peers participating in scenarios.
          </Text>
        </Stack>
        <Button
          leftSection={<IconPlus size={16} />}
          onClick={() => setDialog({ kind: 'create' })}
        >
          Add peer
        </Button>
      </Group>

      {peers.isError ? (
        <Alert
          color="red"
          icon={<IconAlertTriangle size={18} />}
          title="Peers unavailable"
          variant="light"
        >
          <Stack gap="xs" align="flex-start">
            <Text size="sm" c="dimmed">
              Couldn&apos;t load peers. Check the API and try again.
            </Text>
            <Button
              size="xs"
              variant="subtle"
              color="red"
              onClick={() => peers.refetch()}
            >
              Retry
            </Button>
          </Stack>
        </Alert>
      ) : peers.isLoading || !peers.data ? (
        <Card padding="lg" withBorder shadow="xs">
          <Stack gap="sm">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} h={40} radius="sm" />
            ))}
          </Stack>
        </Card>
      ) : (
        <Card padding={0} withBorder shadow="xs">
          <Table highlightOnHover verticalSpacing="sm" horizontalSpacing="md">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Endpoint</Table.Th>
                <Table.Th>Origin host</Table.Th>
                <Table.Th>Status</Table.Th>
                <Table.Th w={100} ta="right">
                  Actions
                </Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {peers.data.length === 0 ? (
                <Table.Tr>
                  <Table.Td colSpan={5}>
                    <Text c="dimmed" ta="center" py="md" size="sm">
                      No peers yet. Click <strong>Add peer</strong> to create one.
                    </Text>
                  </Table.Td>
                </Table.Tr>
              ) : (
                peers.data.map((peer) => (
                  <Table.Tr key={peer.id}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {peer.name}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" ff="monospace">
                        {peer.endpoint}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" ff="monospace">
                        {peer.originHost}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Tooltip
                        label={peer.statusDetail ?? peer.status}
                        disabled={!peer.statusDetail}
                      >
                        <Badge
                          variant="light"
                          color={statusColor[peer.status]}
                          radius="sm"
                        >
                          {peer.status}
                        </Badge>
                      </Tooltip>
                    </Table.Td>
                    <Table.Td>
                      <Group gap={4} justify="flex-end" wrap="nowrap">
                        <Tooltip label="Edit">
                          <ActionIcon
                            variant="subtle"
                            aria-label={`Edit ${peer.name}`}
                            onClick={() => setDialog({ kind: 'edit', peer })}
                          >
                            <IconPencil size={16} />
                          </ActionIcon>
                        </Tooltip>
                        <Tooltip label="Delete">
                          <ActionIcon
                            variant="subtle"
                            color="red"
                            aria-label={`Delete ${peer.name}`}
                            onClick={() => setDialog({ kind: 'delete', peer })}
                          >
                            <IconTrash size={16} />
                          </ActionIcon>
                        </Tooltip>
                      </Group>
                    </Table.Td>
                  </Table.Tr>
                ))
              )}
            </Table.Tbody>
          </Table>
        </Card>
      )}

      <CreatePeerModal
        open={dialog.kind === 'create'}
        onClose={() => setDialog({ kind: 'closed' })}
      />
      <EditPeerModal
        peer={dialog.kind === 'edit' ? dialog.peer : undefined}
        onClose={() => setDialog({ kind: 'closed' })}
      />
      <DeletePeerModal
        peer={dialog.kind === 'delete' ? dialog.peer : undefined}
        onClose={() => setDialog({ kind: 'closed' })}
      />
    </Stack>
  );
}

/** Modal wrapper around PeerForm for creating a new peer. */
function CreatePeerModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const createPeer = useCreatePeer();

  const handleSubmit = async (values: PeerInput) => {
    try {
      const peer = await createPeer.mutateAsync(values);
      notifications.show({
        color: 'teal',
        title: 'Peer created',
        message: `${peer.name} is ready.`,
      });
      onClose();
      return peer;
    } catch (err) {
      // 422 is handled in-form. Re-throw so the form can route it; otherwise
      // show a toast for non-field errors.
      if (err instanceof ApiError && err.status === 422) throw err;
      notifications.show({
        color: 'red',
        title: 'Could not create peer',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
      throw err;
    }
  };

  return (
    <Modal
      opened={open}
      onClose={onClose}
      title="Add peer"
      centered
      closeOnClickOutside={!createPeer.isPending}
      closeOnEscape={!createPeer.isPending}
    >
      <PeerForm
        submitLabel="Create peer"
        submitting={createPeer.isPending}
        onSubmit={handleSubmit}
        onCancel={onClose}
      />
    </Modal>
  );
}

/** Modal wrapper around PeerForm for editing an existing peer. */
function EditPeerModal({
  peer,
  onClose,
}: {
  peer: Peer | undefined;
  onClose: () => void;
}) {
  return (
    <Modal
      opened={Boolean(peer)}
      onClose={onClose}
      title={peer ? `Edit ${peer.name}` : undefined}
      centered
    >
      {peer && <EditPeerForm peer={peer} onClose={onClose} />}
    </Modal>
  );
}

function EditPeerForm({ peer, onClose }: { peer: Peer; onClose: () => void }) {
  const updatePeer = useUpdatePeer(peer.id);

  const handleSubmit = async (values: PeerInput) => {
    try {
      const next = await updatePeer.mutateAsync(values);
      notifications.show({
        color: 'teal',
        title: 'Peer updated',
        message: `${next.name} has been saved.`,
      });
      onClose();
      return next;
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) throw err;
      notifications.show({
        color: 'red',
        title: 'Could not update peer',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
      throw err;
    }
  };

  return (
    <PeerForm
      initial={peer}
      submitLabel="Save changes"
      submitting={updatePeer.isPending}
      onSubmit={handleSubmit}
      onCancel={onClose}
    />
  );
}

/** Lightweight confirm-before-delete modal. */
function DeletePeerModal({
  peer,
  onClose,
}: {
  peer: Peer | undefined;
  onClose: () => void;
}) {
  const deletePeer = useDeletePeer();

  const handleConfirm = async () => {
    if (!peer) return;
    try {
      await deletePeer.mutateAsync(peer.id);
      notifications.show({
        color: 'teal',
        title: 'Peer deleted',
        message: `${peer.name} was removed.`,
      });
      onClose();
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Could not delete peer',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

  return (
    <Modal
      opened={Boolean(peer)}
      onClose={onClose}
      title="Delete peer"
      centered
      closeOnClickOutside={!deletePeer.isPending}
      closeOnEscape={!deletePeer.isPending}
    >
      <Stack gap="md">
        <Text size="sm">
          Are you sure you want to delete{' '}
          <strong>{peer?.name ?? ''}</strong>? This cannot be undone.
        </Text>
        <Group justify="flex-end">
          <Button
            variant="subtle"
            onClick={onClose}
            disabled={deletePeer.isPending}
          >
            Cancel
          </Button>
          <Button color="red" loading={deletePeer.isPending} onClick={handleConfirm}>
            Delete
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
