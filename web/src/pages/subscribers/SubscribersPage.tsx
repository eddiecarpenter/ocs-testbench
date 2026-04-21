import {
  ActionIcon,
  Alert,
  Button,
  Card,
  Drawer,
  Group,
  Menu,
  Modal,
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
  IconSearch,
} from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import { ApiError } from '../../api/errors';
import type {
  Subscriber,
  SubscriberInput,
  TacEntry,
} from '../../api/resources/subscribers';
import {
  useCreateSubscriber,
  useDeleteSubscriber,
  useSubscribers,
  useTacCatalog,
  useUpdateSubscriber,
} from '../../api/resources/subscribers';
import { SubscriberForm } from './SubscriberForm';

/**
 * Drawer styles — same pattern as Peers (flex column with its own
 * scrollable body and sticky footer). Kept as a module-level const so
 * both drawers reuse the same shape.
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

/**
 * Subscribers listing. Matches the Figma "Subscribers / Light" and
 * "Subscribers - Edit / Light" screens. Mirrors the Peers CRUD pattern:
 * search, table, kebab-to-edit, right-hand drawer, delete-via-drawer.
 *
 * No status filter and no lifecycle actions — a Subscriber is pure
 * configuration with no runtime state.
 */
export function SubscribersPage() {
  const subs = useSubscribers();
  const catalog = useTacCatalog();
  const [query, setQuery] = useState('');
  const [drawer, setDrawer] = useState<
    { kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; sub: Subscriber }
  >({ kind: 'closed' });
  const [deleteTarget, setDeleteTarget] = useState<Subscriber | undefined>();

  const filtered = useMemo(() => {
    if (!subs.data) return [];
    const q = query.trim().toLowerCase();
    if (!q) return subs.data;
    return subs.data.filter(
      (s) =>
        s.msisdn.toLowerCase().includes(q) ||
        s.iccid.toLowerCase().includes(q) ||
        (s.imei ?? '').toLowerCase().includes(q),
    );
  }, [subs.data, query]);

  return (
    <Stack gap="lg" p="md">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={4}>
          <Title order={2} fw={600}>
            Subscribers
          </Title>
          <Text c="dimmed" size="sm">
            Manage subscriber identities (MSISDN, ICCID, IMEI).
          </Text>
        </Stack>
        <Button
          leftSection={<IconPlus size={16} />}
          onClick={() => setDrawer({ kind: 'create' })}
        >
          Add subscriber
        </Button>
      </Group>

      <Group>
        <TextInput
          placeholder="Search by MSISDN, ICCID, IMEI…"
          value={query}
          onChange={(e) => setQuery(e.currentTarget.value)}
          leftSection={<IconSearch size={14} />}
          w={320}
          aria-label="Search subscribers"
        />
      </Group>

      {subs.isError ? (
        <Alert
          color="red"
          icon={<IconAlertTriangle size={18} />}
          title="Subscribers unavailable"
          variant="light"
        >
          <Stack gap="xs" align="flex-start">
            <Text size="sm" c="dimmed">
              Couldn&apos;t load subscribers. Check the API and try again.
            </Text>
            <Button
              size="xs"
              variant="subtle"
              color="red"
              onClick={() => subs.refetch()}
            >
              Retry
            </Button>
          </Stack>
        </Alert>
      ) : subs.isLoading || !subs.data ? (
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
                <Table.Th tt="uppercase" fz="xs" c="dimmed">MSISDN</Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">ICCID</Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">IMEI</Table.Th>
                <Table.Th tt="uppercase" fz="xs" c="dimmed">Device</Table.Th>
                <Table.Th w={48} aria-label="Actions" />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {filtered.length === 0 ? (
                <Table.Tr>
                  <Table.Td colSpan={5}>
                    <Text c="dimmed" ta="center" py="md" size="sm">
                      {subs.data.length === 0
                        ? 'No subscribers yet. Click Add subscriber to create one.'
                        : 'No subscribers match the current filters.'}
                    </Text>
                  </Table.Td>
                </Table.Tr>
              ) : (
                filtered.map((sub) => (
                  <SubscriberRow
                    key={sub.id}
                    sub={sub}
                    catalog={catalog.data ?? []}
                    onEdit={() => setDrawer({ kind: 'edit', sub })}
                  />
                ))
              )}
            </Table.Tbody>
          </Table>
        </Card>
      )}

      <CreateSubscriberDrawer
        open={drawer.kind === 'create'}
        catalog={catalog.data ?? []}
        onClose={() => setDrawer({ kind: 'closed' })}
      />
      <EditSubscriberDrawer
        sub={drawer.kind === 'edit' ? drawer.sub : undefined}
        catalog={catalog.data ?? []}
        onClose={() => setDrawer({ kind: 'closed' })}
        onRequestDelete={(s) => {
          setDrawer({ kind: 'closed' });
          setDeleteTarget(s);
        }}
      />
      <DeleteSubscriberModal
        sub={deleteTarget}
        onClose={() => setDeleteTarget(undefined)}
      />
    </Stack>
  );
}

/**
 * One row in the subscribers table. Resolves the TAC → manufacturer/
 * model display string from the catalogue so the "Device" column
 * matches what the edit drawer shows.
 */
