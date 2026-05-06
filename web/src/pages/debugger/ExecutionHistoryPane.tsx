/**
 * Left pane — flat execution history.
 *
 * Shows every step that was actually executed (including repeated steps
 * from loops) in the order they ran. Unlike the old ProgressPane, which
 * mirrored the scenario template, this reflects real execution order:
 * five UPDATE repetitions produce five rows.
 *
 * Each row:
 *   - step number (N)
 *   - status icon
 *   - label + kind chip
 *   - result code badge (when available)
 *   - RTT
 *
 * Clicking a completed row fires `viewHistorical(i)`.
 */
import {
  Badge,
  Box,
  Group,
  Progress,
  ScrollArea,
  Stack,
  Text,
  Title,
  UnstyledButton,
} from '@mantine/core';
import {
  IconCheck,
  IconCircleDashed,
  IconClock,
  IconLoader2,
  IconX,
} from '@tabler/icons-react';
import { useEffect, useRef, useState } from 'react';

import type { StepRecord } from '../../api/resources/executions';

import {
  extractResultCode,
  resultCodeColor,
  resultCodeLabel,
} from './lastResponseLogic';
import {
  formatDuration,
  isStepCompleted,
  kindColor,
  kindLabel,
} from './progressPaneLogic';
import { useExecutionStore } from './useDebuggerStore';

export function ExecutionHistoryPane() {
  const steps = useExecutionStore((s) => s.steps);
  const cursor = useExecutionStore((s) => s.cursor);
  const historicalIndex = useExecutionStore((s) => s.historicalIndex);
  const viewHistorical = useExecutionStore((s) => s.viewHistorical);
  const sleepCountdown = useExecutionStore((s) => s.sleepCountdown);
  const clearSleepCountdown = useExecutionStore((s) => s.clearSleepCountdown);

  const successCount = steps.filter((s) => s.state === 'success').length;
  const failureCount = steps.filter(
    (s) => s.state === 'failure' || s.state === 'error',
  ).length;

  return (
    <Stack gap="xs" h="100%" data-testid="debugger-history-pane">
      <Box>
        <Title order={5}>Execution History</Title>
        <Text size="xs" c="dimmed">
          {steps.length === 0
            ? 'No steps executed yet'
            : [
                `${steps.length} step${steps.length !== 1 ? 's' : ''}`,
                successCount > 0 ? `${successCount} success` : null,
                failureCount > 0 ? `${failureCount} failed` : null,
              ]
                .filter(Boolean)
                .join(' · ')}
        </Text>
      </Box>

      <ScrollArea style={{ flex: 1 }}>
        <Stack gap={2}>
          {steps.length === 0 && (
            <Text size="xs" c="dimmed" mt="xs">
              Waiting for first step…
            </Text>
          )}
          {steps.map((step, i) => (
            <HistoryRow
              key={i}
              step={step}
              index={i}
              isSelected={
                historicalIndex !== null ? historicalIndex === i : cursor === i
              }
              onClick={() => {
                if (isStepCompleted(step)) viewHistorical(i);
              }}
            />
          ))}
          {sleepCountdown && (
            <SleepCountdown
              totalSec={sleepCountdown.totalSec}
              endsAt={sleepCountdown.endsAt}
              onExpired={clearSleepCountdown}
            />
          )}
        </Stack>
      </ScrollArea>
    </Stack>
  );
}

interface HistoryRowProps {
  step: StepRecord;
  index: number;
  isSelected: boolean;
  onClick: () => void;
}

