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
 *
 * Peers used to be filterable via a dropdown but were dropped — peer
 * is now a sortable column on the table itself. Re-run goes through
 * the Start-Run dialog (no silent re-run); Stop confirms via modal.
 */
import {
  Alert,
  Button,
  Card,
  Group,
  Modal,
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
} from '@tabler/icons-react';
import { useCallback, useMemo, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';

import { useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../../api/errors';
import { usePeers } from '../../api/resources/peers';
import {
  executionKeys,
  useAbortExecution,
  useCreateExecution,
  useExecutions,
  type ExecutionPage,
  type ExecutionSummary,
  type StartExecutionInput,
} from '../../api/resources/executions';
import { useScenarios } from '../../api/resources/scenarios';
import { notifyError } from '../../utils/notify';
import type { ScenarioSummary } from '../scenarios/types';

import { buildSubHeader } from './buildSubHeader';
import { ExecutionsFilterBar } from './ExecutionsFilterBar';
import { ExecutionsSidebar } from './ExecutionsSidebar';
import { ExecutionsTable } from './ExecutionsTable';
import { prependExecutions } from './optimisticPrepend';
import { StartRunModal } from './StartRunModal';
import {
  applyTableFilters,
  countByStatusFilter,
  parseStatusFilter,
  selectScenarioForHeader,
} from './selectors';

export function ExecutionsPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();

  const scenarioId = searchParams.get('scenario');
  const statusFilter = parseStatusFilter(searchParams.get('status'));

  const scenariosQuery = useScenarios();
  const peersQuery = usePeers();
  // The sidebar needs the full execution set to compute per-scenario
  // counts and last-run timestamps; the right-pane table consumes a
  // filtered slice driven by the URL `?scenario=` param.
  const allExecutionsQuery = useExecutions({ limit: 500 });
  const tableQuery = useExecutions(
    scenarioId ? { scenarioId, limit: 500 } : { limit: 500 },
  );
  const create = useCreateExecution();
  const abort = useAbortExecution();
  const queryClient = useQueryClient();

  const [pendingStop, setPendingStop] = useState<ExecutionSummary | null>(null);
  const [startRunFor, setStartRunFor] = useState<ScenarioSummary | null>(null);
  const [startRunErrors, setStartRunErrors] = useState<Record<string, string>>(
    {},
  );
  // Pre-fill values + dialog-shape options for the next StartRunModal
  // open. Driven by the entry point: "Run as continuous batch…",
  // "Re-run with overrides…", etc. Cleared on close.
  const [startRunInitial, setStartRunInitial] = useState<
    import('./StartRunModal').StartRunInitialValues | undefined
  >(undefined);

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
    () => applyTableFilters(tableRows, { status: statusFilter }),
    [tableRows, statusFilter],
  );

  const chipCounts = useMemo(() => countByStatusFilter(tableRows), [tableRows]);


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

  /**
   * "View" kebab item — opens the Debugger route for the row. Replaces
   * the previous row-onClick navigation; matches the kebab-driven
   * pattern used on Peers / Subscribers / Scenarios.
   */
  const handleViewRow = useCallback(
    (row: ExecutionSummary) => {
      navigate(`/executions/${encodeURIComponent(row.id)}`);
    },
    [navigate],
  );

  /**
   * "Stop" kebab item (running rows only) — opens a confirm modal.
   * Confirmation fires `POST /executions/:id/abort` and the row's
   * status flips to `aborted` after the next refetch.
   */
  const handleStopRow = useCallback((row: ExecutionSummary) => {
    setPendingStop(row);
  }, []);

  const confirmStop = useCallback(async () => {
    if (!pendingStop) return;
    try {
      await abort.mutateAsync(pendingStop.id);
      notifications.show({
        color: 'orange',
        title: `Run #${pendingStop.id} stopped`,
        message: pendingStop.scenarioName,
      });
      setPendingStop(null);
    } catch (err) {
      notifyError({
        title: 'Stop failed',
        message: (err as ApiError | Error).message,
      });
    }
  }, [abort, pendingStop]);


  /**
   * "Run scenario" — opens the Start-Run dialog with Interactive
   * defaults so the user reviews mode + overrides before launching.
   * Continuous mode and peer/subscriber overrides are one click away
   * inside the dialog.
   */
  const handleStartRun = useCallback(() => {
    if (!selectedScenario) {
      notifications.show({
        color: 'yellow',
        title: 'Pick a scenario first',
        message: 'Select a scenario in the left pane before launching a Run.',
      });
      return;
    }
    setStartRunErrors({});
    setStartRunInitial({
      mode: 'continuous',
      concurrency: 1,
      repeats: 10,
      overrideExpanded: false,
      title: 'Run scenario',
      instance: 'fresh',
    });
    setStartRunFor(selectedScenario);
  }, [selectedScenario]);

  /**
   * "Re-run" from a row kebab — opens the Start-Run dialog pre-filled
   * with the source row's mode + bindings, override section expanded.
   * Every re-run goes through the dialog (the previous "silent"
   * re-run was removed) so the user can confirm or tweak parameters.
   */
  const handleRerunRow = useCallback(
    (row: ExecutionSummary) => {
      const sourceScenario = scenariosQuery.data?.find(
        (s) => s.id === row.scenarioId,
      );
      if (!sourceScenario) {
        notifyError({
          title: 'Scenario not found',
          message: `Source scenario for run #${row.id} is no longer available.`,
        });
        return;
      }
      setStartRunErrors({});
      setStartRunInitial({
        mode: row.mode,
        peerId: row.peerId ?? null,
        // ExecutionSummary has a single subscriberId; the dialog only
        // edits one, matching the row.
        subscriberId: null,
        concurrency: 1,
        repeats: 10,
        overrideExpanded: true,
        title: `Re-run #${row.id}`,
        instance: `rerun-${row.id}`,
      });
      setStartRunFor(sourceScenario);
    },
    [scenariosQuery.data],
  );

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
          setStartRunInitial(undefined);
          navigate(`/executions/${encodeURIComponent(first.id)}`);
          return;
        }

        // Continuous: optimistically prepend onto every list cache so
        // the row appears at the top instantly. The subsequent
        // invalidate refetches authoritative state in the background;
        // on failure the cache is reconciled by the same invalidate.
        //
        // Scope the update to list-keyed caches only — `executionKeys.all`
        // is a prefix that also matches `detail(id)` entries (whose data
        // is an `Execution`, not an `ExecutionPage`), and spreading
        // `prev.items` on those would throw.
        queryClient.setQueriesData<ExecutionPage>(
          { queryKey: [...executionKeys.all, 'list'] },
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
        setStartRunInitial(undefined);
      } catch (err) {
        const apiErr = err as ApiError;
        if (apiErr instanceof ApiError && apiErr.errors) {
          setStartRunErrors(apiErr.fieldErrors());
          notifyError({
            title: 'Run failed',
            message: 'Validation errors — see fields below.',
          });
        } else {
          notifyError({
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
                    navigate(
                      `/scenarios/${encodeURIComponent(selectedScenario.id)}`,
                      // Carry the current Executions URL (incl. ?scenario / ?status)
                      // so the Builder modal can navigate the user back here when they close.
                      {
                        state: {
                          returnTo: `/executions${location.search}`,
                        },
                      },
                    )
                  }
                  data-testid="executions-edit-scenario"
                >
                  Edit scenario
                </Button>
              </>
            )}
            <Button
              leftSection={<IconPlayerPlay size={14} />}
              onClick={handleStartRun}
              disabled={!selectedScenario || create.isPending}
              loading={create.isPending}
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
              onViewRow={handleViewRow}
              onRerunRow={handleRerunRow}
              onStopRow={handleStopRow}
            />
          )}
        </Card>
      </Stack>

      <Modal
        opened={Boolean(pendingStop)}
        onClose={() => {
          if (!abort.isPending) setPendingStop(null);
        }}
        title="Stop execution"
        centered
        closeOnClickOutside={!abort.isPending}
        closeOnEscape={!abort.isPending}
      >
        <Stack gap="md">
          <Text size="sm">
            Stop run <strong>#{pendingStop?.id ?? ''}</strong> for{' '}
            <strong>{pendingStop?.scenarioName ?? ''}</strong>? The engine
            transitions the run to <code>aborted</code>; in-flight CCRs are
            terminated best-effort. This cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button
              variant="subtle"
              onClick={() => setPendingStop(null)}
              disabled={abort.isPending}
            >
              Cancel
            </Button>
            <Button
              color="red"
              onClick={confirmStop}
              loading={abort.isPending}
              data-testid="executions-stop-confirm"
            >
              Stop
            </Button>
          </Group>
        </Stack>
      </Modal>

      <StartRunModal
        scenario={startRunFor}
        initial={startRunInitial}
        isPending={create.isPending}
        onClose={() => {
          if (!create.isPending) {
            setStartRunFor(null);
            setStartRunErrors({});
            setStartRunInitial(undefined);
          }
        }}
        onSubmit={handleStartRunSubmit}
        fieldErrors={startRunErrors}
      />
    </Group>
  );
}
