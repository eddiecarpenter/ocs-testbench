import {
  Button,
  Divider,
  Group,
  NumberInput,
  SegmentedControl,
  Stack,
  Switch,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { useEffect } from 'react';

import { ApiError } from '../../api/errors';
import type { Peer, PeerInput } from '../../api/resources/peers';

interface PeerFormProps {
  /** When provided, the form pre-fills with this peer's fields (edit mode). */
  initial?: Peer;
  mode: 'create' | 'edit';
  submitting?: boolean;
  testing?: boolean;
  deleting?: boolean;
  onSubmit: (values: PeerInput) => Promise<Peer | void>;
  onTest?: (values: PeerInput) => void;
  onDelete?: () => void;
  onCancel: () => void;
}

const EMPTY: PeerInput = {
  name: '',
  host: '',
  port: 3868,
  originHost: '',
  originRealm: '',
  transport: 'TCP',
  watchdogIntervalSeconds: 30,
  autoConnect: true,
};

function fromPeer(peer: Peer): PeerInput {
  return {
    name: peer.name,
    host: peer.host,
    port: peer.port,
    originHost: peer.originHost,
    originRealm: peer.originRealm,
    transport: peer.transport,
    watchdogIntervalSeconds: peer.watchdogIntervalSeconds,
    autoConnect: peer.autoConnect,
  };
}

/** Sectioned heading (IDENTITY / CONNECTION / BEHAVIOUR) from the Figma. */
function SectionLabel({ children }: { children: string }) {
  return (
    <Text
      size="xs"
      fw={600}
      c="dimmed"
      tt="uppercase"
      style={{ letterSpacing: 0.5 }}
    >
      {children}
    </Text>
  );
}

/**
 * Peer create/edit form used inside the right-hand Drawer. Handles
 * client-side validation and routes 422 responses onto the right fields
 * via `form.setErrors()`.
 */
export function PeerForm({
  initial,
  mode,
  submitting,
  testing,
  deleting,
  onSubmit,
  onTest,
  onDelete,
  onCancel,
}: PeerFormProps) {
  // Controlled mode (default) — we rely on reactive re-render of
  // `form.isValid()` to enable/disable the Test button as the user types.
  const form = useForm<PeerInput>({
    initialValues: initial ? fromPeer(initial) : EMPTY,
    validateInputOnChange: true,
    validate: {
      name: (v) => (v.trim() ? null : 'Name is required'),
      host: (v) => (v.trim() ? null : 'Host is required'),
      port: (v) =>
        typeof v === 'number' && v >= 1 && v <= 65535
          ? null
          : 'Port must be between 1 and 65535',
      originHost: (v) => (v.trim() ? null : 'Origin-Host is required'),
      originRealm: (v) => (v.trim() ? null : 'Origin-Realm is required'),
      watchdogIntervalSeconds: (v) =>
        typeof v === 'number' && v >= 5 && v <= 3600
          ? null
          : 'Watchdog interval must be between 5 and 3600 seconds',
    },
  });

  // Reset form whenever we get a different peer to edit (drawer reuse).
  useEffect(() => {
    form.setValues(initial ? fromPeer(initial) : EMPTY);
    form.resetDirty();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial?.id]);

  const handleSubmit = form.onSubmit(async (values) => {
    try {
      await onSubmit(values);
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        const fieldErrors = err.fieldErrors();
        if (Object.keys(fieldErrors).length > 0) {
          form.setErrors(fieldErrors);
          return;
        }
      }
      throw err;
    }
  });

  const canTest = form.isValid();
  const handleTest = () => {
    if (!onTest || !canTest) return;
    onTest(form.getValues());
  };

  return (
    <form
      onSubmit={handleSubmit}
      style={{
        display: 'flex',
        flexDirection: 'column',
        flex: 1,
        minHeight: 0,
      }}
    >
      {/* Header */}
      <Stack gap={4} pb="md">
        <Text
          size="xs"
          fw={600}
          c="dimmed"
          tt="uppercase"
          style={{ letterSpacing: 0.5 }}
        >
          {mode === 'edit' ? 'Edit peer' : 'Add peer'}
        </Text>
        <Title order={3} fw={600}>
          {mode === 'edit' ? (initial?.name ?? 'Peer') : 'New peer'}
        </Title>
      </Stack>

      {/* Scrollable body */}
      <Stack gap="lg" style={{ flex: 1, overflowY: 'auto' }} pr="xs">
        {/* IDENTITY */}
        <Stack gap="sm">
          <SectionLabel>Identity</SectionLabel>
          <TextInput
            label="Name"
            placeholder="peer-06"
            required
            key={form.key('name')}
            {...form.getInputProps('name')}
          />
          <TextInput
            label="Origin-Host"
            placeholder="ctf-06.test.local"
            required
            key={form.key('originHost')}
            {...form.getInputProps('originHost')}
          />
          <TextInput
            label="Origin-Realm"
            placeholder="test.local"
            required
            key={form.key('originRealm')}
            {...form.getInputProps('originRealm')}
          />
        </Stack>

        <Divider />

        {/* CONNECTION */}
        <Stack gap="sm">
          <SectionLabel>Connection</SectionLabel>
          <Group grow align="flex-start">
            <TextInput
              label="Host"
              placeholder="10.0.1.5"
              required
              key={form.key('host')}
              {...form.getInputProps('host')}
            />
            <NumberInput
              label="Port"
              placeholder="3868"
              min={1}
              max={65535}
              clampBehavior="strict"
              required
              hideControls
              key={form.key('port')}
              {...form.getInputProps('port')}
            />
          </Group>
          <div>
            <Text size="sm" fw={500} mb={4}>
              Transport
            </Text>
            <SegmentedControl
              fullWidth
              data={[
                { label: 'TCP', value: 'TCP' },
                { label: 'TLS', value: 'TLS' },
              ]}
              key={form.key('transport')}
              {...form.getInputProps('transport')}
            />
          </div>
          <NumberInput
            label="Watchdog interval"
            suffix=" seconds"
            min={5}
            max={3600}
            clampBehavior="strict"
            required
            key={form.key('watchdogIntervalSeconds')}
            {...form.getInputProps('watchdogIntervalSeconds')}
          />
        </Stack>

        <Divider />

        {/* BEHAVIOUR */}
        <Stack gap="sm">
          <SectionLabel>Behaviour</SectionLabel>
          <Group justify="space-between" align="flex-start" wrap="nowrap">
            <Stack gap={2}>
              <Text size="sm" fw={500}>
                Auto-connect on startup
              </Text>
              <Text size="xs" c="dimmed">
                Automatically connect this peer when the application starts
              </Text>
            </Stack>
            <Switch
              key={form.key('autoConnect')}
              {...form.getInputProps('autoConnect', { type: 'checkbox' })}
            />
          </Group>
        </Stack>
      </Stack>

      {/* Footer */}
      <Group justify="space-between" pt="md" mt="md" style={{ borderTop: '1px solid var(--mantine-color-gray-3)' }}>
        {mode === 'edit' && onDelete ? (
          <Button
            variant="subtle"
            color="red"
            onClick={onDelete}
            loading={deleting}
            disabled={submitting || testing}
          >
            Delete
          </Button>
        ) : (
          <span />
        )}
        <Group gap="xs">
          <Button variant="default" onClick={onCancel} disabled={submitting}>
            Cancel
          </Button>
          {onTest && (
            <Button
              variant="default"
              onClick={handleTest}
              loading={testing}
              disabled={!canTest || submitting || deleting}
            >
              Test
            </Button>
          )}
          <Button type="submit" loading={submitting} disabled={testing || deleting}>
            {mode === 'edit' ? 'Update' : 'Create'}
          </Button>
        </Group>
      </Group>
    </form>
  );
}
