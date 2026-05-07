/**
 * Builder — Steps tab.
 *
 * Two-column layout: ordered list on the left (drag + arrow reorder, add /
 * remove), step editor on the right. Lifecycle of the steps array lives
 * in the Zustand draft store; this tab is a controlled view onto
 * `draft.steps` plus a `selectedIndex` local state.
 *
 * Drag-reorder is wired through `@dnd-kit/sortable`; the same array
 * mutation is used by the up/down arrow controls.
 */
import {
  ActionIcon,
  Badge,
  Card,
  Checkbox,
  Group,
  Menu,
  NumberInput,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import {
  IconArrowDown,
  IconArrowUp,
  IconDots,
  IconGripVertical,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useMediaQuery } from '@mantine/hooks';
import { useMemo, useState } from 'react';

import { listSystemVariables } from '../selectors';
import { useScenarioDraftStore } from '../scenarioDraftStore';
import type {
  RequestStep,
  RequestType,
  ScenarioStep,
  ServiceModel,
  SessionMode,
  VarValue,
} from '../types';
import { ExpressionInput } from './ExpressionInput';
import { isLegalRequestType, legalRequestTypes } from './stepsValidation';

/** Default kind / requestType for a freshly-added step. */
function defaultStep(mode: SessionMode): ScenarioStep {
  const requestType: RequestType = mode === 'session' ? 'UPDATE' : 'EVENT';
  const step: RequestStep = { kind: 'request', requestType };
  return step;
}

// ---------------------------------------------------------------------------
// Sortable row
// ---------------------------------------------------------------------------

interface SortableRowProps {
  step: ScenarioStep;
  index: number;
  total: number;
  selected: boolean;
  legalRequestType: boolean;
  /** When true, collapse the per-row actions into a kebab. */
  compactActions: boolean;
  onSelect: (i: number) => void;
  onMoveUp: (i: number) => void;
  onMoveDown: (i: number) => void;
  onRemove: (i: number) => void;
}

function SortableRow({
  step,
  index,
  total,
  selected,
  legalRequestType,
  compactActions,
  onSelect,
  onMoveUp,
  onMoveDown,
  onRemove,
}: SortableRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition } =
    useSortable({ id: index.toString() });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    backgroundColor: selected ? 'var(--mantine-color-blue-light)' : undefined,
    cursor: 'pointer',
  } as const;

  const stepLabel = step.label ?? step.requestType;
  const overrideCount = Object.keys(step.overrides ?? {}).length;

  return (
    <Table.Tr
      ref={setNodeRef}
      style={style}
      onClick={() => onSelect(index)}
      data-testid={`steps-row-${index}`}
    >
      <Table.Td
        onClick={(e) => e.stopPropagation()}
        style={{ paddingLeft: 4, paddingRight: 0, width: 56 }}
      >
        <Group gap={2} wrap="nowrap" align="center">
          <ActionIcon
            variant="subtle"
            aria-label="Drag to reorder"
            {...attributes}
            {...listeners}
          >
            <IconGripVertical size={14} />
          </ActionIcon>
          <Text size="sm" c="dimmed" component="span">
            {index + 1}
          </Text>
        </Group>
      </Table.Td>
      <Table.Td>
        <Group gap={4}>
          <Badge color={legalRequestType ? 'blue' : 'red'} variant="light">
            {stepLabel}
          </Badge>
          {!legalRequestType && (
            <Badge color="red" variant="filled" size="xs">
              invalid
            </Badge>
          )}
          {overrideCount > 0 && (
            <Badge color="orange" variant="outline" size="xs">
              {overrideCount} override{overrideCount === 1 ? '' : 's'}
            </Badge>
          )}
        </Group>
      </Table.Td>
      <Table.Td
        onClick={(e) => e.stopPropagation()}
        style={{ width: compactActions ? 40 : 120 }}
      >
        {compactActions ? (
          <Group justify="flex-end">
            <Menu position="bottom-end" withinPortal>
              <Menu.Target>
                <ActionIcon
                  variant="subtle"
                  aria-label="Step actions"
                  data-testid={`steps-actions-${index}`}
                >
                  <IconDots size={14} />
                </ActionIcon>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Item
                  leftSection={<IconArrowUp size={14} />}
                  disabled={index === 0}
                  onClick={() => onMoveUp(index)}
                  data-testid={`steps-up-${index}`}
                >
                  Move up
                </Menu.Item>
                <Menu.Item
                  leftSection={<IconArrowDown size={14} />}
                  disabled={index === total - 1}
                  onClick={() => onMoveDown(index)}
                  data-testid={`steps-down-${index}`}
                >
                  Move down
                </Menu.Item>
                <Menu.Divider />
                <Menu.Item
                  color="red"
                  leftSection={<IconTrash size={14} />}
                  onClick={() => onRemove(index)}
                  data-testid={`steps-remove-${index}`}
                >
                  Remove
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        ) : (
          <Group gap={2} justify="flex-end" wrap="nowrap">
            <ActionIcon
              variant="subtle"
              aria-label="Move up"
              disabled={index === 0}
              onClick={() => onMoveUp(index)}
              data-testid={`steps-up-${index}`}
            >
              <IconArrowUp size={14} />
            </ActionIcon>
            <ActionIcon
              variant="subtle"
              aria-label="Move down"
              disabled={index === total - 1}
              onClick={() => onMoveDown(index)}
              data-testid={`steps-down-${index}`}
            >
              <IconArrowDown size={14} />
            </ActionIcon>
            <ActionIcon
              variant="subtle"
              color="red"
              aria-label="Remove step"
              onClick={() => onRemove(index)}
              data-testid={`steps-remove-${index}`}
            >
              <IconTrash size={14} />
            </ActionIcon>
          </Group>
        )}
      </Table.Td>
    </Table.Tr>
  );
}