function SubscriberRow({
  sub,
  catalog,
  onEdit,
}: {
  sub: Subscriber;
  catalog: TacEntry[];
  onEdit: () => void;
}) {
  const device = sub.tac
    ? catalog.find((e) => e.tac === sub.tac)?.model ?? `TAC ${sub.tac}`
    : '—';
  return (
    <Table.Tr>
      <Table.Td>
        <Text size="sm" ff="monospace" fw={600}>
          {sub.msisdn}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm" ff="monospace">
          {sub.iccid}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm" ff="monospace" c={sub.imei ? undefined : 'dimmed'}>
          {sub.imei ?? '—'}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm" c={sub.tac ? undefined : 'dimmed'}>
          {device}
        </Text>
      </Table.Td>
      <Table.Td>
        <Menu shadow="md" width={160} position="bottom-end">
          <Menu.Target>
            <ActionIcon
              variant="subtle"
              color="gray"
              aria-label={`Actions for ${sub.msisdn}`}
            >
              <IconDots size={16} />
            </ActionIcon>
          </Menu.Target>
          <Menu.Dropdown>
            <Menu.Item leftSection={<IconPencil size={14} />} onClick={onEdit}>
              Edit
            </Menu.Item>
          </Menu.Dropdown>
        </Menu>
      </Table.Td>
    </Table.Tr>
  );
}

function CreateSubscriberDrawer({
  open,
  catalog,
  onClose,
}: {
  open: boolean;
  catalog: TacEntry[];
  onClose: () => void;
}) {
  const createSub = useCreateSubscriber();

  const handleSubmit = async (values: SubscriberInput) => {
    try {
      const created = await createSub.mutateAsync(values);
      notifications.show({
        color: 'teal',
        title: 'Subscriber created',
        message: `${created.msisdn} is ready.`,
      });
      onClose();
      return created;
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) throw err;
      notifications.show({
        color: 'red',
        title: 'Could not create subscriber',
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
      closeOnClickOutside={!createSub.isPending}
      closeOnEscape={!createSub.isPending}
      styles={DRAWER_STYLES}
    >
      <SubscriberForm
        mode="create"
        catalog={catalog}
        submitting={createSub.isPending}
        onSubmit={handleSubmit}
        onCancel={onClose}
      />
    </Drawer>
  );
}

function EditSubscriberDrawer({
  sub,
  catalog,
  onClose,
  onRequestDelete,
}: {
  sub: Subscriber | undefined;
  catalog: TacEntry[];
  onClose: () => void;
  onRequestDelete: (s: Subscriber) => void;
}) {
  return (
    <Drawer
      opened={Boolean(sub)}
      onClose={onClose}
      position="right"
      size={480}
      padding="lg"
      withCloseButton
      styles={DRAWER_STYLES}
    >
      {sub && (
        <EditSubscriberForm
          sub={sub}
          catalog={catalog}
          onClose={onClose}
          onRequestDelete={() => onRequestDelete(sub)}
        />
      )}
    </Drawer>
  );
}

function EditSubscriberForm({
  sub,
  catalog,
  onClose,
  onRequestDelete,
}: {
  sub: Subscriber;
  catalog: TacEntry[];
  onClose: () => void;
  onRequestDelete: () => void;
}) {
  const updateSub = useUpdateSubscriber(sub.id);

  const handleSubmit = async (values: SubscriberInput) => {
    try {
      const next = await updateSub.mutateAsync(values);
      notifications.show({
        color: 'teal',
        title: 'Subscriber updated',
        message: `${next.msisdn} was saved.`,
      });
      onClose();
      return next;
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) throw err;
      notifications.show({
        color: 'red',
        title: 'Could not update subscriber',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
      throw err;
    }
  };

  return (
    <SubscriberForm
      mode="edit"
      initial={sub}
      catalog={catalog}
      submitting={updateSub.isPending}
      onSubmit={handleSubmit}
      onDelete={onRequestDelete}
      onCancel={onClose}
    />
  );
}

function DeleteSubscriberModal({
  sub,
  onClose,
}: {
  sub: Subscriber | undefined;
  onClose: () => void;
}) {
  const deleteSub = useDeleteSubscriber();

  const handleConfirm = async () => {
    if (!sub) return;
    try {
      await deleteSub.mutateAsync(sub.id);
      notifications.show({
        color: 'teal',
        title: 'Subscriber deleted',
        message: `${sub.msisdn} was removed.`,
      });
      onClose();
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Could not delete subscriber',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

  return (
    <Modal
      opened={Boolean(sub)}
      onClose={onClose}
      title="Delete subscriber"
      centered
      closeOnClickOutside={!deleteSub.isPending}
      closeOnEscape={!deleteSub.isPending}
    >
      <Stack gap="md">
        <Text size="sm">
          Are you sure you want to delete subscriber{' '}
          <strong>{sub?.msisdn ?? ''}</strong>? This cannot be undone.
        </Text>
        <Group justify="flex-end">
          <Button
            variant="subtle"
            onClick={onClose}
            disabled={deleteSub.isPending}
          >
            Cancel
          </Button>
          <Button
            color="red"
            loading={deleteSub.isPending}
            onClick={handleConfirm}
          >
            Delete
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
