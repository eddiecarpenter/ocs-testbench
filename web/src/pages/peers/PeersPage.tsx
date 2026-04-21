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
  useConnectPeer,
  useCreatePeer,
  useDeletePeer,
  usePeers,
  useRestartPeer,
  useTestPeerConfig,
  useUpdatePeer,
  useDisconnectPeer,
} from '../../api/resources/peers';
import { PeerStatusLabel } from '../../components/peer/PeerStatusLabel';
import { PeerForm } from './PeerForm';

type StatusFilter = 'all' | PeerStatus;

type PeerPrompt =
  | { kind: 'created'; peer: Peer }
  | { kind: 'updated-idle'; peer: Peer }
  | { kind: 'updated-live'; peer: Peer };

/** Post-mutation a peer is "live" if it's active or attempting/holding an error. */
function isLivePeer(p: Peer): boolean {
  return (
    p.status === 'connected' ||
    p.status === 'connecting' ||
    p.status === 'error'
  );
}

/**
 * Make the Drawer a vertical flex container so the form inside can have
 * its own flex:1 scrollable middle and a sticky footer. Without this the
 * Drawer body has its own `overflow: auto`, which fights the form's own
 * scroll region and pushes the footer off-screen when the body grows.
 */
const DRAWER_STYLES = {
  content: {
    display: 'flex',
    flexDirection: 'column' as const,
  },
  body: {
    display: 'flex',
    flexDirection: 'column' as const,
    flex: 1,
    minHeight: 0,
    overflow: 'hidden',
  },
};

const STATUS_FILTER_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'connected', label: 'Connected' },
  { value: 'connecting', label: 'Connecting' },
  { value: 'disconnecting', label: 'Disconnecting' },
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
  // Single post-mutation prompt, varying by (create vs update) × (idle vs
  // live). Three copy/action combinations:
  //   - created          → "A new peer 'x' was created. Connect now?" → Connect
  //   - updated-idle     → "Peer 'x' was updated but is not currently
  //                         connected. Connect now?" → Connect
  //   - updated-live     → "Peer 'x' was updated. Currently connected.
  //                         Restart?" → Disconnect + Connect
  const [prompt, setPrompt] = useState<PeerPrompt | undefined>();

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
        onCreated={(p) => setPrompt({ kind: 'created', peer: p })}
      />
      <EditPeerDrawer
        peer={drawer.kind === 'edit' ? drawer.peer : undefined}
        onClose={() => setDrawer({ kind: 'closed' })}
        onRequestDelete={(p) => {
          setDrawer({ kind: 'closed' });
          setDeleteTarget(p);
        }}
        onUpdated={(p) =>
          setPrompt({
            kind: isLivePeer(p) ? 'updated-live' : 'updated-idle',
            peer: p,
          })
        }
      />
      <DeletePeerModal
        peer={deleteTarget}
        onClose={() => setDeleteTarget(undefined)}
      />
      <PeerLifecyclePrompt
        prompt={prompt}
        onClose={() => setPrompt(undefined)}
      />
    </Stack>
  );
}

/**
 * Kebab menu on each row — Connect/Disconnect + Edit only. Destructive
 * actions (Delete) deliberately live inside the Edit drawer so the full
 * peer identity is visible at the moment of confirmation — reduces the
 * risk of deleting the wrong row from a dense table. This is the
 * standard CRUD pattern for the app.
 */