function HistoryRow({ step, index, isSelected, onClick }: HistoryRowProps) {
  const completed = isStepCompleted(step);
  const chipLabel =
    step.kind === 'request' && step.requestType
      ? step.requestType
      : kindLabel(step.kind);
  const resultCode = extractResultCode(
    step.response as Record<string, unknown> | undefined,
  );

  const background = isSelected
    ? 'var(--mantine-color-blue-light)'
    : 'transparent';

  return (
    <UnstyledButton
      onClick={onClick}
      disabled={!completed}
      data-testid={`debugger-history-row-${index}`}
      style={{
        padding: '6px 8px',
        borderRadius: 4,
        background,
        cursor: completed ? 'pointer' : 'default',
        opacity: step.state === 'pending' ? 0.5 : 1,
      }}
    >
      <Group gap="xs" wrap="nowrap" justify="space-between">
        <Group gap="xs" wrap="nowrap" miw={0}>
          <HistoryIcon state={step.state} />
          <Text size="sm" truncate>
            {index + 1}. {step.label ?? `Step ${index + 1}`}
          </Text>
          <Badge size="xs" variant="light" color={kindColor(step.kind)}>
            {chipLabel}
          </Badge>
          {resultCode !== undefined && (
            <Badge
              size="xs"
              variant="dot"
              color={resultCodeColor(resultCode)}
            >
              {resultCodeLabel(resultCode)}
            </Badge>
          )}
        </Group>
        {completed && step.durationMs !== undefined && step.durationMs > 0 && (
          <Text size="xs" c="dimmed" style={{ flexShrink: 0 }}>
            {formatDuration(step.durationMs)}
          </Text>
        )}
      </Group>
    </UnstyledButton>
  );
}

// ---------------------------------------------------------------------------
// Sleep countdown
// ---------------------------------------------------------------------------

interface SleepCountdownProps {
  totalSec: number;
  endsAt: number; // Date.now() + totalSec * 1000, set when the delay started
  onExpired: () => void;
}

function SleepCountdown({ totalSec, endsAt, onExpired }: SleepCountdownProps) {
  const [remainingSec, setRemainingSec] = useState(() =>
    Math.max(0, (endsAt - Date.now()) / 1000),
  );
  const rafRef = useRef<number | null>(null);
  const expiredTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onExpiredRef = useRef(onExpired);
  onExpiredRef.current = onExpired;

  useEffect(() => {
    const tick = () => {
      const left = Math.max(0, (endsAt - Date.now()) / 1000);
      setRemainingSec(left);
      if (left > 0) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        // Countdown reached zero — clear after a brief pause so user sees 00:00.
        // Store the ID so the cleanup can cancel it if the component unmounts
        // before it fires (e.g. step.sending arrives and wipes sleepCountdown),
        // preventing a stale callback from clearing the *next* sleep's countdown.
        expiredTimerRef.current = setTimeout(() => onExpiredRef.current(), 1500);
      }
    };
    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      if (expiredTimerRef.current !== null) {
        clearTimeout(expiredTimerRef.current);
        expiredTimerRef.current = null;
      }
    };
  }, [endsAt]);

  const whole = Math.ceil(remainingSec);
  const mins = Math.floor(whole / 60);
  const secs = whole % 60;
  const display = `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  // Bar depletes: starts at 100% (full) and drains to 0% (empty)
  const pct = totalSec > 0 ? (remainingSec / totalSec) * 100 : 0;

  return (
    <Box
      px="sm"
      py={6}
      mt={4}
      style={{
        borderRadius: 6,
        background: 'var(--mantine-color-blue-light)',
        border: '1px solid var(--mantine-color-blue-light-hover)',
      }}
      data-testid="debugger-sleep-countdown"
    >
      <Group gap={8} justify="space-between" align="center" wrap="nowrap" mb={4}>
        <Group gap={4} wrap="nowrap">
          <IconClock size={13} color="var(--mantine-color-blue-6)" />
          <Text size="xs" c="dimmed" fw={500}>Next step in</Text>
        </Group>
        <Text
          fw={800}
          c="blue"
          style={{
            fontSize: 20,
            lineHeight: 1,
            fontVariantNumeric: 'tabular-nums',
            letterSpacing: '1px',
            fontFamily: 'monospace',
          }}
        >
          {display}
        </Text>
      </Group>
      <Progress value={pct} size={8} color="blue" styles={{ section: { transition: 'none' } }} />
    </Box>
  );
}

function HistoryIcon({ state }: { state: string }) {
  switch (state) {
    case 'success':
      return <IconCheck size={14} color="var(--mantine-color-teal-7)" />;
    case 'failure':
    case 'error':
      return <IconX size={14} color="var(--mantine-color-red-7)" />;
    case 'running':
      return <IconLoader2 size={14} color="var(--mantine-color-blue-6)" />;
    case 'skipped':
    case 'pending':
    default:
      return (
        <IconCircleDashed size={14} color="var(--mantine-color-gray-5)" />
      );
  }
}
