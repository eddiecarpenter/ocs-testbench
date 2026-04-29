/**
 * Builder — Variables tab.
 *
 * Three-column layout:
 *
 *   left  — sidebar: System variables (Generated, Bound) listed first,
 *           User variables (Generator, Bound, Extracted) below; each
 *           entry has a coloured chip (GEN / BOUND / EXTR).
 *   mid   — definition pane: read-only for system entries; editable
 *           for user entries with shape-by-source-kind fields.
 *   right — Usage pane: every reference, labelled by tab + location,
 *           with a deep-link button that swaps the active tab via
 *           `?tab=…&select=…&field=…`.
 *
 * Auto-provisioning button surfaces the rules from
 * `provisionVariables.ts` — the user clicks to merge missing
 * service-implied variables into the user list.
 */
import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Group,
  Modal,
  NumberInput,
  Select,
  Stack,
  Text,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import {
  IconArrowRight,
  IconPlus,
  IconTrash,
  IconWand,
} from '@tabler/icons-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { provisionVariables } from '../provisionVariables';
import { useScenarioDraftStore } from '../scenarioDraftStore';
import {
  type SystemVariable,
  type UsageRef,
  findUsages,
  listSystemVariables,
} from '../selectors';
import type {
  GeneratorRefresh,
  GeneratorStrategy,
  Variable,
  VariableSource,
  VariableSourceBound,
  VariableSourceExtracted,
  VariableSourceGenerator,
} from '../types';

type VarKind = 'generator' | 'bound' | 'extracted';

function chipFor(kind: VarKind) {
  if (kind === 'generator') {
    return (
      <Badge color="blue" variant="light" size="xs">
        GEN
      </Badge>
    );
  }
  if (kind === 'bound') {
    return (
      <Badge color="cyan" variant="light" size="xs">
        BOUND
      </Badge>
    );
  }
  return (
    <Badge color="grape" variant="light" size="xs">
      EXTR
    </Badge>
  );
}

interface SidebarProps {
  system: SystemVariable[];
  user: Variable[];
  selected: string;
  /** Reference count per user-variable name (0 → trash enabled). */
  userUsageCounts: Record<string, number>;
  onSelect: (name: string, isSystem: boolean) => void;
  onAdd: () => void;
  onProvision: () => void;
  onRequestRemove: (name: string) => void;
}

function Sidebar({
  system,
  user,
  selected,
  userUsageCounts,
  onSelect,
  onAdd,
  onProvision,
  onRequestRemove,
}: SidebarProps) {
  return (
    <Card withBorder padding="sm" style={{ width: 280 }}>
      <Stack gap="xs">
        <Group justify="space-between">
          <Title order={6}>System</Title>
          <Badge variant="outline">{system.length}</Badge>
        </Group>
        <Stack gap={2}>
          {system.map((v) => (
            <Group
              key={v.name}
              gap="xs"
              wrap="nowrap"
              style={{
                cursor: 'pointer',
                padding: '4px 6px',
                borderRadius: 4,
                backgroundColor:
                  selected === `system:${v.name}`
                    ? 'var(--mantine-color-blue-light)'
                    : undefined,
              }}
              onClick={() => onSelect(v.name, true)}
            >
              {chipFor(v.kind)}
              <Text size="sm">{v.name}</Text>
            </Group>
          ))}
        </Stack>
        <Group justify="space-between">
          <Title order={6}>User</Title>
          <Group gap={4}>
            <Tooltip
              label="Auto-provision: add the variables this scenario's services and serviceModel imply but aren't defined yet (won't overwrite existing ones)"
              multiline
              w={260}
              withArrow
            >
              <ActionIcon
                variant="subtle"
                size="sm"
                aria-label="Auto-provision missing variables"
                onClick={onProvision}
                data-testid="vars-provision"
              >
                <IconWand size={14} />
              </ActionIcon>
            </Tooltip>
            <Tooltip label="Add variable" withArrow>
              <ActionIcon
                variant="filled"
                size="sm"
                aria-label="Add variable"
                onClick={onAdd}
                data-testid="vars-add"
              >
                <IconPlus size={14} />
              </ActionIcon>
            </Tooltip>
          </Group>
        </Group>
        <Stack gap={2}>
          {user.length === 0 && (
            <Text size="sm" c="dimmed">
              No user variables yet.
            </Text>
          )}
          {user.map((v) => {
            const inUseCount = userUsageCounts[v.name] ?? 0;
            const inUse = inUseCount > 0;
            return (
              <Group
                key={v.name}
                gap="xs"
                wrap="nowrap"
                style={{
                  cursor: 'pointer',
                  padding: '4px 6px',
                  borderRadius: 4,
                  backgroundColor:
                    selected === `user:${v.name}`
                      ? 'var(--mantine-color-blue-light)'
                      : undefined,
                }}
                onClick={() => onSelect(v.name, false)}
              >
                {chipFor(v.source.kind)}
                <Text size="sm" style={{ flex: 1 }}>
                  {v.name}
                </Text>
                <Tooltip
                  label={
                    inUse
                      ? `In use — referenced in ${inUseCount} place${inUseCount === 1 ? '' : 's'}`
                      : 'Remove variable'
                  }
                >
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    size="sm"
                    aria-label={`Remove ${v.name}`}
                    disabled={inUse}
                    onClick={(e) => {
                      e.stopPropagation();
                      onRequestRemove(v.name);
                    }}
                    data-testid={`vars-remove-${v.name}`}
                  >
                    <IconTrash size={14} />
                  </ActionIcon>
                </Tooltip>
              </Group>
            );
          })}
        </Stack>
      </Stack>
    </Card>
  );
}

