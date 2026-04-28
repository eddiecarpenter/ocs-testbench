/**
 * Builder header — name, unit type, session mode, service model,
 * description; and the right-hand action bar (Discard, Duplicate,
 * Save, Save & Run).
 *
 * Editing dispatchers come straight from the Zustand draft store;
 * mutation runs are owned by the parent route component and passed
 * down via callbacks (keeps the header free of TanStack Query
 * concerns).
 */
import {
  ActionIcon,
  Badge,
  Button,
  Group,
  Menu,
  Select,
  Stack,
  Title,
  Tooltip,
} from '@mantine/core';
import {
  IconArrowBackUp,
  IconArrowForwardUp,
  IconDots,
  IconDownload,
  IconUpload,
} from '@tabler/icons-react';

import { useScenarioDraftStore } from '../store/scenarioDraftStore';
import type {
  ServiceModel,
  SessionMode,
  UnitType,
} from '../store/types';
import { DebouncedTextInput, DebouncedTextarea } from './DebouncedTextInput';

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
  isSaving: boolean;
  onSave: () => void;
  onSaveAndRun: () => void;
  onDiscard: () => void;
  onDuplicate: () => void;
  onExport: () => void;
  onImport: () => void;
}

export function BuilderHeader({
  isNew,
  isDirty,
  isSaving,
  onSave,
  onSaveAndRun,
  onDiscard,
  onDuplicate,
  onExport,
  onImport,
}: BuilderHeaderProps) {
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

  if (!draft) return null;

  return (
    <Stack gap="md">
      <Group justify="space-between" align="flex-start">
        <Stack gap={4} style={{ flex: 1 }}>
          <Group gap="xs" align="center">
            <Title order={2}>
              {isNew ? 'New scenario' : draft.name || 'Untitled scenario'}
            </Title>
            {isDirty && (
              <Badge color="yellow" variant="light">
                Unsaved
              </Badge>
            )}
            {draft.origin === 'system' && (
              <Badge variant="outline">System</Badge>
            )}
          </Group>
        </Stack>
        <Group gap="xs">
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
          <Button
            variant="default"
            disabled={!isDirty}
            onClick={onDiscard}
            data-testid="builder-discard"
          >
            Discard
          </Button>
          <Button
            variant="default"
            onClick={onDuplicate}
            disabled={isNew}
            data-testid="builder-duplicate"
          >
            Duplicate
          </Button>
          <Menu position="bottom-end" withinPortal>
            <Menu.Target>
              <ActionIcon
                variant="default"
                aria-label="More actions"
                data-testid="builder-more"
              >
                <IconDots size={16} />
              </ActionIcon>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Item
                leftSection={<IconDownload size={14} />}
                onClick={onExport}
                disabled={isNew}
                data-testid="builder-export"
              >
                Export JSON
              </Menu.Item>
              <Menu.Item
                leftSection={<IconUpload size={14} />}
                onClick={onImport}
                data-testid="builder-import"
              >
                Import JSON
              </Menu.Item>
            </Menu.Dropdown>
          </Menu>
          <Button
            onClick={onSave}
            loading={isSaving}
            disabled={!isDirty && !isNew}
            data-testid="builder-save"
          >
            Save
          </Button>
          <Button
            color="green"
            onClick={onSaveAndRun}
            loading={isSaving}
            data-testid="builder-save-and-run"
          >
            Save &amp; Run
          </Button>
        </Group>
      </Group>

      <Group grow align="flex-start">
        <DebouncedTextInput
          label="Name"
          value={draft.name}
          onCommit={setName}
          data-testid="builder-name"
        />
        <Select
          label="Unit type"
          data={UNIT_OPTIONS}
          value={draft.unitType}
          onChange={(v) => v && setUnitType(v as UnitType)}
          allowDeselect={false}
          data-testid="builder-unit-type"
        />
        <Select
          label="Session mode"
          data={SESSION_OPTIONS}
          value={draft.sessionMode}
          onChange={(v) => v && setSessionMode(v as SessionMode)}
          allowDeselect={false}
          data-testid="builder-session-mode"
        />
        <Select
          label="Service model"
          data={SERVICE_MODEL_OPTIONS}
          value={draft.serviceModel}
          onChange={(v) => v && setServiceModel(v as ServiceModel)}
          allowDeselect={false}
          data-testid="builder-service-model"
        />
      </Group>

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
