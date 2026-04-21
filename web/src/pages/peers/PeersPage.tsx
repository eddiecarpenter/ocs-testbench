import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Drawer,
  Group,
  Menu,
  Modal,
  Select,
  Skeleton,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import {
  IconAlertTriangle,
  IconDots,
  IconPencil,
  IconPlus,
  IconPlugConnected,
  IconPlugConnectedX,
  IconSearch,
  IconTrash,
} from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import { ApiError } from '../../api/errors';
import type {
  Peer,
  PeerInput,
  PeerStatus,
  PeerTestResult,
} from '../../api/resources/peers';
import {
  useCreatePeer,
  useDeletePeer,
  usePeers,
  useTestPeer,
  useUpdatePeer,
} from '../../api/resources/peers';
import { PeerStatusLabel } from '../../components/peer/PeerStatusLabel';
import { PeerForm } from './PeerForm';

type StatusFilter = 'all' | PeerStatus;

const STATUS_FILTER_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'connected', label: 'Connected' },
  { value: 'connecting', label: 'Connecting' },
  { value: 'disconnected', label: 'Disconnected' },
  { value: 'error', label: 'Error' },
];

/**
 * Peers listing. Search + status filter, table with kebab action menu,
 * and a right-hand Drawer for create/edit. Matches the Figma "Peers /
 * Light" + "Peers - Edit / Light" screens.
 */
export function PeersPage() {
  const peers = usePeers();
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [drawer, setDrawer] = useState<
    { kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; peer: Peer }
  >({ kind: 'closed' });
  const [deleteTarget, setDeleteTarget] = useState<Peer | undefined>();

  const filtered = useMemo(() => {
    if (!peers.data) return [];
    const q = query.trim().toLowerCase();
    return peers.data.filter((p) => {
      if (statusFilter !== 'all' && p.status !== statusFilter) return false;
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        p.host.toLowerCase().includes(q) ||
        p.originHost.toLowerCase().includes(q) ||
        p.originRealm.toLowerCase().includes(q)
      );
    });
  }, [peers.data, query, statusFilter]);

  return (
    <Stack gap="lg" p="md">
      {/* Title + Add peer */}
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={4}>
          <Title order={2} fw={600}>
            Peers
          </Title>
          <Text c="dimmed" size="sm">
            Configure and manage Diameter peer connections.
          </Text>
        </Stack>
        <Button
          leftSection={<IconPlus size={16} />}
          onClick={() => setDrawer({ kind: 'create' })}
        >
          Add peer
        </Button>
      </Group>

      {/* Search + Status filter */}
      <Group justify="space-between" wrap="nowrap">
        <TextInput
          placeholder="Search peers…"
          value={query}
          onChange={(e) => setQuery(e.currentTarget.value)}
          leftSection={<IconSearch size={14} />}
          w={320}
          aria-label="Search peers"
        />
        <Select
          data={STATUS_FILTER_OPTIONS}
          value={statusFilter}
          onChange={(v) => setStatusFilter((v as StatusFilter | null) ?? 'all')}
          checkIconPosition="right"
          allowDeselect={false}
          w={160}
          aria-label="Status filter"
          renderOption={({ option, checked }) => (
            <Group justify="space-between" w="100%">
              <Text size="sm">Status: {option.label}</Text>
              {checked && <Text size="xs" c="dimmed">✓</Text>}
            </Group>
          )}
          leftSection={<Text size="sm" c="dimmed">Status:</Text>}
          leftSectionWidth={60}
          styles={{ input: { paddingLeft: 60 } }}
        />
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
                <Table.Th tt="uppercase" fz="xs" c="dimmed">
                  Status
                </Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">
                  Name
                </Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">
                  Endpoint
                </Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">
                  Origin-Host
                </Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">
                  Auto-connect
                </Table.Th>
                <Table.Th w={48} aria-label="Actions" />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {filtered.length === 0 ? (
                <Table.Tr>
                  <Table.Td colSpan={6}>
                    <Text c="dimmed" ta="center" py="md" size="sm">
                      {peers.data.length === 0
                        ? 'No peers yet. Click Add peer to create one.'
                        : 'No peers match the current filters.'}
                    </Text>
                  </Table.Td>
                </Table.Tr>
              ) : (
                filtered.map((peer) => (
                  <Table.Tr key={peer.id}>
                    <Table.Td>
                      <PeerStatusLabel status={peer.status} />
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" fw={600}>
                        {peer.name}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" ff="monospace">
                        {peer.host}:{peer.port}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" ff="monospace">
                        {peer.originHost}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Badge
                        variant="light"
                        radius="sm"
                        color={peer.autoConnect ? 'teal' : 'gray'}
                      >
                        {peer.autoConnect ? 'Yes' : 'No'}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <RowMenu
                        peer={peer}
                        onEdit={() => setDrawer({ kind: 'edit', peer })}
                        onDelete={() => setDeleteTarget(peer)}
                      />
                    </Table.Td>
                  </Table.Tr>
                ))
              )}
            </Table.Tbody>
          </Table>
        </Card>
      )}

      <CreatePeerDrawer
        open={drawer.kind === 'create'}
        onClose={() => setDrawer({ kind: 'closed' })}
      />
      <EditPeerDrawer
        peer={drawer.kind === 'edit' ? drawer.peer : undefined}
        onClose={() => setDrawer({ kind: 'closed' })}
        onRequestDelete={(p) => {
          setDrawer({ kind: 'closed' });
          setDeleteTarget(p);
        }}
      />
      <DeletePeerModal
        peer={deleteTarget}
        onClose={() => setDeleteTarget(undefined)}
      />
    </Stack>
  );
}

