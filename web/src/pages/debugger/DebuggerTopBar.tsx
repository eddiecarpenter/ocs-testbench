/**
 * Top bar of the Debugger page.
 *
 * Shape per the Feature body:
 *   • Breadcrumb: `Execution / <scenario-name>`
 *   • Sub-line: peer · subscriber · Mode chip · Status chip · Elapsed
 *   • Right-aligned per-state controls (paused: Run to end / Restart / Stop;
 *     running: Pause / Stop; completed: Export / View scenario / Re-run)
 *
 * Task 1 lands the breadcrumb, sub-line, and disabled control buttons.
 * Task 7 wires the buttons to imperative actions; Task 8 fills the
 * Completed-state Export / View scenario / Re-run.
 *
 * Status + Elapsed read from the page-scoped `executionStore` so they
 * react to SSE-driven transitions live (Task 2 onwards).
 */
import {
  Anchor,
  Badge,
  Breadcrumbs,
  Button,
  Group,
  Stack,
  Text,
} from '@mantine/core';
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
import { Link } from 'react-router';

import type {
  Execution,
  ExecutionState,
} from '../../api/resources/executions';
import {
  STATE_COLOR,
  STATE_LABEL,
  isTerminal,
  modeLabel,
} from '../executions/runTableHelpers';

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
          <ControlButtons />
        </Group>
      </Group>
    </Stack>
  );
}

/**
 * Render the right-side control buttons per state.
 *
 * Buttons are intentionally disabled in Task 1 — Task 7 wires the
 * `onClick` handlers and the per-state hidden / disabled rules.
 */
function ControlButtons() {
  const state = useExecutionStore((s) => s.state);

  if (state === 'paused') {
    return (
      <>
        <Button
          variant="default"
          leftSection={<IconPlayerPlay size={14} />}
          disabled
          data-testid="debugger-run-to-end"
        >
          Run to end
        </Button>
        <Button
          variant="default"
          leftSection={<IconRefresh size={14} />}
          disabled
          data-testid="debugger-restart"
        >
          Restart
        </Button>
        <Button
          color="red"
          leftSection={<IconPlayerStop size={14} />}
          disabled
          data-testid="debugger-stop"
        >
          Stop
        </Button>
      </>
    );
  }

  if (state === 'running') {
    return (
      <>
        <Button
          variant="default"
          leftSection={<IconPlayerPause size={14} />}
          disabled
          data-testid="debugger-pause"
        >
          Pause
        </Button>
        <Button
          color="red"
          leftSection={<IconPlayerStop size={14} />}
          disabled
          data-testid="debugger-stop"
        >
          Stop
        </Button>
      </>
    );
  }

  if (isTerminal(state)) {
    return (
      <>
        <Button
          variant="default"
          leftSection={<IconDownload size={14} />}
          disabled
          data-testid="debugger-export"
        >
          Export
        </Button>
        <Button
          variant="default"
          leftSection={<IconExternalLink size={14} />}
          disabled
          data-testid="debugger-view-scenario"
        >
          View scenario
        </Button>
        <Button
          leftSection={<IconRepeat size={14} />}
          disabled
          data-testid="debugger-rerun"
        >
          Re-run
        </Button>
      </>
    );
  }

  // pending or unknown — no controls.
  return null;
}

/**
 * Elapsed since `startedAt`. Live-ticks while the run is in flight and
 * freezes once terminal. Recomputes from `startedAt` (no internal
 * accumulator) so Pause / Resume scheduler drift can't corrupt it.
 */
function useElapsedMs(execution: Execution, state: ExecutionState): number {
  const isLive = state === 'running' || state === 'paused' || state === 'pending';
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!isLive) return undefined;
    const t = setInterval(() => setNow(Date.now()), 1_000);
    return () => clearInterval(t);
  }, [isLive]);

  const finishedAtMs =
    !isLive && execution.finishedAt ? Date.parse(execution.finishedAt) : now;
  const startedAtMs = Date.parse(execution.startedAt);
  if (Number.isNaN(startedAtMs)) return 0;
  return Math.max(0, finishedAtMs - startedAtMs);
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
