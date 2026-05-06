/**
 * Center pane (viewer mode) — shows the outgoing CCR for a completed step.
 *
 * When `historicalIndex` is set and the step has `requestText`, renders
 * the wire-format text verbatim. Otherwise falls back to the CCR preview
 * tree from the store (template-resolved, best-effort).
 *
 * Header carries Prev / Next buttons to navigate execution history
 * without returning to the history pane.
 */
import {
  Box,
  Button,
  Code,
  Divider,
  Group,
  ScrollArea,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconChevronLeft, IconChevronRight } from '@tabler/icons-react';

import type { PreviewAvpNode } from './ccrPreview';
import { useExecutionStore } from './useDebuggerStore';

export function RequestPane() {
  const steps = useExecutionStore((s) => s.steps);
  const historicalIndex = useExecutionStore((s) => s.historicalIndex);
  const previewTree = useExecutionStore((s) => s.edit.previewTree);
  const viewHistorical = useExecutionStore((s) => s.viewHistorical);

  const step = historicalIndex !== null ? steps[historicalIndex] : undefined;
  const hasPrev = historicalIndex !== null && historicalIndex > 0;
  const hasNext =
    historicalIndex !== null && historicalIndex < steps.length - 1;

  return (
    <Stack gap="xs" h="100%" data-testid="debugger-request-pane">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={2}>
          <Title order={5}>Request</Title>
          {step && (
            <Text size="xs" c="dimmed">
              Step {step.n}: {step.label ?? step.kind}
            </Text>
          )}
        </Stack>
        {historicalIndex !== null && (
          <Group gap={4} wrap="nowrap">
            <Button
              variant="subtle"
              size="xs"
              px={6}
              disabled={!hasPrev}
              onClick={() =>
                hasPrev && viewHistorical((historicalIndex as number) - 1)
              }
              data-testid="debugger-request-prev"
            >
              <IconChevronLeft size={14} />
            </Button>
            <Button
              variant="subtle"
              size="xs"
              px={6}
              disabled={!hasNext}
              onClick={() =>
                hasNext && viewHistorical((historicalIndex as number) + 1)
              }
              data-testid="debugger-request-next"
            >
              <IconChevronRight size={14} />
            </Button>
          </Group>
        )}
      </Group>

      <Divider />

      <ScrollArea style={{ flex: 1 }} data-testid="debugger-request-content">
        {step?.requestText ? (
          <Code
            block
            style={{ fontFamily: 'monospace', whiteSpace: 'pre', fontSize: 11 }}
          >
            {step.requestText}
          </Code>
        ) : previewTree && previewTree.length > 0 ? (
          <Stack gap={2}>
            {previewTree.map((n, i) => (
              <PreviewNode key={`${n.code}-${i}`} node={n} depth={0} />
            ))}
          </Stack>
        ) : (
          <Text size="xs" c="dimmed">
            {historicalIndex === null
              ? 'Select a step in the history to inspect its request.'
              : 'No request data available for this step.'}
          </Text>
        )}
      </ScrollArea>
    </Stack>
  );
}

interface PreviewNodeProps {
  node: PreviewAvpNode;
  depth: number;
}

function PreviewNode({ node, depth }: PreviewNodeProps) {
  return (
    <Box>
      <Group
        gap="xs"
        wrap="nowrap"
        style={{ marginLeft: depth * 16, fontFamily: 'monospace' }}
      >
        <Text size="xs">
          {node.name}{' '}
          <Text component="span" c="dimmed">
            ({node.code})
          </Text>
        </Text>
        {node.value !== undefined && (
          <Code style={{ fontSize: 11 }}>{node.value}</Code>
        )}
      </Group>
      {node.children?.map((c, i) => (
        <PreviewNode key={`${c.code}-${i}`} node={c} depth={depth + 1} />
      ))}
    </Box>
  );
}
