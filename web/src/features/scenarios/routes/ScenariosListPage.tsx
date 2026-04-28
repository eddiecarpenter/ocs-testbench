import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Group,
  Menu,
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
  IconCopy,
  IconDots,
  IconPlayerPlay,
  IconPlus,
  IconSearch,
} from '@tabler/icons-react';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';

import { ApiError } from '../../../api/errors';
import { usePeers } from '../../../api/resources/peers';
import {
  useDuplicateScenario,
  useRunScenario,
  useScenarios,
} from '../api/scenarios';
import type { ScenarioSummary, UnitType } from '../store/types';

const UNIT_GROUP_ORDER: UnitType[] = ['OCTET', 'TIME', 'UNITS'];

/** Stable group order — independent of the order rows arrive in. */
function groupByUnit(rows: ScenarioSummary[]): Record<UnitType, ScenarioSummary[]> {
  const out: Record<UnitType, ScenarioSummary[]> = {
    OCTET: [],
    TIME: [],
    UNITS: [],
  };
  for (const r of rows) out[r.unitType].push(r);
  return out;
}

/**
 * Scenarios listing — replaces the legacy `/scenarios` placeholder.
 *
 * Renders rows grouped under `OCTET` / `TIME` / `UNITS` headers, with
 * search across name + peer, a peer filter, the New CTA, and per-row
 * Run + Duplicate actions. Click-row navigates into the Builder.
 *
 * The list is read-only here — every mutation goes through the API
 * helpers in `../api/scenarios.ts`, which own the cache invalidation.
 */
export function ScenariosListPage() {
  const navigate = useNavigate();
  const scenariosQuery = useScenarios();
  const peersQuery = usePeers();
  const duplicate = useDuplicateScenario();
  const run = useRunScenario();

  const [search, setSearch] = useState('');
  const [peerFilter, setPeerFilter] = useState<string | null>(null);

  /** Map peerId → peerName for the row display + the peer filter dropdown. */
  const peerMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const p of peersQuery.data ?? []) m.set(p.id, p.name);
    return m;
  }, [peersQuery.data]);

  const filtered = useMemo(() => {
    const rows = scenariosQuery.data ?? [];
    const needle = search.trim().toLowerCase();
    return rows.filter((row) => {
      if (peerFilter && row.peerId !== peerFilter) return false;
      if (!needle) return true;
      const peerName = (row.peerId && peerMap.get(row.peerId)) ?? '';
      return (
        row.name.toLowerCase().includes(needle) ||
        peerName.toLowerCase().includes(needle)
      );
    });
  }, [scenariosQuery.data, search, peerFilter, peerMap]);

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

  if (scenariosQuery.isLoading) {
    return (
      <Stack gap="md">
        <Title order={2}>Scenarios</Title>
        <Skeleton height={32} />
        <Skeleton height={140} />
      </Stack>
    );
  }

  if (scenariosQuery.isError) {
    return (
      <Alert
        icon={<IconAlertTriangle size={16} />}
        color="red"
        title="Failed to load scenarios"
      >
        {(scenariosQuery.error as ApiError | Error).message}
      </Alert>
    );
  }

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
      notifications.show({
        color: 'red',
        title: 'Run failed',
        message: (err as Error).message,
      });
    }
  };

  const handleDuplicate = async (row: ScenarioSummary) => {
    try {
      const dup = await duplicate.mutateAsync({ id: row.id });
      notifications.show({
        color: 'green',
        title: 'Scenario duplicated',
        message: `Opened "${dup.name}" in the Builder.`,
      });
      navigate(`/scenarios/${encodeURIComponent(dup.id)}`);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Duplicate failed',
        message: (err as Error).message,
      });
    }
  };

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

      {filtered.length === 0 ? (
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
                          style={{ cursor: 'pointer' }}
                          onClick={() =>
                            navigate(`/scenarios/${encodeURIComponent(row.id)}`)
                          }
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
                          <Table.Td onClick={(e) => e.stopPropagation()}>
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
                                  >
                                    <IconDots size={16} />
                                  </ActionIcon>
                                </Menu.Target>
                                <Menu.Dropdown>
                                  <Menu.Item
                                    leftSection={<IconCopy size={14} />}
                                    onClick={() => handleDuplicate(row)}
                                    data-testid={`scenarios-duplicate-${row.id}`}
                                  >
                                    Duplicate
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
    </Stack>
  );
}