/** Kebab menu on each row — Connect/Disconnect + Edit + Delete. */
function RowMenu({
  peer,
  onEdit,
  onDelete,
}: {
  peer: Peer;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const isConnected = peer.status === 'connected' || peer.status === 'connecting';
  return (
    <Menu shadow="md" width={180} position="bottom-end">
      <Menu.Target>
        <ActionIcon
          variant="subtle"
          color="gray"
          aria-label={`Actions for ${peer.name}`}
        >
          <IconDots size={16} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        {isConnected ? (
          <Menu.Item
            leftSection={<IconPlugConnectedX size={14} />}
            onClick={() =>
              notifications.show({
                color: 'gray',
                title: 'Disconnect not wired',
                message: 'Disconnect action will be wired once the peer lifecycle endpoints land.',
              })
            }
          >
            Disconnect
          </Menu.Item>
        ) : (
          <Menu.Item
            leftSection={<IconPlugConnected size={14} />}
            onClick={() =>
              notifications.show({
                color: 'gray',
                title: 'Connect not wired',
                message: 'Connect action will be wired once the peer lifecycle endpoints land.',
              })
            }
          >
            Connect
          </Menu.Item>
        )}
        <Menu.Item leftSection={<IconPencil size={14} />} onClick={onEdit}>
          Edit
        </Menu.Item>
        <Menu.Divider />
        <Menu.Item
          color="red"
          leftSection={<IconTrash size={14} />}
          onClick={onDelete}
        >
          Delete
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}

/** Drawer wrapping PeerForm in create mode. */
function CreatePeerDrawer({
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
    <Drawer
      opened={open}
      onClose={onClose}
      position="right"
      size={480}
      padding="lg"
      withCloseButton
      closeOnClickOutside={!createPeer.isPending}
      closeOnEscape={!createPeer.isPending}
    >
      <div style={{ height: 'calc(100dvh - 32px)' }}>
        <PeerForm
          mode="create"
          submitting={createPeer.isPending}
          onSubmit={handleSubmit}
          onCancel={onClose}
        />
      </div>
    </Drawer>
  );
}

/** Drawer wrapping PeerForm in edit mode — includes Test + Delete actions. */
function EditPeerDrawer({
  peer,
  onClose,
  onRequestDelete,
}: {
  peer: Peer | undefined;
  onClose: () => void;
  onRequestDelete: (p: Peer) => void;
}) {
  return (
    <Drawer
      opened={Boolean(peer)}
      onClose={onClose}
      position="right"
      size={480}
      padding="lg"
      withCloseButton
    >
      <div style={{ height: 'calc(100dvh - 32px)' }}>
        {peer && (
          <EditPeerForm
            peer={peer}
            onClose={onClose}
            onRequestDelete={() => onRequestDelete(peer)}
          />
        )}
      </div>
    </Drawer>
  );
}

function EditPeerForm({
  peer,
  onClose,
  onRequestDelete,
}: {
  peer: Peer;
  onClose: () => void;
  onRequestDelete: () => void;
}) {
  const updatePeer = useUpdatePeer(peer.id);
  const testPeer = useTestPeer(peer.id);

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

  const handleTest = async () => {
    try {
      const result: PeerTestResult = await testPeer.mutateAsync();
      notifications.show({
        color: result.ok ? 'teal' : 'red',
        title: result.ok ? 'Probe succeeded' : 'Probe failed',
        message: `${result.detail ?? (result.ok ? 'OK' : 'No response')} (${result.durationMs} ms)`,
      });
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Probe failed',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

  return (
    <PeerForm
      mode="edit"
      initial={peer}
      submitting={updatePeer.isPending}
      testing={testPeer.isPending}
      onSubmit={handleSubmit}
      onTest={handleTest}
      onDelete={onRequestDelete}
      onCancel={onClose}
    />
  );
}

/** Lightweight confirm-before-delete modal (kept separate from the drawer). */
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
