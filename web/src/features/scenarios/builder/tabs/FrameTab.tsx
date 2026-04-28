/**
 * Builder — Frame tab.
 *
 * Two-pane layout: a custom recursive AVP-tree view on the left
 * (engine-managed AVPs render dimmed and are not selectable for edit
 * — they remain visible to keep the §8 frame structure honest), and
 * a properties pane on the right scoped to the selected node.
 */
import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Group,
  NumberInput,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import {
  IconChevronDown,
  IconChevronRight,
  IconCircleDot,
  IconLock,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import { useState } from 'react';

import { useScenarioDraftStore } from '../../store/scenarioDraftStore';
import type { AvpNode } from '../../store/types';
import {
  addChildAt,
  type AvpPath,
  getNodeAt,
  isManagedAvp,
  removeNodeAt,
  setNodeAt,
} from './avpTree';

function pathKey(path: AvpPath): string {
  return path.join('.');
}

interface AvpRowProps {
  node: AvpNode;
  path: AvpPath;
  selectedKey: string;
  expanded: Set<string>;
  onToggle: (key: string) => void;
  onSelect: (path: AvpPath) => void;
}

function AvpRow({
  node,
  path,
  selectedKey,
  expanded,
  onToggle,
  onSelect,
}: AvpRowProps) {
  const key = pathKey(path);
  const managed = isManagedAvp(node);
  const isGrouped = Boolean(node.children && node.children.length >= 0 && node.children !== undefined);
  const isOpen = expanded.has(key);
  const isSelected = selectedKey === key;

  return (
    <Stack gap={2} pl={path.length * 12}>
      <Group
        gap={6}
        wrap="nowrap"
        style={{
          backgroundColor: isSelected ? 'var(--mantine-color-blue-light)' : undefined,
          opacity: managed ? 0.55 : 1,
          padding: '4px 6px',
          borderRadius: 4,
          cursor: managed ? 'not-allowed' : 'pointer',
        }}
        onClick={() => {
          if (!managed) onSelect(path);
        }}
        data-testid={`avp-row-${key}`}
      >
        {isGrouped ? (
          <ActionIcon
            variant="transparent"
            size="xs"
            aria-label={isOpen ? 'Collapse' : 'Expand'}
            onClick={(e) => {
              e.stopPropagation();
              onToggle(key);
            }}
          >
            {isOpen ? (
              <IconChevronDown size={14} />
            ) : (
              <IconChevronRight size={14} />
            )}
          </ActionIcon>
        ) : (
          <IconCircleDot size={12} style={{ opacity: 0.4 }} />
        )}
        <Text style={{ flex: 1 }} fw={managed ? 400 : 500}>
          {node.name}
        </Text>
        <Badge variant="outline" size="xs">
          {node.code}
        </Badge>
        {managed && (
          <Badge
            color="gray"
            variant="light"
            size="xs"
            leftSection={<IconLock size={10} />}
          >
            engine-managed
          </Badge>
        )}
        {!isGrouped && node.valueRef && (
          <Text size="xs" c="dimmed">
            ← {node.valueRef}
          </Text>
        )}
      </Group>
      {isGrouped && isOpen && (node.children ?? []).map((child, i) => (
        <AvpRow
          key={`${key}.${i}`}
          node={child}
          path={[...path, i]}
          selectedKey={selectedKey}
          expanded={expanded}
          onToggle={onToggle}
          onSelect={onSelect}
        />
      ))}
    </Stack>
  );
}

interface PropertiesPaneProps {
  node: AvpNode;
  onChange: (replacement: AvpNode) => void;
  onAddChild: () => void;
  onRemove: () => void;
}

function PropertiesPane({
  node,
  onChange,
  onAddChild,
  onRemove,
}: PropertiesPaneProps) {
  const isGrouped = Array.isArray(node.children);

  return (
    <Stack gap="md">
      <Group justify="space-between">
        <Title order={5}>{node.name}</Title>
        <Button
          variant="subtle"
          color="red"
          leftSection={<IconTrash size={14} />}
          onClick={onRemove}
        >
          Remove
        </Button>
      </Group>
      <TextInput
        label="Name"
        value={node.name}
        onChange={(e) => onChange({ ...node, name: e.currentTarget.value })}
      />
      <NumberInput
        label="AVP code"
        min={0}
        value={node.code}
        onChange={(v) =>
          onChange({ ...node, code: typeof v === 'number' ? v : 0 })
        }
      />
      <NumberInput
        label="Vendor-Id (optional)"
        min={0}
        value={node.vendorId ?? ''}
        onChange={(v) =>
          onChange({
            ...node,
            vendorId: typeof v === 'number' ? v : undefined,
          })
        }
      />
      {!isGrouped && (
        <TextInput
          label="Value reference"
          description="Variable name (e.g. MSISDN) — wraps to {{NAME}} in the wire frame."
          value={node.valueRef ?? ''}
          onChange={(e) =>
            onChange({ ...node, valueRef: e.currentTarget.value })
          }
        />
      )}
      {isGrouped && (
        <Stack gap="xs">
          <Group justify="space-between">
            <Text size="sm" fw={500}>
              Children
            </Text>
            <Button
              variant="default"
              size="xs"
              leftSection={<IconPlus size={12} />}
              onClick={onAddChild}
              data-testid="avp-add-child"
            >
              Add child AVP
            </Button>
          </Group>
          {(node.children ?? []).length === 0 ? (
            <Text size="sm" c="dimmed">
              No children. Add one to start nesting.
            </Text>
          ) : (
            <Stack gap={2}>
              {(node.children ?? []).map((c, i) => (
                <Text key={i} size="sm">
                  • {c.name} ({c.code})
                </Text>
              ))}
            </Stack>
          )}
        </Stack>
      )}
    </Stack>
  );
}

export function FrameTab() {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setAvpTree = useScenarioDraftStore((s) => s.setAvpTree);

  const [selected, setSelected] = useState<AvpPath | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  if (!draft) return null;
  const tree = draft.avpTree;

  const selectedKey = selected ? pathKey(selected) : '';
  const selectedNode = selected ? getNodeAt(tree, selected) : null;

  function handleSelect(path: AvpPath) {
    setSelected(path);
  }

  function handleToggle(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function handleAddRoot() {
    const fresh: AvpNode = {
      name: 'New-Avp',
      code: 0,
      valueRef: '',
    };
    setAvpTree([...tree, fresh]);
  }

  function handleChange(node: AvpNode) {
    if (!selected) return;
    setAvpTree(setNodeAt(tree, selected, node));
  }

  function handleAddChild() {
    if (!selected) return;
    const fresh: AvpNode = { name: 'New-Avp', code: 0, valueRef: '' };
    setAvpTree(addChildAt(tree, selected, fresh));
    setExpanded((prev) => new Set(prev).add(selectedKey));
  }

  function handleRemove() {
    if (!selected) return;
    setAvpTree(removeNodeAt(tree, selected));
    setSelected(null);
  }

  return (
    <Group align="flex-start" gap="lg" wrap="nowrap">
      <Card withBorder padding="sm" style={{ flex: 1, minWidth: 0 }}>
        <Stack gap="xs">
          <Group justify="space-between">
            <Title order={5}>AVP frame</Title>
            <ActionIcon
              variant="filled"
              aria-label="Add root AVP"
              onClick={handleAddRoot}
              data-testid="avp-add-root"
            >
              <IconPlus size={16} />
            </ActionIcon>
          </Group>
          <Stack gap={2}>
            {tree.map((node, i) => (
              <AvpRow
                key={i}
                node={node}
                path={[i]}
                selectedKey={selectedKey}
                expanded={expanded}
                onToggle={handleToggle}
                onSelect={handleSelect}
              />
            ))}
          </Stack>
          {tree.length === 0 && (
            <Text c="dimmed" ta="center">
              No AVPs in the frame yet.
            </Text>
          )}
        </Stack>
      </Card>

      <Card withBorder padding="md" style={{ flex: 1, minWidth: 320 }}>
        {selectedNode && !isManagedAvp(selectedNode) ? (
          <PropertiesPane
            node={selectedNode}
            onChange={handleChange}
            onAddChild={handleAddChild}
            onRemove={handleRemove}
          />
        ) : (
          <Text c="dimmed">
            Select a non-managed AVP to edit its properties. Engine-managed
            AVPs are read-only because the runtime owns their value.
          </Text>
        )}
      </Card>
    </Group>
  );
}