function RowMenu({
  peer,
  onEdit,
}: {
  peer: Peer;
  onEdit: () => void;
}) {
  const connect = useConnectPeer();
  const disconnect = useDisconnectPeer();
  // Only `disconnected` offers Connect. Every other state — connected,
  // connecting, disconnecting, restarting, error — offers Disconnect so
  // the user has a way to stop the session (including a peer that's
  // stuck in `error` retrying supervision).
  const showDisconnect = peer.status !== 'disconnected';
  // Transient states are mid-transition; kicking off a second action
  // during one would race the optimistic patch. Disable the menu item
  // until the peer settles.
  const isTransient =
    peer.status === 'connecting' ||
    peer.status === 'disconnecting' ||
    peer.status === 'restarting';

  // Outcome toasts are handled globally by `usePeerStatusToasts` — it
  // watches the list cache and fires on every settled status change, so
  // we only need to surface thrown errors (optimistic rollback puts the
  // cache back to the previous state, which produces no transition).
  const handleConnect = async () => {
    try {
      await connect.mutateAsync(peer.id);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Connect failed',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

  const handleDisconnect = async () => {
    try {
      await disconnect.mutateAsync(peer.id);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Disconnect failed',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

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
        {showDisconnect ? (
          <Menu.Item
            leftSection={<IconPlugConnectedX size={14} />}
            onClick={handleDisconnect}
            disabled={isTransient || disconnect.isPending}
          >
            Disconnect
          </Menu.Item>
        ) : (
          <Menu.Item
            leftSection={<IconPlugConnected size={14} />}
            onClick={handleConnect}
            disabled={isTransient || connect.isPending}
          >
            Connect
          </Menu.Item>
        )}
        <Menu.Item leftSection={<IconPencil size={14} />} onClick={onEdit}>
          Edit
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}

/** Drawer wrapping PeerForm in create mode. */
/**
 * Run a stateless CER/CEA probe for the given candidate config and
 * surface the outcome as a toast. Shared by Create and Edit drawers.
 */
function useProbeConfig() {
  const testConfig = useTestPeerConfig();
  const run = async (values: PeerInput) => {
    try {
      const result: PeerTestResult = await testConfig.mutateAsync(values);
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
  return { run, pending: testConfig.isPending };
}

function CreatePeerDrawer({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (p: Peer) => void;
}) {
  const createPeer = useCreatePeer();
  const probe = useProbeConfig();

  const handleSubmit = async (values: PeerInput) => {
    try {
      const peer = await createPeer.mutateAsync(values);
      notifications.show({
        color: 'teal',
        title: 'Peer created',
        message: `${peer.name} is ready.`,
      });
      onClose();
      // Defer so the drawer close transition runs before the modal opens —
      // avoids the modal overlay stacking under the drawer's fade-out.
      setTimeout(() => onCreated(peer), 0);
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
      styles={DRAWER_STYLES}
    >
      <PeerForm
        mode="create"
        submitting={createPeer.isPending}
        testing={probe.pending}
        onSubmit={handleSubmit}
        onTest={probe.run}
        onCancel={onClose}
      />
    </Drawer>
  );
}

/** Drawer wrapping PeerForm in edit mode — includes Test + Delete actions. */
function EditPeerDrawer({
  peer,
  onClose,
  onRequestDelete,
  onUpdated,
}: {
  peer: Peer | undefined;
  onClose: () => void;
  onRequestDelete: (p: Peer) => void;
  onUpdated: (p: Peer) => void;
}) {
  return (
    <Drawer
      opened={Boolean(peer)}
      onClose={onClose}
      position="right"
      size={480}
      padding="lg"
      withCloseButton
      styles={DRAWER_STYLES}
    >
      {peer && (
        <EditPeerForm
          peer={peer}
          onClose={onClose}
          onRequestDelete={() => onRequestDelete(peer)}
          onUpdated={onUpdated}
        />
      )}
    </Drawer>
  );
}

function EditPeerForm({
  peer,
  onClose,
  onRequestDelete,
  onUpdated,
}: {
  peer: Peer;
  onClose: () => void;
  onRequestDelete: () => void;
  onUpdated: (p: Peer) => void;
}) {
  const updatePeer = useUpdatePeer(peer.id);
  const probe = useProbeConfig();

  const handleSubmit = async (values: PeerInput) => {
    try {
      const next = await updatePeer.mutateAsync(values);
      notifications.show({
        color: 'teal',
        title: 'Peer updated',
        message: `${next.name} has been saved.`,
      });
      onClose();
      setTimeout(() => onUpdated(next), 0);
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
      mode="edit"
      initial={peer}
      submitting={updatePeer.isPending}
      testing={probe.pending}
      onSubmit={handleSubmit}
      onTest={probe.run}
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

/**
 * Unified post-mutation lifecycle prompt. Three variants:
 *
 *   - `created`       — new peer; offer immediate connect.
 *   - `updated-idle`  — edited peer that is not live; offer connect.
 *   - `updated-live`  — edited peer that is live; offer restart so the
 *                       new config takes effect on the running session.
 *
 * autoConnect is intentionally **not** mentioned in any variant — it
 * governs server-startup behaviour only (documented in the form helper
 * text) and has no bearing on the decision here.
 */
function PeerLifecyclePrompt({
  prompt,
  onClose,
}: {
  prompt: PeerPrompt | undefined;
  onClose: () => void;
}) {
  const connect = useConnectPeer();
  const restart = useRestartPeer();

  // Close the dialog the moment the user confirms, then let the mutation
  // settle in the background. The optimistic status patch already gives
  // live feedback in the row; holding the modal open for 10+ seconds
  // while the transport handshakes blocks the user for no reason.
  const handleConfirm = () => {
    if (!prompt) return;
    const { peer, kind } = prompt;
    const isRestart = kind === 'updated-live';
    onClose();

    const promise = isRestart
      ? restart.mutateAsync(peer.id)
      : connect.mutateAsync(peer.id);

    // Success toast is handled globally by `usePeerStatusToasts`, which
    // fires on any status transition. We only need to surface thrown
    // errors here — a rollback leaves the cache unchanged, so no global
    // toast would fire for a failed request.
    promise.catch((err: unknown) => {
      notifications.show({
        color: 'red',
        title: isRestart ? 'Restart failed' : 'Connect failed',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    });
  };

  const copy = prompt ? promptCopy(prompt) : undefined;

  return (
    <Modal
      opened={Boolean(prompt)}
      onClose={onClose}
      title={copy?.title ?? ''}
      centered
    >
      <Stack gap="md">
        <Text size="sm">{copy?.body}</Text>
        <Group justify="flex-end">
          <Button variant="subtle" onClick={onClose}>
            {copy?.cancelLabel ?? 'Cancel'}
          </Button>
          <Button onClick={handleConfirm}>
            {copy?.confirmLabel ?? 'Confirm'}
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}

function promptCopy(p: PeerPrompt): {
  title: string;
  body: React.ReactNode;
  cancelLabel: string;
  confirmLabel: string;
} {
  const name = <strong>&apos;{p.peer.name}&apos;</strong>;
  switch (p.kind) {
    case 'created':
      return {
        title: 'Connect peer now?',
        body: (
          <>
            A new peer {name} was created. Would you like to connect it now?
          </>
        ),
        cancelLabel: 'Not now',
        confirmLabel: 'Connect',
      };
    case 'updated-idle':
      return {
        title: 'Connect peer now?',
        body: (
          <>
            Peer {name} was updated but is not currently{' '}
            <strong>connected</strong>. Would you like to connect it now?
          </>
        ),
        cancelLabel: 'Not now',
        confirmLabel: 'Connect',
      };
    case 'updated-live':
      return {
        title: 'Restart peer?',
        body: (
          <>
            Peer {name} was updated. The peer is currently{' '}
            <strong>connected</strong>. Would you like to restart it so that
            the changes can take effect?
          </>
        ),
        cancelLabel: 'Later',
        confirmLabel: 'Restart',
      };
  }
}
