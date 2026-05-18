/**
 * Right pane (viewer mode) — CCA response for a completed step.
 *
 * Renders:
 *   - result-code chip (palette per resultCodeColor) + RTT + sizes
 *   - assertion list with ✓ / ✗ markers
 *   - extractions list
 *   - raw CCA AVP tree inline (wire-format text or pretty-printed JSON)
 */
import {
  Badge,
  Code,
  Divider,
  Group,
  ScrollArea,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconCheck, IconX } from '@tabler/icons-react';

import type { StepRecord } from '../../api/resources/executions';

import {
  approximateSize,
  extractExtractions,
  extractResultCode,
  formatRtt,
  formatSize,
  pickDisplayedStep,
  resultCodeColor,
  resultCodeLabel,
} from './lastResponseLogic';
import { useExecutionStore } from './useDebuggerStore';

export function LastResponsePane() {
  const steps = useExecutionStore((s) => s.steps);
  const cursor = useExecutionStore((s) => s.cursor);
  const historicalIndex = useExecutionStore((s) => s.historicalIndex);
  const viewHistorical = useExecutionStore((s) => s.viewHistorical);
  const runState = useExecutionStore((s) => s.state);

  const record = pickDisplayedStep(steps, cursor, historicalIndex);
  const isHistorical = historicalIndex !== null;
  const showBackToLive = isHistorical && runState === 'paused';

  if (!record) {
    return (
      <Stack gap="xs" h="100%" data-testid="debugger-last-response-pane">
        <Title order={5}>Response</Title>
        <Text size="xs" c="dimmed">
          No responses yet.
        </Text>
      </Stack>
    );
  }

  const response = record.response as Record<string, unknown> | undefined;
  const resultCode = extractResultCode(response);
  const extractions = extractExtractions(response);

  return (
    <Stack gap="xs" h="100%" data-testid="debugger-last-response-pane">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={2}>
          <Title order={5}>Response</Title>
          <Text size="xs" c="dimmed">
            Step {record.n}: {record.label ?? record.kind}
            {isHistorical ? ' (historical)' : ''}
          </Text>
        </Stack>
        {showBackToLive && (
          <Text
            size="xs"
            c="blue"
            style={{ cursor: 'pointer', flexShrink: 0 }}
            onClick={() => viewHistorical(null)}
            data-testid="debugger-historical-back"
          >
            ← Back to live
          </Text>
        )}
      </Group>

      <Group gap="xs" wrap="wrap">
        <Badge
          variant="filled"
          color={resultCodeColor(resultCode)}
          size="md"
          data-testid="debugger-result-chip"
        >
          {resultCodeLabel(resultCode)}
        </Badge>
        <Text size="xs" c="dimmed">
          RTT {formatRtt(record.durationMs)}
        </Text>
        <Text size="xs" c="dimmed">
          Req {formatSize(record.requestSize ?? approximateSize(record.request as Record<string, unknown>))}
        </Text>
        <Text size="xs" c="dimmed">
          Res {formatSize(approximateSize(response))}
        </Text>
      </Group>

      {record.errorDetail && (
        <Text size="xs" c="red" data-testid="debugger-error-detail">
          {record.errorDetail}
        </Text>
      )}

      <Divider label="Assertions" labelPosition="left" />
      <AssertionList assertions={record.assertionResults ?? []} />

      <Divider label="Extracted variables" labelPosition="left" />
      <ExtractionsList extractions={extractions} />

      <Divider label="CCA AVP tree" labelPosition="left" />
      <ScrollArea style={{ flex: 1 }} data-testid="debugger-cca-avp-tree">
        <Code
          block
          style={{ fontFamily: 'monospace', whiteSpace: 'pre', fontSize: 11 }}
        >
          {record.responseText ?? prettyJson(response)}
        </Code>
      </ScrollArea>
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

interface AssertionListProps {
  assertions: NonNullable<StepRecord['assertionResults']>;
}

function AssertionList({ assertions }: AssertionListProps) {
  if (assertions.length === 0) {
    return (
      <Text size="xs" c="dimmed">
        No assertions on this step.
      </Text>
    );
  }
  return (
    <Stack gap={2} data-testid="debugger-assertions">
      {assertions.map((a, i) => (
        <Group
          key={i}
          gap="xs"
          wrap="nowrap"
          data-testid={`debugger-assertion-${a.passed ? 'pass' : 'fail'}-${i}`}
        >
          {a.passed ? (
            <IconCheck size={14} color="var(--mantine-color-teal-7)" />
          ) : (
            <IconX size={14} color="var(--mantine-color-red-7)" />
          )}
          <Text
            size="xs"
            c={a.passed ? undefined : 'red'}
            style={{ fontFamily: 'monospace' }}
          >
            {a.expression}
            {a.message ? ` — ${a.message}` : ''}
          </Text>
        </Group>
      ))}
    </Stack>
  );
}

interface ExtractionsListProps {
  extractions: Record<string, unknown>;
}

function ExtractionsList({ extractions }: ExtractionsListProps) {
  const entries = Object.entries(extractions);
  if (entries.length === 0) {
    return (
      <Text size="xs" c="dimmed">
        No extractions on this step.
      </Text>
    );
  }
  return (
    <Stack gap={2} data-testid="debugger-extractions">
      {entries.map(([name, value]) => {
        const failed = value === null || value === undefined;
        return (
          <Group
            key={name}
            gap="xs"
            wrap="nowrap"
            data-testid={`debugger-extraction-${name}`}
          >
            <Text size="xs" style={{ fontFamily: 'monospace' }}>
              {name}
            </Text>
            {failed ? (
              <Text
                size="xs"
                c="dimmed"
                fs="italic"
                data-testid="debugger-extraction-not-set"
              >
                (not set)
              </Text>
            ) : (
              <Code style={{ fontSize: 11 }}>{stringify(value)}</Code>
            )}
          </Group>
        );
      })}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

function prettyJson(payload: unknown): string {
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return String(payload);
  }
}

function stringify(v: unknown): string {
  if (v === null) return 'null';
  if (v === undefined) return '(not set)';
  if (typeof v === 'string') return v;
  return String(v);
}