// ---------------------------------------------------------------------------
// Step editor (right pane)
// ---------------------------------------------------------------------------

interface StepEditorProps {
  step: ScenarioStep;
  index: number;
  sessionMode: SessionMode;
  variableNames: string[];
  context?: Parameters<typeof listSystemVariables>[0];
  onChange: (step: ScenarioStep) => void;
}

/** Build a zero-value test variable map from scenario + system variable names. */
function buildTestVars(
  variableNames: string[],
  context?: Parameters<typeof listSystemVariables>[0],
): Record<string, unknown> {
  const vars: Record<string, unknown> = {};
  // Seed system CCA auto-variables with representative values.
  for (const sv of listSystemVariables(context)) {
    vars[sv.name] = sv.name.includes('RESULT') ? 2001
      : sv.name.includes('FUI') ? -1
      : sv.name.includes('GRANTED') || sv.name.includes('VALIDITY') ? 0
      : '';
  }
  // User-defined variables default to 0.
  for (const name of variableNames) {
    if (!(name in vars)) vars[name] = 0;
  }
  return vars;
}

function StepEditor({
  step,
  index,
  sessionMode,
  variableNames,
  context,
  onChange,
}: StepEditorProps) {
  const testVars = buildTestVars(variableNames, context);
  return (
    <Stack gap="md">
      <Title order={5}>Step {index + 1}</Title>
      <RequestFields
        step={step as RequestStep}
        sessionMode={sessionMode}
        variableNames={variableNames}
        testVars={testVars}
        onChange={onChange}
      />
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Per-kind field clusters
// ---------------------------------------------------------------------------

interface RequestFieldsProps {
  step: RequestStep;
  sessionMode: SessionMode;
  variableNames: string[];
  testVars: Record<string, unknown>;
  onChange: (step: ScenarioStep) => void;
}

function RequestFields({ step, sessionMode, variableNames, testVars, onChange }: RequestFieldsProps) {
  const legal = legalRequestTypes(sessionMode);
  const requestTypeError = !isLegalRequestType(sessionMode, step.requestType)
    ? `Request type ${step.requestType} is not legal under sessionMode ${sessionMode}`
    : null;

  const isUpdate = step.requestType === 'UPDATE';
  const useUntil = step.repeatUntil !== undefined;
  const useValidity = step.useValidityTime ?? false;

  return (
    <Stack gap="md">
      <TextInput
        label="Label (optional)"
        placeholder="e.g. CCR-U data"
        value={step.label ?? ''}
        onChange={(e) => onChange({ ...step, label: e.currentTarget.value || undefined })}
      />

      <Select
        label="Request type"
        data={(['INITIAL', 'UPDATE', 'TERMINATE', 'EVENT'] as RequestType[]).map(
          (rt) => ({
            value: rt,
            label: rt,
            disabled: !legal.includes(rt),
          }),
        )}
        value={step.requestType}
        onChange={(v) =>
          v && onChange({ ...step, requestType: v as RequestType })
        }
        error={requestTypeError}
        allowDeselect={false}
        data-testid="step-request-type"
      />

      {isUpdate && (
        <Stack gap="xs">
          <Text size="sm" fw={500}>Repeat</Text>

          <Checkbox
            label="Repeat until condition"
            checked={useUntil}
            onChange={(e) =>
              onChange({
                ...step,
                repeatUntil: e.currentTarget.checked ? '' : undefined,
                repeat: e.currentTarget.checked ? undefined : (step.repeat ?? 1),
              })
            }
          />

          {useUntil ? (
            <Stack gap="xs">
              <ExpressionInput
                label="Exit when (expression)"
                placeholder="e.g. {{RESULT_CODE}} != 2001"
                value={step.repeatUntil ?? ''}
                onChange={(v) => onChange({ ...step, repeatUntil: v })}
                testVars={testVars}
              />
              <NumberInput
                label="Max iterations (safety cap)"
                min={1}
                value={step.maxRepeat ?? ''}
                onChange={(v) =>
                  onChange({ ...step, maxRepeat: typeof v === 'number' ? v : undefined })
                }
              />
            </Stack>
          ) : (
            <NumberInput
              label="Repeat count"
              description="Number of times to send this step"
              min={1}
              value={step.repeat ?? 1}
              onChange={(v) =>
                onChange({ ...step, repeat: typeof v === 'number' ? v : 1 })
              }
            />
          )}

          <Text size="sm" fw={500} mt="xs">Delay between sends</Text>

          <Checkbox
            label="Use Validity-Time from CCA"
            checked={useValidity}
            onChange={(e) =>
              onChange({ ...step, useValidityTime: e.currentTarget.checked || undefined })
            }
          />

          {useValidity ? (
            <NumberInput
              label="Validity-Time scale (0.8 = 80% of interval)"
              min={0.1}
              max={2}
              step={0.1}
              decimalScale={2}
              value={step.validityScale ?? 1}
              onChange={(v) =>
                onChange({ ...step, validityScale: typeof v === 'number' ? v : 1 })
              }
            />
          ) : (
            <Group grow>
              <NumberInput
                label="Base delay (s)"
                min={0}
                value={step.delaySec ?? 0}
                onChange={(v) =>
                  onChange({ ...step, delaySec: typeof v === 'number' ? v : 0 })
                }
              />
              <NumberInput
                label="Jitter (s)"
                description="Random ±jitter added each send"
                min={0}
                value={step.delayJitterSec ?? 0}
                onChange={(v) =>
                  onChange({ ...step, delayJitterSec: typeof v === 'number' ? v : 0 })
                }
              />
            </Group>
          )}
        </Stack>
      )}

      <OverridesEditor
        overrides={step.overrides ?? {}}
        variableNames={variableNames}
        onChange={(overrides) => onChange({ ...step, overrides })}
      />
    </Stack>
  );
}

type OverridesMap = { [key: string]: VarValue };

interface OverridesEditorProps {
  overrides: OverridesMap;
  variableNames: string[];
  onChange: (overrides: OverridesMap) => void;
}

function OverridesEditor({
  overrides,
  variableNames,
  onChange,
}: OverridesEditorProps) {
  const entries = Object.entries(overrides);
  const remaining = variableNames.filter((n) => !(n in overrides));

  return (
    <Stack gap="xs">
      <Text size="sm" fw={500}>
        Variable overrides
      </Text>
      {entries.length === 0 ? (
        <Text size="sm" c="dimmed">
          No overrides — step will use scenario defaults.
        </Text>
      ) : (
        <Stack gap={4}>
          {entries.map(([name, value]) => (
            <Group key={name} gap="xs" wrap="nowrap">
              <Badge color="orange" variant="light">
                OVERRIDE
              </Badge>
              <Text style={{ flex: 1 }}>{name}</Text>
              <input
                value={String(value ?? '')}
                onChange={(e) =>
                  onChange({ ...overrides, [name]: e.currentTarget.value })
                }
                aria-label={`Override value for ${name}`}
                style={{ flex: 1 }}
              />
              <ActionIcon
                variant="subtle"
                color="red"
                aria-label={`Remove override ${name}`}
                onClick={() => {
                  const next = { ...overrides };
                  delete next[name];
                  onChange(next);
                }}
              >
                <IconTrash size={14} />
              </ActionIcon>
            </Group>
          ))}
        </Stack>
      )}
      {remaining.length > 0 && (
        <Select
          placeholder="Add override for variable…"
          data={remaining.map((n) => ({ value: n, label: n }))}
          value={null}
          onChange={(v) => v && onChange({ ...overrides, [v]: '' })}
          searchable
          clearable
        />
      )}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Tab body
// ---------------------------------------------------------------------------

export function StepsTab() {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setSteps = useScenarioDraftStore((s) => s.setSteps);

  // Per-row actions: inline when there's room, kebab when the
  // viewport is genuinely cramped. Threshold mirrors the modal's
  // "tablet or smaller" breakpoint.
  const compactActions = useMediaQuery('(max-width: 768px)') ?? false;

  const sessionMode = draft?.sessionMode ?? 'session';
  const steps = useMemo(() => draft?.steps ?? [], [draft?.steps]);
  const variableNames = useMemo(
    () => (draft?.variables ?? []).map((v) => v.name),
    [draft?.variables],
  );

  const [selectedIndex, setSelectedIndex] = useState<number>(0);

  // Clamp the selection if the list shrunk underneath us.
  // Computed during render, not in an effect — `react-hooks/set-state-in-effect`.
  const clamped =
    steps.length === 0
      ? 0
      : Math.min(selectedIndex, Math.max(0, steps.length - 1));
  const effectiveIndex = clamped;

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  function moveTo(from: number, to: number) {
    if (from === to || to < 0 || to >= steps.length) return;
    setSteps(arrayMove(steps, from, to));
    if (effectiveIndex === from) setSelectedIndex(to);
  }

  function handleDragEnd(e: DragEndEvent) {
    const from = Number(e.active.id);
    const to = Number(e.over?.id ?? from);
    if (Number.isNaN(from) || Number.isNaN(to)) return;
    moveTo(from, to);
  }

  function addStep() {
    const next = [...steps, defaultStep(sessionMode)];
    setSteps(next);
    setSelectedIndex(next.length - 1);
  }

  function removeStep(i: number) {
    const next = steps.filter((_, idx) => idx !== i);
    setSteps(next);
    if (effectiveIndex >= next.length) {
      setSelectedIndex(Math.max(0, next.length - 1));
    }
  }

  function updateStep(i: number, step: ScenarioStep) {
    const next = steps.map((s, idx) => (idx === i ? step : s));
    setSteps(next);
  }

  if (!draft) return null;
  const selected = steps[effectiveIndex];

  return (
    <Group align="flex-start" gap="lg" wrap="wrap">
      <Card withBorder padding="sm" style={{ flex: '1 1 280px', minWidth: 280 }}>
        <Stack gap="xs">
          <Group justify="space-between">
            <Title order={5}>Steps</Title>
            <ActionIcon
              variant="filled"
              aria-label="Add step"
              onClick={addStep}
              data-testid="steps-add"
            >
              <IconPlus size={16} />
            </ActionIcon>
          </Group>
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragEnd={handleDragEnd}
          >
            <SortableContext
              items={steps.map((_, i) => i.toString())}
              strategy={verticalListSortingStrategy}
            >
              <Table>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>#</Table.Th>
                    <Table.Th>Type</Table.Th>
                    <Table.Th aria-label="Actions" />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {steps.map((step, i) => (
                    <SortableRow
                      key={i}
                      step={step}
                      index={i}
                      total={steps.length}
                      selected={i === effectiveIndex}
                      legalRequestType={
                        step.kind !== 'request' ||
                        isLegalRequestType(sessionMode, step.requestType)
                      }
                      compactActions={compactActions}
                      onSelect={setSelectedIndex}
                      onMoveUp={(idx) => moveTo(idx, idx - 1)}
                      onMoveDown={(idx) => moveTo(idx, idx + 1)}
                      onRemove={removeStep}
                    />
                  ))}
                </Table.Tbody>
              </Table>
            </SortableContext>
          </DndContext>
          {steps.length === 0 && (
            <Text c="dimmed" ta="center">
              No steps yet — click + to add one.
            </Text>
          )}
        </Stack>
      </Card>

      <Card withBorder padding="md" style={{ flex: '2 1 360px', minWidth: 320 }}>
        {selected ? (
          <StepEditor
            step={selected}
            index={effectiveIndex}
            sessionMode={sessionMode}
            variableNames={variableNames}
            context={draft ? { services: draft.services, serviceModel: draft.serviceModel as ServiceModel } : undefined}
            onChange={(s) => updateStep(effectiveIndex, s)}
          />
        ) : (
          <Text c="dimmed">Select a step to edit it.</Text>
        )}
      </Card>
    </Group>
  );
}
