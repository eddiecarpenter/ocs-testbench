/**
 * Builder — Frame tab.
 *
 * Two-pane layout: recursive AVP-tree on the left, properties on the right.
 *
 * Three tiers of AVP node:
 *   engine-only  — injected at send time, NOT stored in avpTree, shown as
 *                  informational rows above the tree (Session-Id).
 *   locked leaf  — stored in avpTree, locked=true, no remove button, but
 *                  valueRef IS editable (Origin-Host, Node-Functionality …).
 *   locked group — stored in avpTree, locked=true, expand/collapse only;
 *                  its children's values can still be edited.
 *   user AVP     — fully editable + removable.
 *
 * Value reference on a leaf AVP is a free-text Autocomplete: the user may
 * type a literal (e.g. "4", "32251@3gpp.org") or pick a variable name
 * from the suggestion list.
 */
import {
  ActionIcon,
  Autocomplete,
  Badge,
  Button,
  Card,
  Divider,
  Group,
  Modal,
  NumberInput,
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
  isLockedAvp,
  isLockedGroup,
  removeNodeAt,
  setNodeAt,
} from './avpTree';

function pathKey(path: AvpPath): string {
  return path.join('.');
}

/**
 * Read-only informational row for AVPs that are 100% engine-managed and
 * not stored in avpTree at all (e.g. Session-Id — always auto-generated).
 */
function EngineOnlyRow({ code, name, hint }: { code: number; name: string; hint: string }) {
  return (
    <Group
      gap={6}
      wrap="nowrap"
      style={{ opacity: 0.55, padding: '4px 6px', cursor: 'not-allowed' }}
    >
      <IconCircleDot size={12} style={{ opacity: 0.4 }} />
      <Badge variant="outline" size="xs">{code}</Badge>
      <Text style={{ flex: 1 }} fw={400}>{name}</Text>
      <Badge color="gray" variant="light" size="xs" leftSection={<IconLock size={10} />}>
        engine
      </Badge>
      <Text size="xs" c="dimmed">{hint}</Text>
    </Group>
  );
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
  const locked = isLockedAvp(node);
  const lockedGroup = isLockedGroup(node);
  const isGrouped = Array.isArray(node.children);
  const isOpen = expanded.has(key);
  const isSelected = selectedKey === key;

  // Locked groups are expand/collapse only — not selectable for editing.
  const selectable = !lockedGroup;

  return (
    <Stack gap={2} pl={path.length * 12}>
      <Group
        gap={6}
        wrap="nowrap"
        style={{
          backgroundColor: isSelected ? 'var(--mantine-color-blue-light)' : undefined,
          padding: '4px 6px',
          borderRadius: 4,
          cursor: selectable ? 'pointer' : 'default',
        }}
        onClick={() => {
          if (isGrouped) {
            onToggle(key);
          } else if (selectable) {
            onSelect(path);
          }
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
        <Text style={{ flex: 1 }} fw={500}>
          {node.name}
        </Text>
        {locked && (
          <Badge
            color="blue"
            variant="light"
            size="xs"
            leftSection={<IconLock size={10} />}
          >
            engine
          </Badge>
        )}
        {!isGrouped && node.valueRef && (
          <Text size="xs" c="dimmed">
            ← {/^\d/.test(node.valueRef) ? node.valueRef : `{{${node.valueRef}}}`}
          </Text>
        )}
        {!locked && (
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
  onChange: (replacement: AvpNode) => void;
  onAddChild: () => void;
}

function PropertiesPane({
  node,
  variableOptions,
  onChange,
  onAddChild,
}: PropertiesPaneProps) {
  const locked = isLockedAvp(node);
  const isGrouped = Array.isArray(node.children);

  // Flatten variable groups into a simple string list for Autocomplete.
  const variableNames: string[] = variableOptions.flatMap((g) =>
    g.items.map((item) => item.value),
  );

  return (
    <Stack gap="md">
      <TextInput
        label="Name"
        value={node.name}
        disabled={locked}
        onChange={(e) => onChange({ ...node, name: e.currentTarget.value })}
      />
      <NumberInput
        label="AVP code"
        min={0}
        value={node.code}
        disabled={locked}
        onChange={(v) =>
          onChange({ ...node, code: typeof v === 'number' ? v : 0 })
        }
      />
      <NumberInput
        label="Vendor-Id (optional)"
        min={0}
        value={node.vendorId ?? ''}
        disabled={locked}
        onChange={(v) =>
          onChange({
            ...node,
            vendorId: typeof v === 'number' ? v : undefined,
          })
        }
      />
      {!isGrouped && (
        <Autocomplete
          label="Value reference"
          description={
            locked
              ? 'Type a literal value (e.g. "4", "32251@3gpp.org") or a variable name from the list below.'
              : 'Type a literal value or select a variable name — wraps to {{NAME}} in the wire frame.'
          }
          placeholder='e.g. "4" or MSISDN'
          data={variableNames}
          value={node.valueRef ?? ''}
          onChange={(v) => onChange({ ...node, valueRef: v })}
          clearable
          data-testid="avp-value-ref"
        />
      )}
      {isGrouped && !locked && (
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
      {isGrouped && locked && (
        <Text size="sm" c="dimmed">
          This is a mandatory grouped AVP — its structure is fixed. Select a
          child leaf to change its value reference.
        </Text>
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
  const { options: variableOptions } =
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

  // A locked-group node is not selectable; a locked-leaf IS selectable.
  const canShowProperties = selectedNode !== null && !isLockedGroup(selectedNode);

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
              <EngineOnlyRow code={263} name="Session-Id"          hint="Auto-generated per RFC 6733 §8.8" />
              <EngineOnlyRow code={258} name="Auth-Application-Id" hint="4 (Credit-Control / Gy)" />
              <Divider my={4} />
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
          {canShowProperties ? (
            <PropertiesPane
              node={selectedNode!}
              variableOptions={variableOptions}
              onChange={handleChange}
              onAddChild={handleAddChild}
            />
          ) : (
            <Text c="dimmed">
              {selectedNode && isLockedGroup(selectedNode)
                ? 'This is a mandatory grouped AVP. Select a child leaf AVP to edit its value reference.'
                : 'Select an AVP to edit its properties.'}
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
