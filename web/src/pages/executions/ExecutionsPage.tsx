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
 * Subsequent tasks fill in the sidebar (Task 3), table (Task 4),
 * filters + actions (Task 5), and Start-Run dialog (Task 6/7).
 */
import {
  Alert,
  Card,
  Group,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconAlertTriangle, IconPencil } from '@tabler/icons-react';
import { Button } from '@mantine/core';
import { useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

import { ApiError } from '../../api/errors';
import { usePeers } from '../../api/resources/peers';
import { useScenarios } from '../../api/resources/scenarios';
import type { ScenarioSummary } from '../scenarios/types';

import { buildSubHeader } from './buildSubHeader';
import { selectScenarioForHeader } from './selectors';

export function ExecutionsPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const scenarioId = searchParams.get('scenario');

  const scenariosQuery = useScenarios();
  const peersQuery = usePeers();

  const peerNameById = useMemo(() => {
    const m = new Map<string, string>();
    for (const p of peersQuery.data ?? []) m.set(p.id, p.name);
    return m;
  }, [peersQuery.data]);

  const selectedScenario: ScenarioSummary | undefined = useMemo(
    () => selectScenarioForHeader(scenariosQuery.data ?? [], scenarioId),
    [scenariosQuery.data, scenarioId],
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

  return (
    <Group align="flex-start" gap="lg" wrap="nowrap" data-testid="executions-page">
      {/* Sidebar (left) — Task 3 fills this in. */}
      <Card
        withBorder
        padding="md"
        w={300}
        miw={280}
        data-testid="executions-sidebar"
      >
        <Stack gap="xs">
          <Title order={5}>Scenarios</Title>
          <Text c="dimmed" size="sm">
            Sidebar coming online next.
          </Text>
        </Stack>
      </Card>

      {/* Right pane — header + table. Table comes in Task 4. */}
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
          <Text c="dimmed" size="sm" ta="center" py="lg">
            Run table coming online next.
          </Text>
        </Card>
      </Stack>
    </Group>
  );
}

