/**
 * Executions list — replaces the legacy `/executions` placeholder.
 *
 * Two-pane shell: a scenario sidebar on the left (selection drives the
 * right-pane filter), a header + filter bar + run table on the right.
 * URL search params own the page state so deep-links and refresh
 * round-trip cleanly:
 *
 *   ?scenario=<id>           — sidebar selection (omit = "All runs")
 *   &status=<filter>         — filter chip selection (all / running /
 *                              completed / failed)
 *   &peer=<id>               — peer filter dropdown
 *
 * Task 6 / 7 plug in the Start-Run dialog; this task wires up the
 * filter chips, peer dropdown, header CTAs, and per-row Re-run
 * affordances.
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
import { notifications } from '@mantine/notifications';
import {
  IconAlertTriangle,
  IconPencil,
  IconPlayerPlay,
  IconRotate,
} from '@tabler/icons-react';
import { useCallback, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

import { useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../../api/errors';
import { usePeers } from '../../api/resources/peers';
import {
  executionKeys,
  useCreateExecution,
  useExecutions,
  useRerunExecution,
  type ExecutionPage,
  type ExecutionSummary,
  type StartExecutionInput,
} from '../../api/resources/executions';
import { useScenarios } from '../../api/resources/scenarios';
import type { ScenarioSummary } from '../scenarios/types';

import { buildSubHeader } from './buildSubHeader';
import { ExecutionsFilterBar } from './ExecutionsFilterBar';
import { ExecutionsSidebar } from './ExecutionsSidebar';
import { ExecutionsTable } from './ExecutionsTable';
import { prependExecutions } from './optimisticPrepend';
import { RerunConfirmModal } from './RerunConfirmModal';
import { StartRunModal } from './StartRunModal';
import {
  applyTableFilters,
  countByStatusFilter,
  parseStatusFilter,
  selectLatestRunForScenario,
  selectScenarioForHeader,
} from './selectors';

export function ExecutionsPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const scenarioId = searchParams.get('scenario');
  const statusFilter = parseStatusFilter(searchParams.get('status'));
  const peerFilter = searchParams.get('peer');

  const scenariosQuery = useScenarios();
  const peersQuery = usePeers();
  // The sidebar needs the full execution set to compute per-scenario
  // counts and last-run timestamps; the right-pane table consumes a
  // filtered slice driven by the URL `?scenario=` param.
  const allExecutionsQuery = useExecutions({ limit: 500 });
  const tableQuery = useExecutions(
    scenarioId ? { scenarioId, limit: 500 } : { limit: 500 },
  );
  const rerun = useRerunExecution();
  const create = useCreateExecution();
  const queryClient = useQueryClient();

  const [pendingRerun, setPendingRerun] = useState<ExecutionSummary | null>(
    null,
  );
  const [startRunFor, setStartRunFor] = useState<ScenarioSummary | null>(null);
  const [startRunErrors, setStartRunErrors] = useState<Record<string, string>>(
    {},
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

  const tableRows = useMemo(
    () => tableQuery.data?.items ?? [],
    [tableQuery.data],
  );

  const filteredRows = useMemo(
    () =>
      applyTableFilters(tableRows, {
        status: statusFilter,
        peerId: peerFilter,
      }),
    [tableRows, statusFilter, peerFilter],
  );

  const chipCounts = useMemo(() => countByStatusFilter(tableRows), [tableRows]);

  const peerOptions = useMemo(() => {
    const ids = new Set<string>();
    for (const r of tableRows) {
      if (r.peerId) ids.add(r.peerId);
    }
    return Array.from(ids).map((id) => ({
      value: id,
      label: peerNameById.get(id) ?? id,
    }));
  }, [tableRows, peerNameById]);

  const latestRunForSelected = useMemo(
    () =>
      selectedScenario
        ? selectLatestRunForScenario(tableRows, selectedScenario.id)
        : undefined,
    [selectedScenario, tableRows],
  );

  // ---------------------------------------------------------------------------
  // URL helpers
  // ---------------------------------------------------------------------------

  const setParam = useCallback(
    (key: string, value: string | null) => {
      const next = new URLSearchParams(searchParams);
      if (value === null) {
        next.delete(key);
      } else {
        next.set(key, value);
      }
      setSearchParams(next, { replace: false });
    },
    [searchParams, setSearchParams],
  );

  const handleSidebarSelect = useCallback(
    (nextId: string | null) => setParam('scenario', nextId),
    [setParam],
  );

  // ---------------------------------------------------------------------------
  // Run-launch handlers
  // ---------------------------------------------------------------------------

  const launchRerun = useCallback(
    async (source: ExecutionSummary) => {
      try {
        const res = await rerun.mutateAsync(source.id);
        const newId = res.items[0]?.id;
        notifications.show({
          color: 'green',
          title: `Re-running #${source.id}…`,
          message: newId
            ? `New run #${newId} queued for ${source.scenarioName}`
            : `New run queued for ${source.scenarioName}`,
        });
        if (newId && source.mode === 'interactive') {
          navigate(`/executions/${encodeURIComponent(newId)}`);
        }
      } catch (err) {
        notifications.show({
          color: 'red',
          title: 'Re-run failed',
          message: (err as ApiError | Error).message,
        });
      }
    },
    [rerun, navigate],
  );

  const handleRerunRow = useCallback(
    (row: ExecutionSummary) => {
      if (row.mode === 'continuous') {
        setPendingRerun(row);
      } else {
        void launchRerun(row);
      }
    },
    [launchRerun],
  );

  const handleRerunLatest = useCallback(() => {
    if (!latestRunForSelected) return;
    void launchRerun(latestRunForSelected);
  }, [launchRerun, latestRunForSelected]);

  const handleStartRun = useCallback(() => {
    setStartRunErrors({});
    if (selectedScenario) {
      setStartRunFor(selectedScenario);
      return;
    }
    // No scenario picked in the sidebar: the dialog must be pre-filled,
    // so prompt the operator to choose one first.
    notifications.show({
      color: 'yellow',
      title: 'Pick a scenario first',
      message:
        'Select a scenario in the left pane before launching a Run.',
    });
  }, [selectedScenario]);

  /**
   * POST /executions and branch on the server response.
   *
   *   Interactive success: navigate to /executions/<new id> (lands on
   *   F2's stub for now).
   *
   *   Continuous success: optimistically prepend the new rows onto
   *   every cached `useExecutions(...)` slice so the new row shows
   *   "at the top" even before the next refetch lands; then
   *   invalidate so the source-of-truth refetches in the background.
   *   On failure the optimistic write is rolled back via
   *   invalidateQueries (the next fetch supersedes it).
   *
   *   Validation error (RFC 7807, 422): surface field-level errors
   *   under the matching inputs; keep the dialog open. Reuse the
   *   ApiError pattern from ScenarioBuilderPage.handleSave.
   *
   *   Other error: toast and keep the dialog open.
   */
  const handleStartRunSubmit = useCallback(
    async (input: StartExecutionInput) => {
      setStartRunErrors({});
      try {
        const result = await create.mutateAsync(input);
        const created = result.items;
        if (created.length === 0) return;

        if (input.mode === 'interactive') {
          const first = created[0];
          notifications.show({
            color: 'green',
            title: 'Run started',
            message: `#${first.id} — ${first.scenarioName}`,
          });
          setStartRunFor(null);
          navigate(`/executions/${encodeURIComponent(first.id)}`);
          return;
        }

        // Continuous: optimistically prepend onto every list cache so
        // the row appears at the top instantly. The subsequent
        // invalidate refetches authoritative state in the background;
        // on failure the cache is reconciled by the same invalidate.
        queryClient.setQueriesData<ExecutionPage>(
          { queryKey: executionKeys.all },
          (prev) => prependExecutions(prev, created),
        );
        void queryClient.invalidateQueries({ queryKey: executionKeys.all });

        notifications.show({
          color: 'green',
          title: 'Continuous run started',
          message:
            created.length > 1
              ? `${created.length} workers queued for ${created[0].scenarioName}`
              : `#${created[0].id} — ${created[0].scenarioName}`,
        });
        setStartRunFor(null);
      } catch (err) {
        const apiErr = err as ApiError;
        if (apiErr instanceof ApiError && apiErr.errors) {
          setStartRunErrors(apiErr.fieldErrors());
          notifications.show({
            color: 'red',
            title: 'Run failed',
            message: 'Validation errors — see fields below.',
          });
        } else {
          notifications.show({
            color: 'red',
            title: 'Run failed',
            message: (err as Error).message,
          });
        }
      }
    },
    [create, navigate, queryClient],
  );

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

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

          <Group gap="xs" wrap="nowrap">
            {selectedScenario && (
              <>
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
                <Button
                  variant="default"
                  leftSection={<IconRotate size={14} />}
                  onClick={handleRerunLatest}
                  disabled={!latestRunForSelected || rerun.isPending}
                  loading={rerun.isPending}
                  data-testid="executions-rerun-latest"
                >
                  Re-run latest
                </Button>
              </>
            )}
            <Button
              leftSection={<IconPlayerPlay size={14} />}
              onClick={handleStartRun}
              disabled={!selectedScenario}
              data-testid="executions-start-run"
            >
              Run scenario
            </Button>
          </Group>
        </Group>

        <ExecutionsFilterBar
          status={statusFilter}
          counts={chipCounts}
          onStatusChange={(next) =>
            setParam('status', next === 'all' ? null : next)
          }
          peerOptions={peerOptions}
          peerId={peerFilter}
          onPeerChange={(next) => setParam('peer', next)}
        />

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
              executions={filteredRows}
              showScenarioColumn={!scenarioId}
              onRerunRow={handleRerunRow}
            />
          )}
        </Card>
      </Stack>

      <RerunConfirmModal
        source={pendingRerun}
        isPending={rerun.isPending}
        onClose={() => {
          if (!rerun.isPending) setPendingRerun(null);
        }}
        onConfirm={() => {
          if (!pendingRerun) return;
          void launchRerun(pendingRerun).finally(() => setPendingRerun(null));
        }}
      />

      <StartRunModal
        scenario={startRunFor}
        isPending={create.isPending}
        onClose={() => {
          if (!create.isPending) {
            setStartRunFor(null);
            setStartRunErrors({});
          }
        }}
        onSubmit={handleStartRunSubmit}
        fieldErrors={startRunErrors}
      />
    </Group>
  );
}
