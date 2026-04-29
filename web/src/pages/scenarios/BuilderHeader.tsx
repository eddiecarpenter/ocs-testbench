/**
 * Builder header — title row (editable scenario name + dirty badge +
 * Undo/Redo) plus the form fields (unit type, session mode, service
 * model, description).
 *
 * The scenario name lives in the title itself: click the pencil to
 * switch the title into an edit input; Enter or blur commits, Esc
 * cancels. While editing, the input takes the full width of the
 * title column (badges hide briefly so the input has room).
 *
 * The session-terminating actions (Save / Discard / Delete) live in
 * `BuilderFooter`, sticky at the bottom of the modal body.
 */
import {
  ActionIcon,
  Badge,
  Grid,
  Group,
  Select,
  Stack,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core';
import {
  IconArrowBackUp,
  IconArrowForwardUp,
  IconPencil,
} from '@tabler/icons-react';
import { useEffect, useRef, useState } from 'react';

import { useScenarioDraftStore } from './scenarioDraftStore';
import type {
  ServiceModel,
  SessionMode,
  UnitType,
} from './types';
import { DebouncedTextarea } from './DebouncedTextInput';

const UNIT_OPTIONS: { value: UnitType; label: string }[] = [
  { value: 'OCTET', label: 'Octet' },
  { value: 'TIME', label: 'Time' },
  { value: 'UNITS', label: 'Units' },
];

const SESSION_OPTIONS: { value: SessionMode; label: string }[] = [
  { value: 'session', label: 'Session' },
  { value: 'event', label: 'Event' },
];

const SERVICE_MODEL_OPTIONS: { value: ServiceModel; label: string }[] = [
  { value: 'root', label: 'Root' },
  { value: 'single-mscc', label: 'Single MSCC' },
  { value: 'multi-mscc', label: 'Multi MSCC' },
];

interface BuilderHeaderProps {
  isNew: boolean;
  isDirty: boolean;
}

export function BuilderHeader({ isNew, isDirty }: BuilderHeaderProps) {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setName = useScenarioDraftStore((s) => s.setName);
  const setDescription = useScenarioDraftStore((s) => s.setDescription);
  const setUnitType = useScenarioDraftStore((s) => s.setUnitType);
  const setSessionMode = useScenarioDraftStore((s) => s.setSessionMode);
  const setServiceModel = useScenarioDraftStore((s) => s.setServiceModel);
  const undo = useScenarioDraftStore((s) => s.undo);
  const redo = useScenarioDraftStore((s) => s.redo);
  const canUndo = useScenarioDraftStore((s) => s.canUndo());
  const canRedo = useScenarioDraftStore((s) => s.canRedo());

  const [editingName, setEditingName] = useState(false);

  if (!draft) return null;

  const displayName =
    isNew && !draft.name
      ? 'New scenario'
      : draft.name || 'Untitled scenario';

  return (
    <Stack gap="md">
      <Group justify="space-between" align="flex-start" wrap="nowrap">
        {/*
         * The title column owns ALL remaining width up to the
         * Undo/Redo cluster on the right. When editing, the
         * NameInput renders as the only direct child of this
         * flex-1 Stack, so it stretches naturally to fill it.
         */}
        <Stack gap={4} style={{ flex: 1, minWidth: 0 }}>
          {editingName ? (
            <NameInput
              initialValue={draft.name}
              onCommit={(next) => {
                if (next !== draft.name) setName(next);
                setEditingName(false);
              }}
              onCancel={() => setEditingName(false)}
            />
          ) : (
            <Group gap="xs" align="center" wrap="wrap">
              <Title order={2}>{displayName}</Title>
              <Tooltip label="Edit name">
                <ActionIcon
                  variant="subtle"
                  onClick={() => setEditingName(true)}
                  aria-label="Edit name"
                  data-testid="builder-name-edit"
                >
                  <IconPencil size={14} />
                </ActionIcon>
              </Tooltip>
              {isDirty && (
                <Badge color="yellow" variant="light">
                  Unsaved
                </Badge>
              )}
              {draft.origin === 'system' && (
                <Badge variant="outline">System</Badge>
              )}
            </Group>
          )}
        </Stack>
        <Group gap="xs" wrap="nowrap">
          <Tooltip label="Undo">
            <ActionIcon
              variant="default"
              onClick={undo}
              disabled={!canUndo}
              aria-label="Undo"
              data-testid="builder-undo"
            >
              <IconArrowBackUp size={16} />
            </ActionIcon>
          </Tooltip>
          <Tooltip label="Redo">
            <ActionIcon
              variant="default"
              onClick={redo}
              disabled={!canRedo}
              aria-label="Redo"
              data-testid="builder-redo"
            >
              <IconArrowForwardUp size={16} />
            </ActionIcon>
          </Tooltip>
        </Group>
      </Group>

      <Grid columns={3}>
        <Grid.Col span={{ base: 3, sm: 1 }}>
          <Select
            label="Unit type"
            data={UNIT_OPTIONS}
            value={draft.unitType}
            onChange={(v) => v && setUnitType(v as UnitType)}
            allowDeselect={false}
            data-testid="builder-unit-type"
          />
        </Grid.Col>
        <Grid.Col span={{ base: 3, sm: 1 }}>
          <Select
            label="Session mode"
            data={SESSION_OPTIONS}
            value={draft.sessionMode}
            onChange={(v) => v && setSessionMode(v as SessionMode)}
            allowDeselect={false}
            data-testid="builder-session-mode"
          />
        </Grid.Col>
        <Grid.Col span={{ base: 3, sm: 1 }}>
          <Select
            label="Service model"
            data={SERVICE_MODEL_OPTIONS}
            value={draft.serviceModel}
            onChange={(v) => v && setServiceModel(v as ServiceModel)}
            allowDeselect={false}
            data-testid="builder-service-model"
          />
        </Grid.Col>
      </Grid>

      <DebouncedTextarea
        label="Description"
        minRows={2}
        maxRows={4}
        value={draft.description ?? ''}
        onCommit={setDescription}
        data-testid="builder-description"
      />
    </Stack>
  );
}

/**
 * Heading-styled inline name input. Lives only while the user is
 * editing; the parent re-mounts it on each enter-edit so we don't
 * carry state between sessions. Empty trimmed input on commit
 * reverts to `initialValue`.
 */
function NameInput({
  initialValue,
  onCommit,
  onCancel,
}: {
  initialValue: string;
  onCommit: (next: string) => void;
  onCancel: () => void;
}) {
  const [local, setLocal] = useState(initialValue);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, []);

  return (
    <TextInput
      ref={inputRef}
      value={local}
      onChange={(e) => setLocal(e.currentTarget.value)}
      onBlur={() => {
        const trimmed = local.trim();
        onCommit(trimmed === '' ? initialValue : local);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.currentTarget.blur();
        } else if (e.key === 'Escape') {
          onCancel();
        }
      }}
      variant="unstyled"
      size="xl"
      styles={{
        input: {
          fontSize: 'var(--mantine-h2-font-size)',
          fontWeight: 700,
          lineHeight: 'var(--mantine-h2-line-height)',
          padding: 0,
          height: 'auto',
          minHeight: 0,
        },
      }}
      data-testid="builder-name"
    />
  );
}
