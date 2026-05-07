/**
 * Builder header — title row (editable scenario name + dirty badge +
 * Undo/Redo) plus the form fields (vendor, service type, session mode,
 * service model, description).
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

import { usePeers } from '../../api/resources/peers';
import { useSubscribers } from '../../api/resources/subscribers';
import { useScenarioDraftStore } from './scenarioDraftStore';
import type {
  ServiceProfile,
  ServiceType,
  SessionMode,
} from './types';
import { DebouncedTextarea } from './DebouncedTextInput';

const VENDOR_OPTIONS: { value: ServiceProfile; label: string }[] = [
  { value: '3GPP', label: '3GPP' },
  { value: 'HUAWEI', label: 'Huawei' },
];

const SERVICE_TYPE_OPTIONS: { value: ServiceType; label: string }[] = [
  { value: 'VOICE', label: 'Voice' },
  { value: 'DATA', label: 'Data' },
  { value: 'SMS', label: 'SMS' },
  { value: 'USSD1_EVENT', label: 'USSD1 Event' },
  { value: 'USSD1_SESSION', label: 'USSD1 Session' },
  { value: 'USSD2_SESSION', label: 'USSD2 Session' },
];

const SESSION_OPTIONS: { value: SessionMode; label: string }[] = [
  { value: 'session', label: 'Session' },
  { value: 'event', label: 'Event' },
];

interface BuilderHeaderProps {
  isNew: boolean;
  isDirty: boolean;
}

export function BuilderHeader({ isNew, isDirty }: BuilderHeaderProps) {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setName = useScenarioDraftStore((s) => s.setName);
  const setDescription = useScenarioDraftStore((s) => s.setDescription);
  const setServiceType = useScenarioDraftStore((s) => s.setServiceType);
  const setServiceProfile = useScenarioDraftStore((s) => s.setServiceProfile);
  const setSessionMode = useScenarioDraftStore((s) => s.setSessionMode);
  const setPeerId = useScenarioDraftStore((s) => s.setPeerId);
  const setSubscriberId = useScenarioDraftStore((s) => s.setSubscriberId);
  const setServiceContextId = useScenarioDraftStore((s) => s.setServiceContextId);
  const undo = useScenarioDraftStore((s) => s.undo);
  const redo = useScenarioDraftStore((s) => s.redo);
  const canUndo = useScenarioDraftStore((s) => s.canUndo());
  const canRedo = useScenarioDraftStore((s) => s.canRedo());

  const peers = usePeers();
  const subscribers = useSubscribers();

  const peerOptions = (peers.data ?? []).map((p) => ({ value: p.id, label: p.name }));
  const subscriberOptions = (subscribers.data ?? []).map((s) => ({ value: s.id, label: s.msisdn }));

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
            label="Vendor"
            data={VENDOR_OPTIONS}
            value={draft.serviceProfile ?? '3GPP'}
            onChange={(v) => v && setServiceProfile(v as ServiceProfile)}
            allowDeselect={false}
            data-testid="builder-service-profile"
          />
        </Grid.Col>
        <Grid.Col span={{ base: 3, sm: 1 }}>
          <Select
            label="Service type"
            data={SERVICE_TYPE_OPTIONS}
            value={draft.serviceType ?? null}
            onChange={(v) => v && setServiceType(v as ServiceType)}
            allowDeselect={false}
            data-testid="builder-service-type"
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
            label="Peer"
            placeholder="Select a peer"
            data={peerOptions}
            value={draft.peerId || null}
            onChange={(v) => setPeerId(v ?? '')}
            clearable
            searchable
            withAsterisk
            disabled={peers.isLoading}
            data-testid="builder-peer"
          />
        </Grid.Col>
        <Grid.Col span={{ base: 3, sm: 1 }}>
          <Select
            label="Subscriber"
            placeholder="Select a subscriber"
            data={subscriberOptions}
            value={draft.subscriberId || null}
            onChange={(v) => setSubscriberId(v ?? '')}
            clearable
            searchable
            withAsterisk
            disabled={subscribers.isLoading}
            data-testid="builder-subscriber"
          />
        </Grid.Col>
        <Grid.Col span={{ base: 3, sm: 1 }}>
          <TextInput
            label="Service-Context-Id"
            placeholder="32251@3gpp.org"
            value={draft.serviceContextId ?? ''}
            onChange={(e) => setServiceContextId(e.currentTarget.value || undefined)}
            data-testid="builder-service-context-id"
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