interface GeneratorFieldsProps {
  source: VariableSourceGenerator;
  onChange: (src: VariableSourceGenerator) => void;
}

function GeneratorFields({ source, onChange }: GeneratorFieldsProps) {
  return (
    <>
      <Select
        label="Strategy"
        data={[
          'literal',
          'uuid',
          'incrementer',
          'random-int',
          'random-string',
          'random-choice',
        ].map((s) => ({ value: s, label: s }))}
        value={source.strategy}
        onChange={(v) =>
          v && onChange({ ...source, strategy: v as GeneratorStrategy })
        }
        allowDeselect={false}
      />
      <Select
        label="Refresh"
        data={[
          { value: 'once', label: 'Once per execution' },
          { value: 'per-send', label: 'Per CCR send' },
        ]}
        value={source.refresh}
        onChange={(v) =>
          v && onChange({ ...source, refresh: v as GeneratorRefresh })
        }
        allowDeselect={false}
      />
      <NumberInput
        label="Literal value (when strategy = literal)"
        value={Number(source.params?.value ?? 0)}
        onChange={(v) =>
          onChange({
            ...source,
            params: {
              ...(source.params ?? {}),
              value: typeof v === 'number' ? v : 0,
            },
          })
        }
      />
    </>
  );
}

interface BoundFieldsProps {
  source: VariableSourceBound;
  onChange: (src: VariableSourceBound) => void;
}

function BoundFields({ source, onChange }: BoundFieldsProps) {
  return (
    <>
      <Select
        label="From"
        data={[
          { value: 'subscriber', label: 'Subscriber' },
          { value: 'peer', label: 'Peer' },
          { value: 'config', label: 'Config' },
          { value: 'step', label: 'Step' },
        ]}
        value={source.from}
        onChange={(v) =>
          v &&
          onChange({
            ...source,
            from: v as VariableSourceBound['from'],
          })
        }
        allowDeselect={false}
      />
      <TextInput
        label="Field"
        value={source.field}
        onChange={(e) =>
          onChange({
            ...source,
            field: e.currentTarget.value,
          })
        }
      />
    </>
  );
}

interface ExtractedFieldsProps {
  source: VariableSourceExtracted;
  onChange: (src: VariableSourceExtracted) => void;
}

function ExtractedFields({ source, onChange }: ExtractedFieldsProps) {
  return (
    <>
      <TextInput
        label="Source AVP path"
        description="Dot/bracket notation, e.g. MSCC[100].Granted-Service-Unit.CC-Total-Octets"
        value={source.path}
        onChange={(e) =>
          onChange({
            ...source,
            path: e.currentTarget.value,
          })
        }
      />
      <TextInput
        label="Transform (optional)"
        value={source.transform ?? ''}
        onChange={(e) =>
          onChange({
            ...source,
            transform: e.currentTarget.value,
          })
        }
      />
    </>
  );
}

interface DefinitionPaneProps {
  variable: Variable;
  onChange: (next: Variable) => void;
}

