/**
 * Middle pane — Step Editor + Services panel + CCR preview tree.
 *
 * Active in `paused` state only. In `running` (or any other state)
 * the pane goes read-only — controls disabled, fields uneditable.
 *
 * Layout (top to bottom):
 *   - Next-step header (step name · type chip · "Revert all")
 *   - Context-variables panel (read-only in MVP — system + user
 *     scopes; multi-MSCC variables carry the `RG<rg>_` prefix per
 *     ARCHITECTURE.md §5)
 *   - Services panel ("N of M selected", per-MSCC checkboxes)
 *   - CCR preview tree (resolver from Task 3)
 *   - Regenerate button
 *   - Footer: Skip step / Send CCR
 *
 * Imperative actions go through the page-scoped store; Task 7 wires
 * the actual POSTs and the SSE-driven response loop.
 */
import {
  Badge,
  Button,
  Checkbox,
  Code,
  Divider,
  Group,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from '@mantine/core';
import {
  IconPlayerPlay,
  IconPlayerSkipForward,
  IconRefresh,
  IconRotate,
} from '@tabler/icons-react';
import { useEffect, useMemo } from 'react';

import { useExecution } from '../../api/resources/executions';
import { useScenario } from '../../api/resources/scenarios';

import type { PreviewAvpNode } from './ccrPreview';
import {
  buildStepHeader,
  defaultServicesForStep,
  flattenContext,
  resolvePreview,
} from './stepEditorLogic';
import { useExecutionStore } from './useDebuggerStore';

export function StepEditorPane() {
  const executionId = useExecutionStore((s) => s.executionId);
  const cursor = useExecutionStore((s) => s.cursor);
  const runState = useExecutionStore((s) => s.state);
  const context = useExecutionStore((s) => s.context);
  const servicesEnabled = useExecutionStore((s) => s.edit.servicesEnabled);
  const previewTree = useExecutionStore((s) => s.edit.previewTree);
  const dirty = useExecutionStore((s) => s.edit.dirty);

  const toggleService = useExecutionStore((s) => s.toggleService);
  const regenerate = useExecutionStore((s) => s.regenerate);
  const setPreviewTree = useExecutionStore((s) => s.setPreviewTree);
  const revertEdit = useExecutionStore((s) => s.revertEdit);
  const sendCcr = useExecutionStore((s) => s.sendCcr);
  const skip = useExecutionStore((s) => s.skip);

  const executionQuery = useExecution(executionId);
  const scenarioQuery = useScenario(executionQuery.data?.scenarioId);

  const scenario = scenarioQuery.data;
  const flatContext = useMemo(() => flattenContext(context), [context]);

  // Compute the resolved tree on every input change and push it into
  // the store so other consumers (and the unit tests reading the store
  // directly) see the same tree.
  const resolved = useMemo<PreviewAvpNode[] | null>(() => {
    if (!scenario) return null;
    return resolvePreview(scenario, cursor, flatContext, servicesEnabled);
  }, [scenario, cursor, flatContext, servicesEnabled]);

  useEffect(() => {
    setPreviewTree(resolved);
  }, [resolved, setPreviewTree]);

  // When the cursor advances OR the scenario loads, seed
  // `edit.servicesEnabled` from the step's defaults — but only when
  // the user hasn't dirtied the edit state yet for this cursor.
  useEffect(() => {
    if (!scenario) return;
    if (dirty) return;
    if (servicesEnabled.size > 0) return;
    revertEdit(defaultServicesForStep(scenario, cursor));
    // We intentionally exclude `dirty` from the deps — a transition to
    // dirty must NOT trigger a reset.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scenario, cursor, revertEdit]);

  if (!scenario || !executionQuery.data) {
    return (
      <Stack gap="xs" data-testid="debugger-step-editor-pane">
        <Title order={5}>Step editor</Title>
        <Skeleton height={28} />
        <Skeleton height={120} />
        <Skeleton height={80} />
      </Stack>
    );
  }

  const interactive = runState === 'paused';
  const header = buildStepHeader(scenario, cursor);
  const servicesAll = scenario.services.map((s) => s.id);
  const servicesSelectedCount = servicesEnabled.size;
  const servicesTotal = servicesAll.length;
  const isMultiMscc = scenario.serviceModel === 'multi-mscc';

  return (
    <Stack gap="xs" data-testid="debugger-step-editor-pane">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        <Stack gap={2}>
          <Title order={5}>Step editor</Title>
          {header && (
            <Group gap="xs" wrap="nowrap">
              <Text size="sm" fw={500}>
                {header.title}
              </Text>
              <Badge size="sm" color={header.kindColor} variant="light">
                {header.kindLabel}
              </Badge>
            </Group>
          )}
        </Stack>
        <Tooltip
          label="Reset services & overrides for this step"
          disabled={!interactive}
        >
          <Button
            variant="subtle"
            size="xs"
            leftSection={<IconRotate size={14} />}
            disabled={!interactive}
            onClick={() => revertEdit(defaultServicesForStep(scenario, cursor))}
            data-testid="debugger-revert-all"
          >
            Revert all
          </Button>
        </Tooltip>
      </Group>

      <Divider label="Context variables" labelPosition="left" />
      <ContextVariablesPanel context={flatContext} />

      {isMultiMscc && (
        <>
          <Divider
            label={`Services — ${servicesSelectedCount} of ${servicesTotal} selected`}
            labelPosition="left"
          />
          <Stack gap={4} data-testid="debugger-services-panel">
            {scenario.services.map((svc) => (
              <Checkbox
                key={svc.id}
                label={svcLabel(svc)}
                checked={servicesEnabled.has(svc.id)}
                disabled={!interactive}
                onChange={() => toggleService(svc.id)}
                data-testid={`debugger-service-${svc.id}`}
              />
            ))}
          </Stack>
        </>
      )}

      <Divider
        label={dirty ? 'CCR preview (changed — regenerate)' : 'CCR preview'}
        labelPosition="left"
      />
      <CcrPreview tree={previewTree} />

      <Group justify="flex-end" gap="xs">
        <Button
          variant="subtle"
          size="xs"
          leftSection={<IconRefresh size={14} />}
          onClick={() => regenerate()}
          disabled={!interactive}
          data-testid="debugger-regenerate"
        >
          Regenerate
        </Button>
      </Group>

      <Divider />
      <Group justify="space-between" wrap="nowrap">
        <Button
          variant="default"
          leftSection={<IconPlayerSkipForward size={14} />}
          disabled={!interactive}
          onClick={() => void skip()}
          data-testid="debugger-skip"
        >
          Skip step
        </Button>
        <Button
          leftSection={<IconPlayerPlay size={14} />}
          disabled={!interactive}
          onClick={() => void sendCcr()}
          data-testid="debugger-send-ccr"
        >
          Send CCR
        </Button>
      </Group>

      {!interactive && runState === 'running' && (
        <Text size="xs" c="dimmed" ta="center">
          Sending… (live run; pause to edit)
        </Text>
      )}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

interface ContextVariablesPanelProps {
  context: Record<string, unknown>;
}

function ContextVariablesPanel({ context }: ContextVariablesPanelProps) {
  const entries = Object.entries(context);
  if (entries.length === 0) {
    return (
      <Text size="xs" c="dimmed">
        No variables resolved yet.
      </Text>
    );
  }
  return (
    <Tooltip label="Editing context mid-run lands in a follow-up Feature">
      <Table withTableBorder withColumnBorders fz="xs" data-testid="debugger-context-vars">
        <Table.Tbody>
          {entries.map(([k, v]) => (
            <Table.Tr key={k}>
              <Table.Td style={{ fontFamily: 'monospace' }}>{k}</Table.Td>
              <Table.Td>
                <Code>{stringify(v)}</Code>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Tooltip>
  );
}

interface CcrPreviewProps {
  tree: PreviewAvpNode[] | null;
}

function CcrPreview({ tree }: CcrPreviewProps) {
  if (!tree || tree.length === 0) {
    return (
      <Text size="xs" c="dimmed">
        No preview available.
      </Text>
    );
  }
  return (
    <Stack gap={2} data-testid="debugger-ccr-preview">
      {tree.map((n, i) => (
        <PreviewNode key={`${n.code}-${i}`} node={n} depth={0} />
      ))}
    </Stack>
  );
}

interface PreviewNodeProps {
  node: PreviewAvpNode;
  depth: number;
}

function PreviewNode({ node, depth }: PreviewNodeProps) {
  return (
    <Stack gap={2}>
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
          <Code data-testid={`avp-${node.name}`}>{node.value}</Code>
        )}
      </Group>
      {node.children?.map((c, i) => (
        <PreviewNode key={`${c.code}-${i}`} node={c} depth={depth + 1} />
      ))}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

function svcLabel(svc: { id: string; ratingGroup?: string }): string {
  return svc.ratingGroup ? `${svc.id} (RG ${svc.ratingGroup})` : svc.id;
}

function stringify(v: unknown): string {
  if (v === null) return 'null';
  if (v === undefined) return 'undefined';
  if (typeof v === 'string') return v;
  return String(v);
}
