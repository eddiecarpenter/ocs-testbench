/**
 * Scenarios listing — replaces the legacy `/scenarios` placeholder.
 *
 * Renders rows grouped under `OCTET` / `TIME` / `UNITS` headers, with
 * search across name + peer, a peer filter, the New CTA, and per-row
 * Run / Edit / Duplicate / Delete actions in the kebab menu. Row
 * click does NOT navigate — editing always goes through the kebab.
 *
 * The same component handles `/scenarios`, `/scenarios/new`, and
 * `/scenarios/:id` — it always renders the list, and conditionally
 * mounts the full-screen `<ScenarioBuilderPage>` modal when the URL
 * indicates a create / edit / duplicate flow.
 *
 * The list is read-only here — every mutation goes through the API
 * helpers in `../../api/resources/scenarios.ts`, which own the cache
 * invalidation.
 */
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
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
import { notifyError } from '../../utils/notify';
import {
  IconAlertTriangle,
  IconCopy,
  IconDots,
  IconPencil,
  IconPlayerPlay,
  IconPlus,
  IconSearch,
  IconTrash,
} from '@tabler/icons-react';
import { useMemo, useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router';

import { ApiError } from '../../api/errors';
import { usePeers } from '../../api/resources/peers';
import {
  useDeleteScenario,
  useRunScenario,
  useScenarios,
} from '../../api/resources/scenarios';
import {
  UNIT_GROUP_ORDER,
  filterScenarios,
  groupByUnit,
} from './listSelectors';
import { ScenarioBuilderPage } from './ScenarioBuilderPage';
import type { ScenarioSummary } from './types';

export function ScenariosListPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { id: routeId } = useParams<{ id?: string }>();
  const isEditorOpen =
    location.pathname === '/scenarios/new' || Boolean(routeId);

  const scenariosQuery = useScenarios();
  const peersQuery = usePeers();
  const run = useRunScenario();
  const remove = useDeleteScenario();

  const [search, setSearch] = useState('');
  const [peerFilter, setPeerFilter] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ScenarioSummary | null>(
    null,
  );

  /** Map peerId → peerName for the row display + the peer filter dropdown. */
  const peerMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const p of peersQuery.data ?? []) m.set(p.id, p.name);
    return m;
  }, [peersQuery.data]);

  const filtered = useMemo(
    () =>
      filterScenarios(scenariosQuery.data ?? [], {
        search,
        peerFilter,
        peerNameById: peerMap,
      }),
    [scenariosQuery.data, search, peerFilter, peerMap],
  );

  const grouped = useMemo(() => groupByUnit(filtered), [filtered]);

  const peerOptions = useMemo(() => {
    const ids = new Set<string>();
    for (const r of scenariosQuery.data ?? []) {
      if (r.peerId) ids.add(r.peerId);
    }
    return Array.from(ids).map((id) => ({
      value: id,
      label: peerMap.get(id) ?? id,
    }));
  }, [scenariosQuery.data, peerMap]);

  // ---------------------------------------------------------------------------
  // Row actions
  // ---------------------------------------------------------------------------

  const handleEdit = (row: ScenarioSummary) => {
    navigate(`/scenarios/${encodeURIComponent(row.id)}`);
  };

  const handleRun = async (row: ScenarioSummary) => {
    try {
      const res = await run.mutateAsync(row.id);
      const id = res.items[0]?.id ?? '(no id)';
      notifications.show({
        color: 'green',
        title: 'Run intent fired',
        message: `Execution ${id} started for ${row.name}`,
      });
    } catch (err) {
      notifyError({
        title: 'Run failed',
        message: (err as Error).message,
      });
    }
  };

  /**
   * Duplicate opens a fresh editor pre-filled with the source's data.
   * NO API call — nothing is persisted until the user hits Save in the
   * editor (so Discard truly throws everything away).
   */
  const handleDuplicate = (row: ScenarioSummary) => {
    navigate(
      `/scenarios/new?dup=${encodeURIComponent(row.id)}`,
    );
  };

  const handleDeleteRequest = (row: ScenarioSummary) => {
    setPendingDelete(row);
  };

  const handleDeleteConfirm = async () => {
    if (!pendingDelete) return;
    try {
      await remove.mutateAsync(pendingDelete.id);
      notifications.show({
        color: 'teal',
        title: 'Scenario deleted',
        message: pendingDelete.name,
      });
      setPendingDelete(null);
    } catch (err) {
      notifyError({
        title: 'Could not delete scenario',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <Stack gap="md">
      <Group justify="space-between">
        <Title order={2}>Scenarios</Title>
        <Button
          leftSection={<IconPlus size={16} />}
          onClick={() => navigate('/scenarios/new')}
          data-testid="scenarios-new"
        >
          New scenario
        </Button>
      </Group>

      <Group gap="sm">
        <TextInput
          placeholder="Search by name or peer"
          leftSection={<IconSearch size={14} />}
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          style={{ flex: 1 }}
          data-testid="scenarios-search"
        />
        <Select
          placeholder="All peers"
          data={peerOptions}
          value={peerFilter}
          onChange={setPeerFilter}
          clearable
          w={220}
          data-testid="scenarios-peer-filter"
        />
      </Group>

      {scenariosQuery.isLoading ? (
        <Stack gap="md">
          <Skeleton height={32} />
          <Skeleton height={140} />
        </Stack>
      ) : scenariosQuery.isError ? (
        <Alert
          icon={<IconAlertTriangle size={16} />}
          color="red"
          title="Failed to load scenarios"
        >
          {(scenariosQuery.error as ApiError | Error).message}
        </Alert>
      ) : filtered.length === 0 ? (
        <Card withBorder padding="lg">
          <Text ta="center" c="dimmed">
            No scenarios match the current filters.
          </Text>
        </Card>
      ) : (
        <Stack gap="lg">
          {UNIT_GROUP_ORDER.map((unit) => {
            const rows = grouped[unit];
            if (rows.length === 0) return null;
            return (
              <Card withBorder padding="md" key={unit}>
                <Stack gap="xs">
                  <Group justify="space-between">
                    <Group gap="xs">
                      <Title order={4}>{unit}</Title>
                      <Badge variant="light">{rows.length}</Badge>
                    </Group>
                  </Group>
                  <Table highlightOnHover>
                    <Table.Thead>
                      <Table.Tr>
                        <Table.Th>Name</Table.Th>
                        <Table.Th>Peer</Table.Th>
                        <Table.Th>Steps</Table.Th>
                        <Table.Th>Last updated</Table.Th>
                        <Table.Th aria-label="Actions" />
                      </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                      {rows.map((row) => (
                        <Table.Tr
                          key={row.id}
                          data-testid={`scenarios-row-${row.id}`}
                        >
                          <Table.Td>
                            <Group gap="xs">
                              <Text fw={500}>{row.name}</Text>
                              {row.origin === 'system' && (
                                <Badge variant="outline" size="sm">
                                  system
                                </Badge>
                              )}
                            </Group>
                          </Table.Td>
                          <Table.Td>
                            {row.peerId
                              ? peerMap.get(row.peerId) ?? row.peerId
                              : '—'}
                          </Table.Td>
                          <Table.Td>{row.stepCount}</Table.Td>
                          <Table.Td>
                            {new Date(row.updatedAt).toLocaleString()}
                          </Table.Td>
                          <Table.Td>
                            <Group gap={4} justify="flex-end" wrap="nowrap">
                              <ActionIcon
                                variant="subtle"
                                color="green"
                                aria-label="Run scenario"
                                onClick={() => handleRun(row)}
                                data-testid={`scenarios-run-${row.id}`}
                              >
                                <IconPlayerPlay size={16} />
                              </ActionIcon>
                              <Menu position="bottom-end" withinPortal>
                                <Menu.Target>
                                  <ActionIcon
                                    variant="subtle"
                                    aria-label="More actions"
                                    data-testid={`scenarios-more-${row.id}`}
                                  >
                                    <IconDots size={16} />
                                  </ActionIcon>
                                </Menu.Target>
                                <Menu.Dropdown>
                                  <Menu.Item
                                    leftSection={<IconPencil size={14} />}
                                    onClick={() => handleEdit(row)}
                                    data-testid={`scenarios-edit-${row.id}`}
                                  >
                                    Edit
                                  </Menu.Item>
                                  <Menu.Item
                                    leftSection={<IconCopy size={14} />}
                                    onClick={() => handleDuplicate(row)}
                                    data-testid={`scenarios-duplicate-${row.id}`}
                                  >
                                    Duplicate
                                  </Menu.Item>
                                  <Menu.Divider />
                                  <Menu.Item
                                    color="red"
                                    leftSection={<IconTrash size={14} />}
                                    onClick={() => handleDeleteRequest(row)}
                                    disabled={row.origin === 'system'}
                                    data-testid={`scenarios-delete-${row.id}`}
                                  >
                                    Delete
                                  </Menu.Item>
                                </Menu.Dropdown>
                              </Menu>
                            </Group>
                          </Table.Td>
                        </Table.Tr>
                      ))}
                    </Table.Tbody>
                  </Table>
                </Stack>
              </Card>
            );
          })}
        </Stack>
      )}

      {isEditorOpen && <ScenarioBuilderPage />}

      <Modal
        opened={Boolean(pendingDelete)}
        onClose={() => setPendingDelete(null)}
        title="Delete scenario"
        centered
        closeOnClickOutside={!remove.isPending}
        closeOnEscape={!remove.isPending}
      >
        <Stack gap="md">
          <Text size="sm">
            Are you sure you want to delete{' '}
            <strong>{pendingDelete?.name ?? ''}</strong>? This cannot be
            undone.
          </Text>
          <Group justify="flex-end">
            <Button
              variant="subtle"
              onClick={() => setPendingDelete(null)}
              disabled={remove.isPending}
            >
              Cancel
            </Button>
            <Button
              color="red"
              loading={remove.isPending}
              onClick={handleDeleteConfirm}
              data-testid="scenarios-delete-confirm"
            >
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