function DefinitionPane({ variable, onChange }: DefinitionPaneProps) {
  return (
    <Card withBorder padding="md" style={{ flex: 1, minWidth: 320 }}>
      <Stack gap="md">
        <TextInput
          label="Name"
          value={variable.name}
          onChange={(e) =>
            onChange({ ...variable, name: e.currentTarget.value })
          }
        />
        <TextInput
          label="Description"
          value={variable.description ?? ''}
          onChange={(e) =>
            onChange({ ...variable, description: e.currentTarget.value })
          }
        />
        <Select
          label="Source kind"
          data={[
            { value: 'generator', label: 'Generator (GEN)' },
            { value: 'bound', label: 'Bound (BOUND)' },
            { value: 'extracted', label: 'Extracted (EXTR)' },
          ]}
          value={variable.source.kind}
          onChange={(v) => {
            if (!v) return;
            const newSource: VariableSource =
              v === 'generator'
                ? {
                    kind: 'generator',
                    strategy: 'literal',
                    refresh: 'once',
                    params: { value: 0 },
                  }
                : v === 'bound'
                ? { kind: 'bound', from: 'subscriber', field: 'msisdn' }
                : { kind: 'extracted', path: '' };
            onChange({ ...variable, source: newSource });
          }}
          allowDeselect={false}
        />

        {variable.source.kind === 'generator' && (
          <GeneratorFields
            source={variable.source}
            onChange={(src) => onChange({ ...variable, source: src })}
          />
        )}

        {variable.source.kind === 'bound' && (
          <BoundFields
            source={variable.source}
            onChange={(src) => onChange({ ...variable, source: src })}
          />
        )}

        {variable.source.kind === 'extracted' && (
          <ExtractedFields
            source={variable.source}
            onChange={(src) => onChange({ ...variable, source: src })}
          />
        )}
      </Stack>
    </Card>
  );
}

interface UsagePaneProps {
  refs: UsageRef[];
  onJump: (ref: UsageRef) => void;
}

function UsagePane({ refs, onJump }: UsagePaneProps) {
  return (
    <Card withBorder padding="md" style={{ width: 320 }}>
      <Stack gap="xs">
        <Title order={6}>Usage</Title>
        {refs.length === 0 ? (
          <Text size="sm" c="dimmed">
            No references found.
          </Text>
        ) : (
          refs.map((r, i) => (
            <Group key={i} gap="xs" wrap="nowrap" justify="space-between">
              <Text size="sm" style={{ flex: 1 }}>
                {r.label}
              </Text>
              <ActionIcon
                variant="subtle"
                onClick={() => onJump(r)}
                aria-label="Open in source tab"
                data-testid={`vars-usage-jump-${i}`}
              >
                <IconArrowRight size={14} />
              </ActionIcon>
            </Group>
          ))
        )}
      </Stack>
    </Card>
  );
}

