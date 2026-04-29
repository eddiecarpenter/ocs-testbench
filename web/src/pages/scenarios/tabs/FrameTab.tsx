/**
 * Builder — Frame tab.
 *
 * Two-pane layout: a custom recursive AVP-tree view on the left
 * (engine-managed AVPs render dimmed and are not selectable for edit
 * — they remain visible to keep the §8 frame structure honest), and
 * a properties pane on the right scoped to the selected node.
 *
 * Per-row Remove sits inline in the tree (no Remove button in the
 * properties pane); deleting a non-leaf prompts a confirmation
 * modal because it cascades into all descendants.
 *
 * Value reference on a leaf AVP is a Select of variable names
 * defined on the Variables tab — so the frame stays in lock-step
 * with the variable catalogue.
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
import {
  IconChevronDown,
  IconChevronRight,
  IconCircleDot,
  IconLock,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import { useState } from 'react';

import { useScenarioDraftStore } from '../scenarioDraftStore';
import {
  buildVariableOptions,
  type VariableOptionGroup,
} from '../selectors';
import type { AvpNode } from '../types';
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

/** Sum of node + all descendants — drives the delete-confirm copy. */
function countNodes(node: AvpNode): number {
  if (!node.children || node.children.length === 0) return 1;
  return 1 + node.children.reduce((acc, c) => acc + countNodes(c), 0);
}

interface AvpRowProps {
  node: AvpNode;
  path: AvpPath;
  selectedKey: string;
  expanded: Set<string>;
  onToggle: (key: string) => void;
  onSelect: (path: AvpPath) => void;
  onRequestRemove: (path: AvpPath) => void;
}

function AvpRow({
  node,
  path,
  selectedKey,
  expanded,
  onToggle,
  onSelect,
  onRequestRemove,
}: AvpRowProps) {
  const key = pathKey(path);
  const managed = isManagedAvp(node);
  const isGrouped = Array.isArray(node.children);
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
        <Badge variant="outline" size="xs">
          {node.code}
        </Badge>
        <Text style={{ flex: 1 }} fw={managed ? 400 : 500}>
          {node.name}
        </Text>
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
            ← {`{{${node.valueRef}}}`}
          </Text>
        )}
        {!managed && (
          <Tooltip label="Remove">
            <ActionIcon
              variant="subtle"
              color="red"
              size="sm"
              aria-label="Remove AVP"
              onClick={(e) => {
                e.stopPropagation();
                onRequestRemove(path);
              }}
              data-testid={`avp-remove-${key}`}
            >
              <IconTrash size={14} />
            </ActionIcon>
          </Tooltip>
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
          onRequestRemove={onRequestRemove}
        />
      ))}
    </Stack>
  );
}

interface PropertiesPaneProps {
  node: AvpNode;
  variableOptions: VariableOptionGroup[];
  hasAnyVariable: boolean;
  onChange: (replacement: AvpNode) => void;
  onAddChild: () => void;
}

function PropertiesPane({
  node,
  variableOptions,
  hasAnyVariable,
  onChange,
  onAddChild,
}: PropertiesPaneProps) {
  const isGrouped = Array.isArray(node.children);

  return (
    <Stack gap="md">
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
        <Select
          label="Value reference"
          description="Variable bound to this AVP — wraps to {{NAME}} in the wire frame. Includes engine-provided System variables and any User variables you define."
          placeholder={
            !hasAnyVariable ? 'No variables available' : '{{ ... }}'
          }
          data={variableOptions}
          value={node.valueRef ?? null}
          onChange={(v) =>
            onChange({ ...node, valueRef: v ?? '' })
          }
          clearable
          disabled={!hasAnyVariable}
          data-testid="avp-value-ref"
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
  const [pendingRemove, setPendingRemove] = useState<AvpPath | null>(null);

  if (!draft) return null;
  const tree = draft.avpTree;
  const { options: variableOptions, hasAny: hasAnyVariable } =
    buildVariableOptions(draft.variables);

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

  function requestRemove(path: AvpPath) {
    setPendingRemove(path);
  }

  function confirmRemove() {
    if (!pendingRemove) return;
    setAvpTree(removeNodeAt(tree, pendingRemove));
    if (selected && pathKey(selected).startsWith(pathKey(pendingRemove))) {
      setSelected(null);
    }
    setPendingRemove(null);
  }

  const pendingNode = pendingRemove ? getNodeAt(tree, pendingRemove) : null;
  const pendingDescendantCount = pendingNode ? countNodes(pendingNode) - 1 : 0;

  return (
    <>
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
                  onRequestRemove={requestRemove}
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
              variableOptions={variableOptions}
              hasAnyVariable={hasAnyVariable}
              onChange={handleChange}
              onAddChild={handleAddChild}
            />
          ) : (
            <Text c="dimmed">
              Select a non-managed AVP to edit its properties. Engine-managed
              AVPs are read-only because the runtime owns their value.
            </Text>
          )}
        </Card>
      </Group>

      <Modal
        opened={Boolean(pendingRemove)}
        onClose={() => setPendingRemove(null)}
        title="Remove AVP"
        centered
      >
        <Stack gap="md">
          <Text size="sm">
            Remove <strong>{pendingNode?.name ?? ''}</strong>
            {pendingDescendantCount > 0 && (
              <>
                {' '}and its {pendingDescendantCount}{' '}
                {pendingDescendantCount === 1 ? 'descendant' : 'descendants'}
              </>
            )}
            ? This cannot be undone outside of Discard.
          </Text>
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setPendingRemove(null)}>
              Cancel
            </Button>
            <Button
              color="red"
              onClick={confirmRemove}
              data-testid="avp-remove-confirm"
            >
              Remove
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
}
