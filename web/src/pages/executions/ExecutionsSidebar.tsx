/**
 * Sidebar for the Executions list — pinned scenarios menu.
 *
 * Top entry is "All runs" (clears the `?scenario=` URL param). Below
 * sits every scenario from `useScenarios()` with a count-of-runs
 * badge and the last-run relative timestamp. A search input filters
 * case-insensitively across scenario name. Selecting a scenario
 * writes `?scenario=<id>` so the page state round-trips through the
 * URL.
 *
 * Pure helpers (counts, last-run, name filter) live in `selectors.ts`
 * so tests don't need React.
 */
import {
  Badge,
  Box,
  Group,
  ScrollArea,
  Stack,
  Text,
  TextInput,
  Title,
  UnstyledButton,
} from '@mantine/core';
import { IconSearch } from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import type { ExecutionSummary } from '../../api/resources/executions';
import type { ScenarioSummary } from '../scenarios/types';

import { formatLastRun } from './formatLastRun';
import {
  countRunsByScenario,
  filterScenariosByName,
  lastRunByScenario,
} from './selectors';

interface ExecutionsSidebarProps {
  /** Scenarios from `useScenarios()`. */
  scenarios: readonly ScenarioSummary[];
  /** Executions from `useExecutions()` — drives the count + last-run badges. */
  executions: readonly ExecutionSummary[];
  /** Currently selected scenario id, or `null` for "All runs". */
  selectedScenarioId: string | null;
  /**
   * Fired when the user clicks a sidebar entry. Pass `null` for the
   * "All runs" entry. The parent page reflects this into the URL.
   */
  onSelect(scenarioId: string | null): void;
}

export function ExecutionsSidebar({
  scenarios,
  executions,
  selectedScenarioId,
  onSelect,
}: ExecutionsSidebarProps) {
  const [search, setSearch] = useState('');

  const counts = useMemo(() => countRunsByScenario(executions), [executions]);
  const lastRun = useMemo(() => lastRunByScenario(executions), [executions]);
  const visibleScenarios = useMemo(
    () => filterScenariosByName(scenarios, search),
    [scenarios, search],
  );

  const totalRuns = executions.length;

  return (
    <Stack gap="sm" data-testid="executions-sidebar-stack">
      <Title order={5}>Scenarios</Title>

      <TextInput
        placeholder="Search scenarios"
        leftSection={<IconSearch size={14} />}
        value={search}
        onChange={(e) => setSearch(e.currentTarget.value)}
        data-testid="executions-sidebar-search"
      />

      <SidebarRow
        label="All runs"
        sub="Across every scenario"
        count={totalRuns}
        active={selectedScenarioId === null}
        onClick={() => onSelect(null)}
        testid="executions-sidebar-all"
      />

      <ScrollArea.Autosize mah={520} type="hover">
        <Stack gap={4}>
          {visibleScenarios.length === 0 && search.trim().length > 0 ? (
            <Text c="dimmed" size="sm" ta="center" py="md">
              No scenarios match "{search}".
            </Text>
          ) : (
            visibleScenarios.map((s) => (
              <SidebarRow
                key={s.id}
                label={s.name}
                sub={formatLastRun(lastRun.get(s.id))}
                count={counts.get(s.id) ?? 0}
                active={selectedScenarioId === s.id}
                onClick={() => onSelect(s.id)}
                testid={`executions-sidebar-row-${s.id}`}
              />
            ))
          )}
        </Stack>
      </ScrollArea.Autosize>
    </Stack>
  );
}

interface SidebarRowProps {
  label: string;
  sub: string;
  count: number;
  active: boolean;
  onClick(): void;
  testid: string;
}

function SidebarRow({
  label,
  sub,
  count,
  active,
  onClick,
  testid,
}: SidebarRowProps) {
  return (
    <UnstyledButton
      onClick={onClick}
      data-testid={testid}
      data-active={active || undefined}
      px="sm"
      py="xs"
      style={{
        borderRadius: 'var(--mantine-radius-sm)',
        backgroundColor: active
          ? 'var(--mantine-color-blue-light)'
          : 'transparent',
      }}
    >
      <Group justify="space-between" wrap="nowrap" gap="xs">
        <Box style={{ minWidth: 0, flex: 1 }}>
          <Text fw={active ? 600 : 500} truncate size="sm">
            {label}
          </Text>
          <Text c="dimmed" size="xs" truncate>
            {sub}
          </Text>
        </Box>
        <Badge variant={active ? 'filled' : 'light'} size="sm">
          {count}
        </Badge>
      </Group>
    </UnstyledButton>
  );
}

