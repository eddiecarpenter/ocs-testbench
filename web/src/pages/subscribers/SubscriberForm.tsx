import {
  ActionIcon,
  Button,
  Divider,
  Group,
  Select,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { IconRefresh } from '@tabler/icons-react';
import { useEffect, useMemo } from 'react';

import { ApiError } from '../../api/errors';
import { buildIccid } from '../../api/iccid';
import { buildImei, isValidImei } from '../../api/imei';
import type {
  Subscriber,
  SubscriberInput,
  TacEntry,
} from '../../api/resources/subscribers';
import { isValidMccmnc, useSettings } from '../../settings/settings';

interface SubscriberFormProps {
  /** When provided, the form pre-fills with this subscriber's fields (edit mode). */
  initial?: Subscriber;
  mode: 'create' | 'edit';
  submitting?: boolean;
  deleting?: boolean;
  catalog: TacEntry[];
  onSubmit: (values: SubscriberInput) => Promise<Subscriber | void>;
  onDelete?: () => void;
  onCancel: () => void;
}

/**
 * Form values are a superset of `SubscriberInput`: we carry the picked
 * manufacturer/model as their own keys so the two cascading selects can
 * round-trip, and derive `tac` from them on submit.
 */
interface FormValues {
  msisdn: string;
  iccid: string;
  manufacturer: string;
  model: string;
  imei: string;
}

const EMPTY: FormValues = {
  msisdn: '',
  iccid: '',
  manufacturer: '',
  model: '',
  imei: '',
};

function fromSubscriber(
  sub: Subscriber,
  catalog: TacEntry[],
): FormValues {
  const entry = sub.tac ? catalog.find((e) => e.tac === sub.tac) : undefined;
  return {
    msisdn: sub.msisdn,
    iccid: sub.iccid,
    manufacturer: entry?.manufacturer ?? '',
    model: entry?.model ?? '',
    imei: sub.imei ?? '',
  };
}

/** Small heading like the Peers form. */
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

export function SubscriberForm({
  initial,
  mode,
  submitting,
  deleting,
  catalog,
  onSubmit,
  onDelete,
  onCancel,
}: SubscriberFormProps) {
  const settings = useSettings();
  const form = useForm<FormValues>({
    initialValues: initial ? fromSubscriber(initial, catalog) : EMPTY,
    validateInputOnChange: true,
    validate: {
      msisdn: (v) =>
        /^[0-9]{8,15}$/.test(v.trim())
          ? null
          : 'MSISDN must be 8–15 digits',
      iccid: (v) =>
        /^[0-9]{19,20}$/.test(v.trim())
          ? null
          : 'ICCID must be 19 or 20 digits',
      // Device fields are optional — but if one is set, the other must be
      // too (so the resulting TAC is unambiguous).
      manufacturer: (v, values) =>
        v || !values.model ? null : 'Pick a manufacturer to match the model',
      model: (v, values) =>
        v || !values.manufacturer ? null : 'Pick a model to match the manufacturer',
    },
  });

  // Reset form whenever we switch which subscriber we're editing.
  useEffect(() => {
    form.setValues(initial ? fromSubscriber(initial, catalog) : EMPTY);
    form.resetDirty();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial?.id, catalog]);

  // Manufacturer list is just the distinct brands in the catalogue,
  // alphabetically. Keeping derivation inside the component rather than
  // a module-level constant so it re-runs if the catalogue changes.
  const manufacturers = useMemo(() => {
    const set = new Set<string>();
    for (const e of catalog) set.add(e.manufacturer);
    return [...set].sort().map((m) => ({ value: m, label: m }));
  }, [catalog]);

  const selectedManufacturer = form.getValues().manufacturer;
  const selectedModel = form.getValues().model;

  const models = useMemo(() => {
    if (!selectedManufacturer) return [];
    return catalog
      .filter((e) => e.manufacturer === selectedManufacturer)
      .sort((a, b) => (a.year ?? 0) - (b.year ?? 0) || a.model.localeCompare(b.model))
      .map((e) => ({ value: e.model, label: e.model }));
  }, [catalog, selectedManufacturer]);

  const pickedTac: string | undefined = useMemo(() => {
    if (!selectedManufacturer || !selectedModel) return undefined;
    return catalog.find(
      (e) =>
        e.manufacturer === selectedManufacturer && e.model === selectedModel,
    )?.tac;
  }, [catalog, selectedManufacturer, selectedModel]);

  // When Manufacturer or Model changes, roll a fresh IMEI (or clear it
  // if no device is selected). This mirrors the Figma: the IMEI isn't
  // editable, it's derived from the picked TAC.
  useEffect(() => {
    if (!pickedTac) {
      if (form.getValues().imei) form.setFieldValue('imei', '');
      return;
    }
    const current = form.getValues().imei;
    if (!current || !current.startsWith(pickedTac) || !isValidImei(current)) {
      form.setFieldValue('imei', buildImei(pickedTac) ?? '');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pickedTac]);

  const handleRegenerate = () => {
    if (!pickedTac) return;
    form.setFieldValue('imei', buildImei(pickedTac) ?? '');
  };

  const handleSubmit = form.onSubmit(async (values) => {
    const input: SubscriberInput = {
      msisdn: values.msisdn.trim(),
      iccid: values.iccid.trim(),
      tac: pickedTac || undefined,
      imei: pickedTac && values.imei ? values.imei : undefined,
    };
    try {
      await onSubmit(input);
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        const fieldErrors = err.fieldErrors();
        // Map server `/tac` errors back onto the Manufacturer field so
        // the message lands where the user can act on it. Same story for
        // `/imei` — there's no visible IMEI editor in validation terms,
        // so attach it to the Model field.
        const mapped: Record<string, string> = {};
        for (const [k, v] of Object.entries(fieldErrors)) {
          if (k === 'tac') mapped.manufacturer = v;
          else if (k === 'imei') mapped.model = v;
          else mapped[k] = v;
        }
        if (Object.keys(mapped).length > 0) {
          form.setErrors(mapped);
          return;
        }
      }
      throw err;
    }
  });

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
          {mode === 'edit' ? 'Edit subscriber' : 'Add subscriber'}
        </Text>
        <Title order={3} fw={600}>
          {mode === 'edit' ? (initial?.msisdn ?? 'Subscriber') : 'New subscriber'}
        </Title>
      </Stack>

      {/* Scrollable body */}
      <Stack gap="lg" style={{ flex: 1, overflowY: 'auto' }} pr="xs">
        {/* SIM */}
        <Stack gap="sm">
          <SectionLabel>SIM</SectionLabel>
          <TextInput
            label="MSISDN"
            placeholder="27821234567"
            required
            key={form.key('msisdn')}
            {...form.getInputProps('msisdn')}
          />
          <TextInput
            label="ICCID"
            placeholder="89270100001234567890"
            required
            key={form.key('iccid')}
            {...form.getInputProps('iccid')}
            rightSectionWidth={110}
            rightSection={
              isValidMccmnc(settings.mccmnc) ? (
                <Button
                  variant="subtle"
                  size="compact-xs"
                  leftSection={<IconRefresh size={12} />}
                  onClick={() => {
                    const next = buildIccid(settings.mccmnc);
                    if (next) form.setFieldValue('iccid', next);
                  }}
                >
                  Generate
                </Button>
              ) : (
                <ActionIcon
                  variant="transparent"
                  color="gray"
                  size="sm"
                  disabled
                  aria-label="Generate ICCID (set MCCMNC in Settings)"
                >
                  <IconRefresh size={14} />
                </ActionIcon>
              )
            }
          />
        </Stack>

        <Divider />

        {/* DEVICE */}
        <Stack gap="sm">
          <SectionLabel>Device</SectionLabel>
          <Group grow align="flex-start">
            <Select
              label={
                <Group gap={6}>
                  <Text component="span" size="sm" fw={500}>
                    Manufacturer
                  </Text>
                  <Text component="span" size="xs" c="dimmed">
                    (optional)
                  </Text>
                </Group>
              }
              placeholder="Select"
              data={manufacturers}
              searchable
              clearable
              key={form.key('manufacturer')}
              {...form.getInputProps('manufacturer')}
              onChange={(v) => {
                form.setFieldValue('manufacturer', v ?? '');
                // Switching manufacturer invalidates the model pick.
                form.setFieldValue('model', '');
              }}
            />
            <Select
              label={
                <Group gap={6}>
                  <Text component="span" size="sm" fw={500}>
                    Model
                  </Text>
                  <Text component="span" size="xs" c="dimmed">
                    (optional)
                  </Text>
                </Group>
              }
              placeholder={selectedManufacturer ? 'Select' : '—'}
              data={models}
              disabled={!selectedManufacturer}
              searchable
              clearable
              key={form.key('model')}
              {...form.getInputProps('model')}
              onChange={(v) => form.setFieldValue('model', v ?? '')}
            />
          </Group>
          <TextInput
            label={
              <Group gap={6}>
                <Text component="span" size="sm" fw={500}>
                  IMEI
                </Text>
                <Text component="span" size="xs" c="dimmed">
                  (generated)
                </Text>
              </Group>
            }
            readOnly
            placeholder="—"
            value={form.getValues().imei}
            rightSectionWidth={130}
            rightSection={
              pickedTac ? (
                <Button
                  variant="subtle"
                  size="compact-xs"
                  leftSection={<IconRefresh size={12} />}
                  onClick={handleRegenerate}
                >
                  Regenerate
                </Button>
              ) : (
                <ActionIcon
                  variant="transparent"
                  color="gray"
                  size="sm"
                  disabled
                  aria-label="Regenerate IMEI (disabled)"
                >
                  <IconRefresh size={14} />
                </ActionIcon>
              )
            }
            styles={{ input: { backgroundColor: 'var(--mantine-color-gray-0)' } }}
          />
        </Stack>
      </Stack>

      {/* Footer */}
      <Group
        justify="space-between"
        pt="md"
        mt="md"
        style={{ borderTop: '1px solid var(--mantine-color-gray-3)' }}
      >
        {mode === 'edit' && onDelete ? (
          <Button
            variant="subtle"
            color="red"
            onClick={onDelete}
            loading={deleting}
            disabled={submitting}
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
          <Button
            type="submit"
            loading={submitting}
            disabled={!form.isValid() || deleting}
          >
            {mode === 'edit' ? 'Update' : 'Create'}
          </Button>
        </Group>
      </Group>
    </form>
  );
}
