/**
 * Top bar of the Debugger page.
 *
 * Shape per the Feature body:
 *   • Breadcrumb: `Execution / <scenario-name>`
 *   • Sub-line: peer · subscriber · Mode chip · Status chip · Elapsed
 *   • Right-aligned per-state controls (paused: Run to end / Restart / Stop;
 *     running: Pause / Stop; completed: Export / View scenario / Re-run)
 *
 * Buttons are wired through the page-scoped store's imperative
 * actions; Stop and Restart open Mantine `Modal` confirms before
 * firing. Elapsed time tracks `state === 'running'` only — paused
 * freezes (per AC).
 *
 * Completed-state Export / Re-run handler is filled in by Task 8.
 */
import {
  Anchor,
  Badge,
  Breadcrumbs,
  Button,
  Group,
  Modal,
  Stack,
  Text,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import {
  IconArrowBack,
  IconDownload,
  IconExternalLink,
  IconPlayerPause,
  IconPlayerPlay,
  IconPlayerStop,
  IconRefresh,
  IconRepeat,
} from '@tabler/icons-react';
import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import { ApiError } from '../../api/errors';
import {
  useCreateExecution,
  type Execution,
  type ExecutionState,
} from '../../api/resources/executions';
import { notifyError } from '../../utils/notify';
import {
  STATE_COLOR,
  STATE_LABEL,
  isTerminal,
  modeLabel,
} from '../executions/runTableHelpers';

import {
  buildExportPayload,
  exportFilename,
  exportToString,
} from './exportRun';
import { useExecutionStore } from './useDebuggerStore';

interface DebuggerTopBarProps {
  execution: Execution;
  onBack: () => void;
}

export function DebuggerTopBar({ execution, onBack }: DebuggerTopBarProps) {
  const state = useExecutionStore((s) => s.state);
  const elapsedMs = useElapsedMs(execution, state);

  return (
    <Stack gap="xs" data-testid="debugger-topbar">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={2}>
          <Breadcrumbs separator="/" data-testid="debugger-breadcrumb">
            <Anchor component={Link} to="/executions" size="sm">
              Executions
            </Anchor>
            <Text size="sm" fw={500}>
              {execution.scenarioName}
            </Text>
          </Breadcrumbs>
          <Group gap="xs" wrap="wrap" data-testid="debugger-subline">
            {execution.peerName && (
              <Text size="xs" c="dimmed">
                Peer: {execution.peerName}
              </Text>
            )}
            {execution.subscriberMsisdn && (
              <Text size="xs" c="dimmed">
                Subscriber: {execution.subscriberMsisdn}
              </Text>
            )}
            <Badge variant="light" color="grape" size="sm">
              {modeLabel(execution.mode)}
            </Badge>
            <Badge
              variant="filled"
              color={STATE_COLOR[state]}
              size="sm"
              data-testid="debugger-status-chip"
            >
              {STATE_LABEL[state]}
            </Badge>
            <Text size="xs" c="dimmed" data-testid="debugger-elapsed">
              Elapsed {formatElapsed(elapsedMs)}
            </Text>
          </Group>
        </Stack>

        <Group gap="xs" wrap="nowrap">
          <Button
            variant="subtle"
            leftSection={<IconArrowBack size={14} />}
            onClick={onBack}
          >
            Back
          </Button>
          <ControlButtons execution={execution} />
        </Group>
      </Group>
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Control buttons + confirm modals
// ---------------------------------------------------------------------------

interface ControlButtonsProps {
  execution: Execution;
}

function ControlButtons({ execution }: ControlButtonsProps) {
  const navigate = useNavigate();
  const state = useExecutionStore((s) => s.state);
  const pause = useExecutionStore((s) => s.pause);
  const resume = useExecutionStore((s) => s.resume);
  const runToEnd = useExecutionStore((s) => s.runToEnd);
  const stopAction = useExecutionStore((s) => s.stop);

  const create = useCreateExecution();

  const [stopOpen, setStopOpen] = useState(false);
  const [restartOpen, setRestartOpen] = useState(false);
  const [actionPending, setActionPending] = useState(false);

  const wrap = async (label: string, fn: () => Promise<void>) => {
    setActionPending(true);
    try {
      await fn();
    } catch (err) {
      notifyError({
        title: `${label} failed`,
        message: (err as ApiError | Error).message,
      });
    } finally {
      setActionPending(false);
    }
  };

  const confirmStop = async () => {
    await wrap('Stop', async () => {
      await stopAction();
      notifications.show({
        color: 'orange',
        title: `Run #${execution.id} stopped`,
        message: execution.scenarioName,
      });
      setStopOpen(false);
    });
  };

  const confirmRestart = async () => {
    await wrap('Restart', async () => {
      const result = await create.mutateAsync({
        scenarioId: execution.scenarioId,
        mode: execution.mode,
        concurrency: 1,
        repeats: 1,
        ...(execution.peerId || execution.subscriberId
          ? {
              overrides: {
                ...(execution.peerId ? { peerId: execution.peerId } : {}),
                ...(execution.subscriberId
                  ? { subscriberIds: [execution.subscriberId] }
                  : {}),
              },
            }
          : {}),
      });
      const fresh = result.items[0];
      if (!fresh) {
        throw new Error('Restart returned no execution');
      }
      notifications.show({
        color: 'green',
        title: 'Restarted',
        message: `New run #${fresh.id}`,
      });
      setRestartOpen(false);
      navigate(`/executions/${encodeURIComponent(fresh.id)}`);
    });
  };

  return (
    <>
      {state === 'paused' && (
        <>
          <Button
            variant="default"
            leftSection={<IconPlayerPlay size={14} />}
            onClick={() => void wrap('Resume', () => runToEnd())}
            loading={actionPending}
            data-testid="debugger-run-to-end"
          >
            Run to end
          </Button>
          <Button
            variant="default"
            leftSection={<IconRefresh size={14} />}
            onClick={() => setRestartOpen(true)}
            disabled={actionPending}
            data-testid="debugger-restart"
          >
            Restart
          </Button>
          <Button
            color="red"
            leftSection={<IconPlayerStop size={14} />}
            onClick={() => setStopOpen(true)}
            disabled={actionPending}
            data-testid="debugger-stop"
          >
            Stop
          </Button>
        </>
      )}

      {state === 'running' && (
        <>
          <Button
            variant="default"
            leftSection={<IconPlayerPause size={14} />}
            onClick={() => void wrap('Pause', () => pause())}
            loading={actionPending}
            data-testid="debugger-pause"
          >
            Pause
          </Button>
          <Button
            color="red"
            leftSection={<IconPlayerStop size={14} />}
            onClick={() => setStopOpen(true)}
            disabled={actionPending}
            data-testid="debugger-stop"
          >
            Stop
          </Button>
        </>
      )}

      {state === 'pending' && (
        // Hand-roll a small "Resume" so a freshly-created run can be
        // started from the debugger without the user re-navigating.
        <Button
          variant="default"
          leftSection={<IconPlayerPlay size={14} />}
          onClick={() => void wrap('Resume', () => resume())}
          loading={actionPending}
          data-testid="debugger-resume"
        >
          Resume
        </Button>
      )}

      {isTerminal(state) && <TerminalControls execution={execution} />}

      <Modal
        opened={stopOpen}
        onClose={() => {
          if (!actionPending) setStopOpen(false);
        }}
        title="Stop execution"
        centered
        closeOnClickOutside={!actionPending}
        closeOnEscape={!actionPending}
      >
        <Stack gap="md">
          <Text size="sm">
            Stop run <strong>#{execution.id}</strong> for{' '}
            <strong>{execution.scenarioName}</strong>? The engine
            transitions the run to <code>aborted</code>; in-flight CCRs
            are terminated best-effort. This cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button
              variant="subtle"
              onClick={() => setStopOpen(false)}
              disabled={actionPending}
            >
              Cancel
            </Button>
            <Button
              color="red"
              onClick={() => void confirmStop()}
              loading={actionPending}
              data-testid="debugger-stop-confirm"
            >
              Stop
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal
        opened={restartOpen}
        onClose={() => {
          if (!actionPending) setRestartOpen(false);
        }}
        title="Restart execution"
        centered
        closeOnClickOutside={!actionPending}
        closeOnEscape={!actionPending}
      >
        <Stack gap="md">
          <Text size="sm">
            Restart <strong>#{execution.id}</strong>? Progress on the
            current run will be lost — a fresh execution starts from
            step 1. The historical run remains in the list.
          </Text>
          <Group justify="flex-end">
            <Button
              variant="subtle"
              onClick={() => setRestartOpen(false)}
              disabled={actionPending}
            >
              Cancel
            </Button>
            <Button
              onClick={() => void confirmRestart()}
              loading={actionPending}
              data-testid="debugger-restart-confirm"
            >
              Restart
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
}

/**
 * Terminal-state controls — Export / View scenario / Re-run.
 */
interface TerminalControlsProps {
  execution: Execution;
}

function TerminalControls({ execution }: TerminalControlsProps) {
  const navigate = useNavigate();
  const create = useCreateExecution();

  const handleExport = () => {
    const payload = buildExportPayload(execution);
    const json = exportToString(payload);
    const blob = new Blob([json], { type: 'application/json' });
    const href = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = href;
    a.download = exportFilename(execution.id);
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(href);
  };

  const handleRerun = async () => {
    try {
      const result = await create.mutateAsync({
        scenarioId: execution.scenarioId,
        mode: execution.mode,
        concurrency: 1,
        repeats: 1,
        ...(execution.peerId || execution.subscriberId
          ? {
              overrides: {
                ...(execution.peerId ? { peerId: execution.peerId } : {}),
                ...(execution.subscriberId
                  ? { subscriberIds: [execution.subscriberId] }
                  : {}),
              },
            }
          : {}),
      });
      const fresh = result.items[0];
      if (fresh) {
        notifications.show({
          color: 'green',
          title: 'Re-run started',
          message: `New run #${fresh.id}`,
        });
        navigate(`/executions/${encodeURIComponent(fresh.id)}`);
      }
    } catch (err) {
      notifyError({
        title: 'Re-run failed',
        message: (err as ApiError | Error).message,
      });
    }
  };

  return (
    <>
      <Button
        variant="default"
        leftSection={<IconDownload size={14} />}
        onClick={handleExport}
        data-testid="debugger-export"
      >
        Export
      </Button>
      <Button
        variant="default"
        leftSection={<IconExternalLink size={14} />}
        onClick={() =>
          navigate(`/scenarios/${encodeURIComponent(execution.scenarioId)}`, {
            state: { returnTo: `/executions/${encodeURIComponent(execution.id)}` },
          })
        }
        data-testid="debugger-view-scenario"
      >
        View scenario
      </Button>
      <Button
        leftSection={<IconRepeat size={14} />}
        loading={create.isPending}
        onClick={() => void handleRerun()}
        data-testid="debugger-rerun"
      >
        Re-run
      </Button>
    </>
  );
}

// ---------------------------------------------------------------------------
// Elapsed clock
// ---------------------------------------------------------------------------

/**
 * Elapsed since the run's effective start. Live-ticks only while the
 * run is `running`; paused / terminal states freeze the clock per AC.
 * Reads the timeline from the store (which the SSE driver
 * stamps session-locally on `execution.started` and on terminal
 * events) and falls back to the snapshot's timestamps when the store
 * has not yet seen an SSE event — so a freshly-loaded already-completed
 * run still shows a sensible elapsed.
 */
function useElapsedMs(execution: Execution, state: ExecutionState): number {
  const storeStartedAt = useExecutionStore((s) => s.startedAt);
  const storeFinishedAt = useExecutionStore((s) => s.finishedAt);
  const ticking = state === 'running';
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!ticking) return undefined;
    const t = setInterval(() => setNow(Date.now()), 1_000);
    return () => clearInterval(t);
  }, [ticking]);

  const startedIso = storeStartedAt ?? execution.startedAt;
  const finishedIso = storeFinishedAt ?? execution.finishedAt ?? null;
  // Frozen-clock reference: the run's recorded finishedAt when
  // terminal, otherwise `now` (which stops advancing while paused
  // because the interval above only ticks while `running`).
  const referenceMs = !ticking && finishedIso ? Date.parse(finishedIso) : now;
  const startedAtMs = Date.parse(startedIso);
  if (Number.isNaN(startedAtMs)) return 0;
  return Math.max(0, referenceMs - startedAtMs);
}

function formatElapsed(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}
