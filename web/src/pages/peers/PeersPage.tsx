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
  IconPlayerPlay,
  IconPlayerStop,
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
  useCreatePeer,
  useDeletePeer,
  usePeers,
  useRestartPeer,
  useStartPeer,
  useStopPeer,
  useTestPeerConfig,
  useUpdatePeer,
} from '../../api/resources/peers';
import { PeerStatusLabel } from '../../components/peer/PeerStatusLabel';
import { PeerForm } from './PeerForm';

type StatusFilter = 'all' | PeerStatus;

/**
 * Input to the post-mutation lifecycle toast. Three variants:
 *   - `created`       — brand-new peer (always `stopped`); offer Start.
 *   - `updated-idle`  — edited peer not currently running; offer Start.
 *   - `updated-live`  — edited peer currently live; offer Restart so the
 *                       new config takes effect.
 * All three use the same non-blocking toast — no modals.
 */
type LifecycleToastInput =
  | { kind: 'created'; peer: Peer }
  | { kind: 'updated-idle'; peer: Peer }
  | { kind: 'updated-live'; peer: Peer };

/**
 * Post-mutation a peer is "live" if supervision is engaged — either it's
 * connected, in flight, or retrying. A `stopped` or `disconnected` peer
 * is not live and the update toast should offer Start instead of Restart.
 */
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
/**
 * Mantine's default Drawer transition is 150 ms; we wait slightly longer
 * before firing any follow-up toast/modal so the notification doesn't
 * land behind the still-closing drawer in the top-right corner.
 */
const DRAWER_CLOSE_TRANSITION_MS = 250;

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
  { value: 'stopped', label: 'Stopped' },
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
  // All three post-mutation flows (created / updated-idle / updated-live)
  // surface as a non-blocking toast with inline Connect/Restart actions.
  // See `showLifecycleToast`.
  const start = useStartPeer();
  const restart = useRestartPeer();

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
        onCreated={(p) =>
          showLifecycleToast({ kind: 'created', peer: p }, { start, restart })
        }
      />
      <EditPeerDrawer
        peer={drawer.kind === 'edit' ? drawer.peer : undefined}
        onClose={() => setDrawer({ kind: 'closed' })}
        onRequestDelete={(p) => {
          setDrawer({ kind: 'closed' });
          setDeleteTarget(p);
        }}
        onUpdated={(p) =>
          showLifecycleToast(
            {
              kind: isLivePeer(p) ? 'updated-live' : 'updated-idle',
              peer: p,
            },
            { start, restart },
          )
        }
      />
      <DeletePeerModal
        peer={deleteTarget}
        onClose={() => setDeleteTarget(undefined)}
      />
    </Stack>
  );
}

/**
 * Kebab menu on each row — Start/Stop + Edit only. Destructive
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
  const start = useStartPeer();
  const stop = useStopPeer();
  // Only `stopped` offers Start. Every other state — connected,
  // connecting, disconnected, disconnecting, restarting, error — offers
  // Stop so the user has a way to halt supervision (including a peer
  // stuck in `error` retrying, or a `disconnected` peer that's still
  // being redialed by supervision).
  const showStop = peer.status !== 'stopped';
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
  const handleStart = async () => {
    try {
      await start.mutateAsync(peer.id);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Start failed',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

  const handleStop = async () => {
    try {
      await stop.mutateAsync(peer.id);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Stop failed',
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
        {showStop ? (
          <Menu.Item
            leftSection={<IconPlayerStop size={14} />}
            onClick={handleStop}
            disabled={isTransient || stop.isPending}
          >
            Stop
          </Menu.Item>
        ) : (
          <Menu.Item
            leftSection={<IconPlayerPlay size={14} />}
            onClick={handleStart}
            disabled={isTransient || start.isPending}
          >
            Start
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
      // No separate "created" toast — the lifecycle toast raised by
      // onCreated already leads with "Peer 'x' created", so firing a
      // success toast here would just stack two notifications for the
      // same event (same reasoning as the update flow).
      onClose();
      // Defer so the drawer close transition runs before the modal opens —
      // avoids the modal overlay stacking under the drawer's fade-out.
      // Wait for the Drawer's close transition to finish before firing
      // the follow-up prompt — otherwise the Drawer still briefly covers
      // the top-right corner where the toast/modal lands, and the
      // notification flashes *behind* it. Mantine's default Drawer
      // transition is 150 ms; 250 ms gives a safe margin.
      setTimeout(() => onCreated(peer), DRAWER_CLOSE_TRANSITION_MS);
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
      // No separate "saved" toast — the update action-toast raised by
      // onUpdated already leads with "Peer 'x' updated", so firing a
      // success toast here would just stack two notifications for the
      // same event.
      onClose();
      setTimeout(() => onUpdated(next), DRAWER_CLOSE_TRANSITION_MS);
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
 * Unified post-mutation toast with inline Start/Restart action. Used for
 * all three flows (create, update-idle, update-live). Non-blocking: the
 * user can keep working and the reminder sits in the corner until they
 * act, dismiss, or it auto-closes after 10 s. `autoConnect` is
 * intentionally not mentioned — it governs server-startup only and has
 * no bearing on the current peer status.
 */
function showLifecycleToast(
  p: LifecycleToastInput,
  mutations: {
    start: ReturnType<typeof useStartPeer>;
    restart: ReturnType<typeof useRestartPeer>;
  },
) {
  const isRestart = p.kind === 'updated-live';
  const id = `peer-lifecycle-${p.peer.id}-${Date.now()}`;

  const run = () => {
    notifications.hide(id);
    const promise = isRestart
      ? mutations.restart.mutateAsync(p.peer.id)
      : mutations.start.mutateAsync(p.peer.id);
    promise.catch((err: unknown) => {
      notifications.show({
        color: 'red',
        title: isRestart ? 'Restart failed' : 'Start failed',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    });
  };

  const title =
    p.kind === 'created'
      ? `Peer '${p.peer.name}' created`
      : `Peer '${p.peer.name}' updated`;

  const body =
    p.kind === 'created' ? (
      <>Would you like to <strong>start</strong> it now?</>
    ) : isRestart ? (
      <>
        Currently <strong>connected</strong>. Restart to apply the new
        configuration.
      </>
    ) : (
      <>
        Not currently <strong>running</strong>. Start the peer now to apply
        the new configuration.
      </>
    );

  notifications.show({
    id,
    withCloseButton: true,
    autoClose: 10_000,
    color: isRestart ? 'yellow' : 'blue',
    title: (
      <Text size="sm" fw={600}>
        {title}
      </Text>
    ),
    message: (
      <Stack gap={8} mt={4}>
        <Text size="sm" c="dimmed">
          {body}
        </Text>
        <Group gap="xs">
          <Button size="xs" variant="light" onClick={run}>
            {isRestart ? 'Restart now' : 'Start now'}
          </Button>
          <Button
            size="xs"
            variant="subtle"
            color="gray"
            onClick={() => notifications.hide(id)}
          >
            Dismiss
          </Button>
        </Group>
      </Stack>
    ),
  });
}
