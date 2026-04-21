import { Button, Group, Stack, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useEffect } from 'react';

import { ApiError } from '../../api/errors';
import type { Peer, PeerInput } from '../../api/resources/peers';

interface PeerFormProps {
  /** When provided, the form pre-fills with this peer's fields (edit mode). */
  initial?: Peer;
  submitLabel: string;
  submitting?: boolean;
  onSubmit: (values: PeerInput) => Promise<Peer | void>;
  onCancel: () => void;
}

const EMPTY: PeerInput = {
  name: '',
  endpoint: '',
  originHost: '',
};

/**
 * Create / edit form for a Peer. Handles client-side required-field checks
 * and surfaces server-side 422 validation errors returned by the API onto
 * the right fields via `form.setErrors()`.
 */
export function PeerForm({
  initial,
  submitLabel,
  submitting,
  onSubmit,
  onCancel,
}: PeerFormProps) {
  const form = useForm<PeerInput>({
    mode: 'uncontrolled',
    initialValues: initial
      ? {
          name: initial.name,
          endpoint: initial.endpoint,
          originHost: initial.originHost,
        }
      : EMPTY,
    validate: {
      name: (v) => (v.trim() ? null : 'Name is required'),
      endpoint: (v) =>
        !v.trim()
          ? 'Endpoint is required'
          : /^[^:]+:[0-9]+$/.test(v.trim())
            ? null
            : 'Endpoint must be host:port',
      originHost: (v) => (v.trim() ? null : 'Origin host is required'),
    },
  });

  // Reset form whenever we get a different peer to edit (modal reuse).
  useEffect(() => {
    form.setValues(
      initial
        ? {
            name: initial.name,
            endpoint: initial.endpoint,
            originHost: initial.originHost,
          }
        : EMPTY,
    );
    form.resetDirty();
    // Form instance is stable — only re-sync when `initial` changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial]);

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
      // Let the caller handle non-field errors (notifications).
      throw err;
    }
  });

  return (
    <form onSubmit={handleSubmit}>
      <Stack gap="sm">
        <TextInput
          label="Name"
          placeholder="peer-06"
          required
          key={form.key('name')}
          {...form.getInputProps('name')}
        />
        <TextInput
          label="Endpoint"
          description="host:port — e.g. 10.0.1.5:3868"
          placeholder="10.0.1.5:3868"
          required
          key={form.key('endpoint')}
          {...form.getInputProps('endpoint')}
        />
        <TextInput
          label="Origin host"
          description="Diameter Origin-Host identity"
          placeholder="ctf-06.test.local"
          required
          key={form.key('originHost')}
          {...form.getInputProps('originHost')}
        />
        <Group justify="flex-end" mt="sm">
          <Button variant="subtle" onClick={onCancel} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" loading={submitting}>
            {submitLabel}
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
