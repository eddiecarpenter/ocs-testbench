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
  Button,
  Card,
  Checkbox,
  Group,
  Menu,
  NumberInput,
  Select,
  Stack,
  Table,
  Text,
  Textarea,
  Title,
} from '@mantine/core';
import {
  IconArrowDown,
  IconArrowUp,
  IconDots,
  IconGripVertical,
  IconPlus,
  IconTrash,
  IconX,
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

import { useScenarioDraftStore } from '../scenarioDraftStore';
import type {
  ConsumeStep,
  PauseStep,
  Predicate,
  RequestStep,
  RequestType,
  ScenarioStep,
  SessionMode,
  UpdateRepeatPolicy,
  VarValue,
  WaitStep,
} from '../types';
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

  const stepLabel = step.kind === 'request' ? step.requestType : step.kind.toUpperCase();
  const overrideCount =
    step.kind === 'request' || step.kind === 'consume'
      ? Object.keys(step.overrides ?? {}).length
      : 0;

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
  onChange: (step: ScenarioStep) => void;
}

function StepEditor({
  step,
  index,
  sessionMode,
  variableNames,
  onChange,
}: StepEditorProps) {
  return (
    <Stack gap="md">
      <Title order={5}>Step {index + 1}</Title>

      <Select
        label="Step kind"
        // `consume` is intentionally not offered for new steps — the
        // "Update with repeat" affordance under Request·UPDATE captures
        // the same capability with cleaner naming. Existing scenarios
        // that already carry `consume` steps still load and edit (the
        // ConsumeFields branch below renders for them); the user just
        // can't *create* new consume steps from the dropdown.
        data={[
          { value: 'request', label: 'Request' },
          { value: 'wait', label: 'Wait' },
          { value: 'pause', label: 'Pause' },
          // Surfaced only when the step is already a consume — keeps
          // the dropdown from hiding the row's actual kind.
          ...(step.kind === 'consume'
            ? [{ value: 'consume', label: 'Consume (legacy)' }]
            : []),
        ]}
        value={step.kind}
        onChange={(v) => {
          if (!v || v === step.kind) return;
          if (v === 'request') {
            const fresh: RequestStep = {
              kind: 'request',
              requestType:
                sessionMode === 'session' ? 'UPDATE' : 'EVENT',
            };
            onChange(fresh);
          } else if (v === 'wait') {
            const fresh: WaitStep = { kind: 'wait', durationMs: 1000 };
            onChange(fresh);
          } else if (v === 'pause') {
            const fresh: PauseStep = { kind: 'pause' };
            onChange(fresh);
          }
          // No `consume` create path — see the dropdown comment above.
        }}
        allowDeselect={false}
      />

      {step.kind === 'request' && (
        <RequestFields
          step={step}
          sessionMode={sessionMode}
          variableNames={variableNames}
          onChange={onChange}
        />
      )}

      {step.kind === 'consume' && (
        <ConsumeFields
          step={step}
          variableNames={variableNames}
          onChange={onChange}
        />
      )}

      {step.kind === 'wait' && (
        <NumberInput
          label="Duration (ms)"
          min={0}
          value={step.durationMs}
          onChange={(v) =>
            onChange({ ...step, durationMs: typeof v === 'number' ? v : 0 })
          }
        />
      )}

      {step.kind === 'pause' && (
        <Stack gap="xs">
          <Textarea
            label="Pause prompt (optional)"
            value={step.prompt ?? ''}
            onChange={(e) =>
              onChange({ ...step, prompt: e.currentTarget.value })
            }
          />
        </Stack>
      )}
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
  onChange: (step: ScenarioStep) => void;
}

function RequestFields({ step, sessionMode, variableNames, onChange }: RequestFieldsProps) {
  const legal = legalRequestTypes(sessionMode);
  const requestTypeError = !isLegalRequestType(sessionMode, step.requestType)
    ? `Request type ${step.requestType} is not legal under sessionMode ${sessionMode}`
    : null;

  const isUpdate = step.requestType === 'UPDATE';

  return (
    <Stack gap="md">
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
        onChange={(v) => {
          if (!v) return;
          const next: RequestStep = { ...step, requestType: v as RequestType };
          // `repeat` is UPDATE-only per the schema's if/then/else
          // narrowing — strip it when the user picks anything else
          // so the saved scenario stays valid.
          if (v !== 'UPDATE' && next.repeat !== undefined) {
            delete next.repeat;
          }
          onChange(next);
        }}
        error={requestTypeError}
        allowDeselect={false}
        data-testid="step-request-type"
      />

      {isUpdate && (
        <RepeatPolicyEditor
          policy={step.repeat}
          variableNames={variableNames}
          onChange={(policy) => {
            if (policy === undefined) {
              const { repeat: _drop, ...rest } = step;
              void _drop;
              onChange(rest);
            } else {
              onChange({ ...step, repeat: policy });
            }
          }}
        />
      )}

      <OverridesEditor
        overrides={step.overrides ?? {}}
        variableNames={variableNames}
        onChange={(overrides) => onChange({ ...step, overrides })}
      />
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Repeat-policy editor — UPDATE-only
// ---------------------------------------------------------------------------

interface RepeatPolicyEditorProps {
  policy: UpdateRepeatPolicy | undefined;
  variableNames: string[];
  onChange: (policy: UpdateRepeatPolicy | undefined) => void;
}

const PREDICATE_OPS: { value: Predicate['op']; label: string }[] = [
  { value: 'lt', label: '<' },
  { value: 'lte', label: '≤' },
  { value: 'eq', label: '=' },
  { value: 'ne', label: '≠' },
  { value: 'gte', label: '≥' },
  { value: 'gt', label: '>' },
];

function RepeatPolicyEditor({
  policy,
  variableNames,
  onChange,
}: RepeatPolicyEditorProps) {
  const enabled = policy !== undefined;
  const hasTimes = policy?.times !== undefined;
  const hasUntil = policy?.until !== undefined;
  // Schema requires at least one bound — surface that as a form-level
  // validation message so the user knows why the save would fail.
  const missingBound = enabled && !hasTimes && !hasUntil;

  return (
    <Stack gap="xs" data-testid="step-repeat-editor">
      <Checkbox
        label="Repeat this step"
        description="Send the CCR-UPDATE multiple times in a loop, optionally bounded by a count and/or an exit condition."
        checked={enabled}
        onChange={(e) => {
          if (e.currentTarget.checked) {
            onChange({ times: 1, delayMs: 0 });
          } else {
            onChange(undefined);
          }
        }}
      />

      {enabled && (
        <Stack gap="sm" pl="md">
          <Group gap="xs" align="flex-end" grow>
            <NumberInput
              label="Repeat (times)"
              description="Hard cap on iterations. Includes the first CCR-UPDATE."
              min={1}
              value={policy?.times ?? ''}
              onChange={(v) =>
                onChange({
                  ...(policy ?? {}),
                  times: typeof v === 'number' && v >= 1 ? v : undefined,
                })
              }
              data-testid="step-repeat-times"
            />
            <NumberInput
              label="Delay between (ms)"
              description="Sleep between iterations — after CCA, before next CCR."
              min={0}
              value={policy?.delayMs ?? 0}
              onChange={(v) =>
                onChange({
                  ...(policy ?? {}),
                  delayMs: typeof v === 'number' && v >= 0 ? v : 0,
                })
              }
              data-testid="step-repeat-delay"
            />
          </Group>

          <Stack gap={4}>
            <Text size="sm" fw={500}>
              Stop when (optional)
            </Text>
            {hasUntil ? (
              <Group gap="xs" wrap="nowrap" align="center">
                <Select
                  data={variableNames.map((n) => ({ value: n, label: n }))}
                  value={policy?.until?.variable ?? null}
                  searchable
                  placeholder="Variable"
                  onChange={(v) =>
                    v &&
                    onChange({
                      ...(policy ?? {}),
                      until: {
                        ...(policy?.until ?? { op: 'gte', value: 0 }),
                        variable: v,
                      },
                    })
                  }
                  style={{ flex: 1 }}
                  data-testid="step-repeat-until-variable"
                />
                <Select
                  data={PREDICATE_OPS}
                  value={policy?.until?.op ?? 'gte'}
                  onChange={(v) =>
                    v &&
                    onChange({
                      ...(policy ?? {}),
                      until: {
                        ...(policy?.until ?? { variable: '', value: 0 }),
                        op: v as Predicate['op'],
                      },
                    })
                  }
                  allowDeselect={false}
                  w={80}
                  data-testid="step-repeat-until-op"
                />
                <input
                  value={String(policy?.until?.value ?? '')}
                  placeholder="Value"
                  onChange={(e) => {
                    const raw = e.currentTarget.value;
                    // Cheap coercion — number when parseable, else string.
                    const num = Number(raw);
                    const value: Predicate['value'] =
                      raw !== '' && Number.isFinite(num) ? num : raw;
                    onChange({
                      ...(policy ?? {}),
                      until: {
                        ...(policy?.until ?? { variable: '', op: 'gte' }),
                        value,
                      },
                    });
                  }}
                  style={{ flex: 1, padding: '6px 8px' }}
                  data-testid="step-repeat-until-value"
                />
                <ActionIcon
                  variant="subtle"
                  color="red"
                  aria-label="Remove stop condition"
                  onClick={() => {
                    const next = { ...(policy ?? {}) };
                    delete next.until;
                    onChange(next);
                  }}
                >
                  <IconX size={14} />
                </ActionIcon>
              </Group>
            ) : (
              <Button
                variant="default"
                size="xs"
                leftSection={<IconPlus size={12} />}
                onClick={() =>
                  onChange({
                    ...(policy ?? {}),
                    until: {
                      variable: variableNames[0] ?? '',
                      op: 'gte',
                      value: 0,
                    },
                  })
                }
                data-testid="step-repeat-add-until"
              >
                Add stop condition
              </Button>
            )}
          </Stack>

          {missingBound && (
            <Text size="xs" c="red">
              Provide either a `times` cap or a `stop when` condition —
              an unbounded loop is invalid.
            </Text>
          )}
        </Stack>
      )}
    </Stack>
  );
}

interface ConsumeFieldsProps {
  step: ConsumeStep;
  variableNames: string[];
  onChange: (step: ScenarioStep) => void;
}

function ConsumeFields({ step, variableNames, onChange }: ConsumeFieldsProps) {
  return (
    <Stack gap="md">
      <NumberInput
        label="Window (ms)"
        min={0}
        value={step.windowMs}
        onChange={(v) =>
          onChange({ ...step, windowMs: typeof v === 'number' ? v : 0 })
        }
      />
      <NumberInput
        label="Max rounds (optional)"
        min={0}
        value={step.maxRounds ?? ''}
        onChange={(v) =>
          onChange({
            ...step,
            maxRounds: typeof v === 'number' ? v : undefined,
          })
        }
      />
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
            onChange={(s) => updateStep(effectiveIndex, s)}
          />
        ) : (
          <Text c="dimmed">Select a step to edit it.</Text>
        )}
      </Card>
    </Group>
  );
}
