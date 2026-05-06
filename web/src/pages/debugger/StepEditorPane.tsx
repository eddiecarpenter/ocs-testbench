/**
 * Center pane (debugger mode) — two-column step editor.
 *
 * Active only when the execution is `paused` and no historical step is
 * selected. In any other state the pane is not rendered.
 *
 * Layout (left | right):
 *   Left (~380px):
 *     - Step header (name · type chip · "Revert all")
 *     - System variables (read-only, locked)
 *     - User / scenario variables (editable — overrides sent with POST /step)
 *     - Extracted variables (read-only, derived by prior steps)
 *     - Services panel (multi-MSCC only)
 *   Right (flex):
 *     - CCR preview tree (live-resolved with current overrides)
 *     - Regenerate button
 *   Footer (below both columns):
 *     - Skip step | Send CCR
 */
import {
  Badge,
  Box,
  Button,
  Checkbox,
  Code,
  Divider,
  Flex,
  Group,
  ScrollArea,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core';
import {
  IconLock,
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
  const overrides = useExecutionStore((s) => s.edit.overrides);

  const toggleService = useExecutionStore((s) => s.toggleService);
  const regenerate = useExecutionStore((s) => s.regenerate);
  const setPreviewTree = useExecutionStore((s) => s.setPreviewTree);
  const revertEdit = useExecutionStore((s) => s.revertEdit);
  const setOverride = useExecutionStore((s) => s.setOverride);
  const sendCcr = useExecutionStore((s) => s.sendCcr);
  const skip = useExecutionStore((s) => s.skip);

  const executionQuery = useExecution(executionId);
  const scenarioQuery = useScenario(executionQuery.data?.scenarioId);
  const scenario = scenarioQuery.data;

  // Merge overrides on top of context for live CCR preview resolution.
  const flatContext = useMemo(
    () => ({ ...flattenContext(context), ...overrides }),
    [context, overrides],
  );

  const resolved = useMemo<PreviewAvpNode[] | null>(() => {
    if (!scenario) return null;
    return resolvePreview(scenario, cursor, flatContext, servicesEnabled);
  }, [scenario, cursor, flatContext, servicesEnabled]);

  useEffect(() => {
    setPreviewTree(resolved);
  }, [resolved, setPreviewTree]);

  // Seed servicesEnabled from the step defaults when the cursor advances.
  useEffect(() => {
    if (!scenario) return;
    if (dirty) return;
    if (servicesEnabled.size > 0) return;
    revertEdit(defaultServicesForStep(scenario, cursor));
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
  const isMultiMscc = scenario.serviceModel === 'multi-mscc';
  const servicesSelectedCount = servicesEnabled.size;
  const servicesTotal = scenario.services.length;

  const systemVars = Object.entries(context.system);
  const userVars = Object.entries(context.user);
  const extractedVars = Object.entries(context.extracted);

  return (
    <Stack gap="sm" h="100%" data-testid="debugger-step-editor-pane">
      {/* Header row */}
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

      {/* Two-column body */}
      <Flex gap="md" style={{ flex: 1, minHeight: 0 }}>
        {/* Left: variables + services */}
        <ScrollArea w={380} style={{ flexShrink: 0 }}>
          <Stack gap="xs">
            {systemVars.length > 0 && (
              <>
                <Divider label="System variables" labelPosition="left" />
                <Stack gap={4} data-testid="debugger-system-vars">
                  {systemVars.map(([k, v]) => (
                    <LockedVarRow key={k} name={k} value={v} />
                  ))}
                </Stack>
              </>
            )}

            {userVars.length > 0 && (
              <>
                <Divider label="Scenario variables" labelPosition="left" />
                <Stack gap={4} data-testid="debugger-user-vars">
                  {userVars.map(([k, v]) => (
                    <EditableVarRow
                      key={k}
                      name={k}
                      contextValue={String(v ?? '')}
                      override={overrides[k]}
                      disabled={!interactive}
                      onChange={(val) => setOverride(k, val)}
                    />
                  ))}
                </Stack>
              </>
            )}

            {extractedVars.length > 0 && (
              <>
                <Divider label="Extracted variables" labelPosition="left" />
                <Stack gap={4} data-testid="debugger-extracted-vars">
                  {extractedVars.map(([k, v]) => (
                    <LockedVarRow key={k} name={k} value={v} />
                  ))}
                </Stack>
              </>
            )}

            {systemVars.length === 0 &&
              userVars.length === 0 &&
              extractedVars.length === 0 && (
                <Text size="xs" c="dimmed">
                  No context variables yet.
                </Text>
              )}

            {isMultiMscc && (
              <>
                <Divider
                  label={`Services — ${servicesSelectedCount} of ${servicesTotal}`}
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
          </Stack>
        </ScrollArea>

        {/* Right: CCR preview */}
        <Stack gap="xs" style={{ flex: 1, minWidth: 0 }}>
          <Divider label="CCR preview" labelPosition="left" />
          <ScrollArea style={{ flex: 1 }}>
            <CcrPreview tree={previewTree} />
          </ScrollArea>
          <Group justify="flex-end">
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
        </Stack>
      </Flex>

      {/* Footer */}
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

interface LockedVarRowProps {
  name: string;
  value: unknown;
}

function LockedVarRow({ name, value }: LockedVarRowProps) {
  return (
    <Group gap="xs" wrap="nowrap" data-testid={`debugger-var-locked-${name}`}>
      <IconLock size={11} color="var(--mantine-color-gray-5)" />
      <Text size="xs" style={{ fontFamily: 'monospace', flexShrink: 0 }} miw={120}>
        {name}
      </Text>
      <Code style={{ fontSize: 11 }}>{stringify(value)}</Code>
    </Group>
  );
}

interface EditableVarRowProps {
  name: string;
  contextValue: string;
  override: string | undefined;
  disabled: boolean;
  onChange: (value: string) => void;
}

function EditableVarRow({
  name,
  contextValue,
  override,
  disabled,
  onChange,
}: EditableVarRowProps) {
  const isOverridden = override !== undefined;
  return (
    <Group gap="xs" wrap="nowrap" data-testid={`debugger-var-edit-${name}`}>
      <Text
        size="xs"
        style={{ fontFamily: 'monospace', flexShrink: 0 }}
        miw={120}
        c={isOverridden ? 'orange' : undefined}
      >
        {name}
      </Text>
      <TextInput
        size="xs"
        value={override ?? contextValue}
        disabled={disabled}
        placeholder={contextValue || '(empty)'}
        onChange={(e) => onChange(e.currentTarget.value)}
        style={{ flex: 1, fontFamily: 'monospace' }}
        styles={{
          input: {
            fontFamily: 'monospace',
            fontSize: 11,
            borderColor: isOverridden
              ? 'var(--mantine-color-orange-5)'
              : undefined,
          },
        }}
        data-testid={`debugger-var-input-${name}`}
      />
    </Group>
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
          <Code style={{ fontSize: 11 }} data-testid={`avp-${node.name}`}>
            {node.value}
          </Code>
        )}
      </Group>
      {node.children?.map((c, i) => (
        <PreviewNode key={`${c.code}-${i}`} node={c} depth={depth + 1} />
      ))}
    </Box>
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
