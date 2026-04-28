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
  Card,
  Group,
  NumberInput,
  Select,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import { IconArrowRight, IconPlus, IconTrash, IconWand } from '@tabler/icons-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { provisionVariables } from '../../store/provisionVariables';
import { useScenarioDraftStore } from '../../store/scenarioDraftStore';
import {
  type SystemVariable,
  type UsageRef,
  findUsages,
  listSystemVariables,
} from '../../store/selectors';
import type {
  GeneratorRefresh,
  GeneratorStrategy,
  Variable,
  VariableSource,
  VariableSourceBound,
  VariableSourceExtracted,
  VariableSourceGenerator,
} from '../../store/types';

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
  onSelect: (name: string, isSystem: boolean) => void;
  onAdd: () => void;
  onProvision: () => void;
}

function Sidebar({
  system,
  user,
  selected,
  onSelect,
  onAdd,
  onProvision,
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
            <ActionIcon
              variant="subtle"
              size="sm"
              aria-label="Auto-provision missing variables"
              onClick={onProvision}
              data-testid="vars-provision"
            >
              <IconWand size={14} />
            </ActionIcon>
            <ActionIcon
              variant="filled"
              size="sm"
              aria-label="Add variable"
              onClick={onAdd}
              data-testid="vars-add"
            >
              <IconPlus size={14} />
            </ActionIcon>
          </Group>
        </Group>
        <Stack gap={2}>
          {user.length === 0 && (
            <Text size="sm" c="dimmed">
              No user variables yet.
            </Text>
          )}
          {user.map((v) => (
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
              <Text size="sm">{v.name}</Text>
            </Group>
          ))}
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
  onRemove: () => void;
}

function DefinitionPane({
  variable,
  onChange,
  onRemove,
}: DefinitionPaneProps) {
  return (
    <Card withBorder padding="md" style={{ flex: 1, minWidth: 320 }}>
      <Stack gap="md">
        <Group justify="space-between">
          <Title order={5}>{variable.name}</Title>
          <ActionIcon
            variant="subtle"
            color="red"
            onClick={onRemove}
            aria-label="Remove variable"
          >
            <IconTrash size={14} />
          </ActionIcon>
        </Group>
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
  const [, setParams] = useSearchParams();

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
    const next = user.map((v) => (v.name === selectedUser.name ? updated : v));
    setVariables(next);
    if (updated.name !== selectedUser.name) {
      setSelected(`user:${updated.name}`);
    }
  }

  function handleRemove() {
    if (!selectedUser) return;
    const next = user.filter((v) => v.name !== selectedUser.name);
    setVariables(next);
    setSelected(next.length > 0 ? `user:${next[0].name}` : '');
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

  return (
    <Group align="flex-start" gap="lg" wrap="nowrap">
      <Sidebar
        system={system}
        user={user}
        selected={selected}
        onSelect={(n, isSystem) =>
          setSelected(`${isSystem ? 'system' : 'user'}:${n}`)
        }
        onAdd={handleAdd}
        onProvision={handleProvision}
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
          onRemove={handleRemove}
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
    </Group>
  );
}
