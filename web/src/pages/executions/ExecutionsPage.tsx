/**
 * Executions list — replaces the legacy `/executions` placeholder.
 *
 * Two-pane shell: a scenario sidebar on the left (selection drives the
 * right-pane filter), a header + table on the right. URL search params
 * own the page state so deep-links and refresh round-trip cleanly:
 *
 *   ?scenario=<id>           — sidebar selection (omit = "All runs")
 *   &state=<state>           — filter chip selection (matches OpenAPI v0.2)
 *   &peer=<id>               — peer filter dropdown
 *
 * Subsequent tasks fill in the table (Task 4), filters + actions
 * (Task 5), and Start-Run dialog (Tasks 6 / 7).
 */
import {
  Alert,
  Button,
  Card,
  Group,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconAlertTriangle, IconPencil } from '@tabler/icons-react';
import { useCallback, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

import { ApiError } from '../../api/errors';
import { usePeers } from '../../api/resources/peers';
import { useExecutions } from '../../api/resources/executions';
import { useScenarios } from '../../api/resources/scenarios';
import type { ScenarioSummary } from '../scenarios/types';

import { buildSubHeader } from './buildSubHeader';
import { ExecutionsSidebar } from './ExecutionsSidebar';
import { ExecutionsTable } from './ExecutionsTable';
import { selectScenarioForHeader } from './selectors';

export function ExecutionsPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const scenarioId = searchParams.get('scenario');

  const scenariosQuery = useScenarios();
  const peersQuery = usePeers();
  // The sidebar needs the full execution set to compute per-scenario
  // counts and last-run timestamps; the right-pane table consumes a
  // filtered slice driven by the URL `?scenario=` param.
  const allExecutionsQuery = useExecutions({ limit: 500 });
  const tableQuery = useExecutions(
    scenarioId ? { scenarioId, limit: 500 } : { limit: 500 },
  );

  const peerNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const p of peersQuery.data ?? []) m.set(p.id, p.name);
    return m;
  }, [peersQuery.data]);

  const selectedScenario: ScenarioSummary | undefined = useMemo(
    () => selectScenarioForHeader(scenariosQuery.data ?? [], scenarioId),
    [scenariosQuery.data, scenarioId],
  );

  const handleSidebarSelect = useCallback(
    (nextId: string | null) => {
      const next = new URLSearchParams(searchParams);
      if (nextId === null) {
        next.delete('scenario');
      } else {
        next.set('scenario', nextId);
      }
      setSearchParams(next, { replace: false });
    },
    [searchParams, setSearchParams],
  );

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

  const sidebarLoading =
    scenariosQuery.isLoading || allExecutionsQuery.isLoading;

  return (
    <Group align="flex-start" gap="lg" wrap="nowrap" data-testid="executions-page">
      <Card
        withBorder
        padding="md"
        w={320}
        miw={280}
        data-testid="executions-sidebar"
      >
        {sidebarLoading ? (
          <Stack gap="xs">
            <Skeleton height={28} />
            <Skeleton height={36} />
            <Skeleton height={120} />
          </Stack>
        ) : (
          <ExecutionsSidebar
            scenarios={scenariosQuery.data ?? []}
            executions={allExecutionsQuery.data?.items ?? []}
            selectedScenarioId={scenarioId ?? null}
            onSelect={handleSidebarSelect}
          />
        )}
      </Card>

      <Stack gap="md" style={{ flex: 1, minWidth: 0 }}>
        <Group justify="space-between" align="flex-start" wrap="nowrap">
          <Stack gap={2}>
            <Title order={2} data-testid="executions-header-title">
              {selectedScenario?.name ?? 'All runs'}
            </Title>
            {selectedScenario ? (
              <Text c="dimmed" size="sm" data-testid="executions-header-subtitle">
                {buildSubHeader(selectedScenario, peerNameById)}
              </Text>
            ) : (
              <Text c="dimmed" size="sm" data-testid="executions-header-subtitle">
                Runs across every scenario.
              </Text>
            )}
          </Stack>

          {selectedScenario && (
            <Button
              variant="default"
              leftSection={<IconPencil size={14} />}
              onClick={() =>
                navigate(`/scenarios/${encodeURIComponent(selectedScenario.id)}`)
              }
              data-testid="executions-edit-scenario"
            >
              Edit scenario
            </Button>
          )}
        </Group>

        <Card withBorder padding="md" data-testid="executions-table">
          {tableQuery.isLoading ? (
            <Stack gap="xs">
              <Skeleton height={40} />
              <Skeleton height={120} />
            </Stack>
          ) : tableQuery.isError ? (
            <Alert
              icon={<IconAlertTriangle size={16} />}
              color="red"
              title="Failed to load executions"
            >
              {(tableQuery.error as ApiError | Error).message}
            </Alert>
          ) : (
            <ExecutionsTable
              executions={tableQuery.data?.items ?? []}
              showScenarioColumn={!scenarioId}
            />
          )}
        </Card>
      </Stack>
    </Group>
  );
}