export function VariablesTab() {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setVariables = useScenarioDraftStore((s) => s.setVariables);
  const updateVariable = useScenarioDraftStore((s) => s.updateVariable);
  const removeVariable = useScenarioDraftStore((s) => s.removeVariable);
  const [, setParams] = useSearchParams();
  const [pendingDelete, setPendingDelete] = useState<{
    name: string;
    usages: UsageRef[];
  } | null>(null);

  const system = listSystemVariables();
  const user = draft?.variables ?? [];
  const allByName = new Map<string, { isSystem: boolean }>();
  for (const v of system) allByName.set(v.name, { isSystem: true });
  for (const v of user) allByName.set(v.name, { isSystem: false });

  const [selected, setSelected] = useState<string>(() => {
    if (user.length > 0) return `user:${user[0].name}`;
    if (system.length > 0) return `system:${system[0].name}`;
    return '';
  });

  if (!draft) return null;

  const [scope, name] = selected.split(':') as ['system' | 'user', string];
  const selectedSystem =
    scope === 'system' ? system.find((s) => s.name === name) : undefined;
  const selectedUser =
    scope === 'user' ? user.find((u) => u.name === name) : undefined;
  const selectedName = selectedSystem?.name ?? selectedUser?.name ?? '';

  const usages = selectedName ? findUsages(draft, selectedName) : [];

  function handleAdd() {
    const baseName = `VAR_${user.length + 1}`;
    const next: Variable = {
      name: baseName,
      description: '',
      source: {
        kind: 'generator',
        strategy: 'literal',
        refresh: 'once',
        params: { value: 0 },
      },
    };
    setVariables([...user, next]);
    setSelected(`user:${baseName}`);
  }

  function handleProvision() {
    if (!draft) return;
    const next = provisionVariables(draft);
    setVariables(next);
  }

  function handleChange(updated: Variable) {
    if (!selectedUser) return;
    const trimmed = updated.name.trim();
    const isRename = trimmed !== selectedUser.name;
    if (isRename) {
      // Reject empty names and collisions with another existing
      // variable (system or user) so refactor stays unambiguous.
      if (trimmed === '') {
        notifications.show({
          color: 'red',
          title: 'Name cannot be empty',
          message: 'Variable names must contain at least one character.',
        });
        return;
      }
      if (
        trimmed !== selectedUser.name &&
        allByName.has(trimmed)
      ) {
        notifications.show({
          color: 'red',
          title: 'Name already in use',
          message: `Another variable is already called "${trimmed}".`,
        });
        return;
      }
    }
    // Atomic rename + propagate (single undo step).
    updateVariable(selectedUser.name, { ...updated, name: trimmed });
    if (isRename) setSelected(`user:${trimmed}`);
  }

  function requestRemove(name: string) {
    if (!draft) return;
    const refs = findUsages(draft, name);
    // Open the modal whether or not usages exist; modal renders the
    // appropriate "blocked, in use" or "confirm delete" body based on
    // refs.length.
    setPendingDelete({ name, usages: refs });
  }

  function confirmDelete() {
    if (!pendingDelete || pendingDelete.usages.length > 0) return;
    removeVariable(pendingDelete.name);
    const remaining = user.filter((v) => v.name !== pendingDelete.name);
    setSelected(remaining.length > 0 ? `user:${remaining[0].name}` : '');
    setPendingDelete(null);
  }

  function handleJump(ref: UsageRef) {
    setParams(
      (prev) => {
        const np = new URLSearchParams(prev);
        np.set('tab', ref.tab);
        if (ref.select) np.set('select', ref.select);
        else np.delete('select');
        if (ref.field) np.set('field', ref.field);
        else np.delete('field');
        return np;
      },
      { replace: false },
    );
  }

  // Per-user-variable usage count, computed once and threaded into the
  // sidebar so each row's trash can decide enabled vs disabled.
  const userUsageCounts: Record<string, number> = {};
  for (const v of user) {
    userUsageCounts[v.name] = findUsages(draft, v.name).length;
  }

  return (
    <Group align="flex-start" gap="lg" wrap="nowrap">
      <Sidebar
        system={system}
        user={user}
        selected={selected}
        userUsageCounts={userUsageCounts}
        onSelect={(n, isSystem) =>
          setSelected(`${isSystem ? 'system' : 'user'}:${n}`)
        }
        onAdd={handleAdd}
        onProvision={handleProvision}
        onRequestRemove={requestRemove}
      />
      {selectedSystem && (
        <Card withBorder padding="md" style={{ flex: 1, minWidth: 320 }}>
          <Stack gap="md">
            <Group justify="space-between">
              <Title order={5}>{selectedSystem.name}</Title>
              {chipFor(selectedSystem.kind)}
            </Group>
            <Text size="sm" c="dimmed">
              {selectedSystem.description}
            </Text>
            <Text size="sm" c="dimmed">
              System variables are auto-provisioned at run time and cannot be
              edited from the Builder.
            </Text>
          </Stack>
        </Card>
      )}
      {selectedUser && (
        <DefinitionPane
          variable={selectedUser}
          onChange={handleChange}
        />
      )}
      {!selectedSystem && !selectedUser && (
        <Card withBorder padding="md" style={{ flex: 1, minWidth: 320 }}>
          <Text c="dimmed">
            Select a variable on the left to see its definition and usage.
          </Text>
        </Card>
      )}
      <UsagePane refs={usages} onJump={handleJump} />

      <Modal
        opened={Boolean(pendingDelete)}
        onClose={() => setPendingDelete(null)}
        title={
          pendingDelete && pendingDelete.usages.length > 0
            ? 'Cannot remove variable'
            : 'Remove variable'
        }
        centered
      >
        {pendingDelete && pendingDelete.usages.length > 0 ? (
          <Stack gap="md">
            <Text size="sm">
              <strong>{pendingDelete.name}</strong> is referenced in{' '}
              {pendingDelete.usages.length}{' '}
              {pendingDelete.usages.length === 1 ? 'place' : 'places'}.
              Remove or replace those references first, then delete.
            </Text>
            <Stack gap={4}>
              {pendingDelete.usages.map((u, i) => (
                <Text key={i} size="sm" c="dimmed">
                  • {u.label}
                </Text>
              ))}
            </Stack>
            <Group justify="flex-end">
              <Button onClick={() => setPendingDelete(null)}>OK</Button>
            </Group>
          </Stack>
        ) : (
          <Stack gap="md">
            <Text size="sm">
              Remove <strong>{pendingDelete?.name ?? ''}</strong>? This
              cannot be undone outside of Discard.
            </Text>
            <Group justify="flex-end">
              <Button variant="subtle" onClick={() => setPendingDelete(null)}>
                Cancel
              </Button>
              <Button
                color="red"
                onClick={confirmDelete}
                data-testid="vars-remove-confirm"
              >
                Remove
              </Button>
            </Group>
          </Stack>
        )}
      </Modal>
    </Group>
  );
}
