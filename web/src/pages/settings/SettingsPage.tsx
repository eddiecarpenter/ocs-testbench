import {
  ActionIcon,
  Alert,
  Anchor,
  Badge,
  Button,
  Card,
  Divider,
  Group,
  Modal,
  NumberInput,
  PasswordInput,
  SegmentedControl,
  Select,
  Skeleton,
  Stack,
  Switch,
  Text,
  Textarea,
  TextInput,
  Title,
  useMantineColorScheme,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import {
  IconCheck,
  IconDeviceLaptop,
  IconInfoCircle,
  IconMoon,
  IconPlus,
  IconRefresh,
  IconSun,
  IconTrash,
} from '@tabler/icons-react';
import { useEffect, useState } from 'react';

import {
  type CustomDictionary,
  type CustomDictionaryInput,
  useCreateDictionary,
  useDeleteDictionary,
  useDictionaries,
  useUpdateDictionary,
} from '../../api/resources/dictionaries';
import { notifyError } from '../../utils/notify';
import {
  DIAMETER_TRANSPORTS,
  LOG_LEVELS,
  isValidMccmnc,
  setSettings,
  useSettings,
  type Settings as AppSettings,
} from '../../settings/settings';

const BUILT_IN_DICTIONARIES = [
  { id: 'rfc6733',  name: 'RFC 6733 — Diameter Base Protocol' },
  { id: 'rfc4006',  name: 'RFC 4006 — Diameter Credit Control' },
  { id: 'ts32299',  name: '3GPP TS 32.299 — Ro/Rf Charging' },
];

/** Labelled section heading — shared with the Peers / Subscribers forms. */
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
 * Settings page — three sections as per docs/design/screens/06-settings.png:
 *
 *  - **General**: colour scheme (reuses Mantine's store), auto-open browser,
 *    log level.
 *  - **Diameter defaults**: values used when creating a new peer.
 *  - **AVP Dictionaries**: the built-in dictionary catalogue plus any
 *    custom uploads. Read-only stand-in until the backend exposes the
 *    endpoint.
 *
 * All persisted values live in `src/settings/settings.ts` (localStorage).
 * Save writes all sections at once; the Reset button reverts the form to
 * the stored state so a user can discard in-progress edits without
 * reloading the page.
 */
export function SettingsPage() {
  const settings = useSettings();
  const { colorScheme, setColorScheme } = useMantineColorScheme();

  const form = useForm<AppSettings>({
    initialValues: settings,
    validateInputOnChange: true,
    validate: {
      mccmnc: (v) =>
        isValidMccmnc(v.trim())
          ? null
          : 'MCCMNC must be 5 or 6 digits (MCC + MNC)',
      originHostSuffix: (v) =>
        v.trim() ? null : 'Origin-Host suffix is required',
      originRealm: (v) => (v.trim() ? null : 'Origin-Realm is required'),
      watchdogIntervalSeconds: (v) =>
        typeof v === 'number' && v >= 5 && v <= 3600
          ? null
          : 'Watchdog interval must be between 5 and 3600 seconds',
    },
  });

  const handleSubmit = form.onSubmit((values) => {
    setSettings({
      ...values,
      mccmnc: values.mccmnc.trim(),
      originHostSuffix: values.originHostSuffix.trim(),
      originRealm: values.originRealm.trim(),
    });
    form.resetDirty();
    notifications.show({
      color: 'teal',
      title: 'Settings saved',
      message: 'Your preferences were updated.',
    });
  });

  const handleReset = () => {
    form.setValues(settings);
    form.resetDirty();
  };

  return (
    <Stack gap="lg" p="md" maw={720}>
      <Stack gap={4}>
        <Title order={2} fw={600}>
          Settings
        </Title>
        <Text c="dimmed" size="sm">
          Configure testbench defaults and preferences
        </Text>
      </Stack>

      <form onSubmit={handleSubmit}>
        <Stack gap="md">
          {/* ─── General ───────────────────────────────────────── */}
          <Card padding="lg" withBorder shadow="xs">
            <Stack gap="md">
              <SectionLabel>General</SectionLabel>

              <Group justify="space-between" align="center" wrap="nowrap">
                <Text size="sm" fw={500}>
                  Theme
                </Text>
                <SegmentedControl
                  value={colorScheme}
                  onChange={(v) =>
                    setColorScheme(v as 'light' | 'dark' | 'auto')
                  }
                  data={[
                    {
                      value: 'light',
                      label: (
                        <Group gap={6} justify="center">
                          <IconSun size={14} />
                          <Text size="xs">Light</Text>
                        </Group>
                      ),
                    },
                    {
                      value: 'dark',
                      label: (
                        <Group gap={6} justify="center">
                          <IconMoon size={14} />
                          <Text size="xs">Dark</Text>
                        </Group>
                      ),
                    },
                    {
                      value: 'auto',
                      label: (
                        <Group gap={6} justify="center">
                          <IconDeviceLaptop size={14} />
                          <Text size="xs">System</Text>
                        </Group>
                      ),
                    },
                  ]}
                />
              </Group>

              <Group justify="space-between" align="center" wrap="nowrap">
                <Text size="sm" fw={500}>
                  Auto-open browser
                </Text>
                <Switch
                  key={form.key('autoOpenBrowser')}
                  {...form.getInputProps('autoOpenBrowser', { type: 'checkbox' })}
                />
              </Group>

              <Group justify="space-between" align="center" wrap="nowrap">
                <Text size="sm" fw={500}>
                  Log level
                </Text>
                <Select
                  data={LOG_LEVELS}
                  allowDeselect={false}
                  w={160}
                  checkIconPosition="right"
                  key={form.key('logLevel')}
                  {...form.getInputProps('logLevel')}
                />
              </Group>
            </Stack>
          </Card>

          {/* ─── Diameter defaults ─────────────────────────────── */}
          <Card padding="lg" withBorder shadow="xs">
            <Stack gap="md">
              <Stack gap={4}>
                <SectionLabel>Diameter defaults</SectionLabel>
                <Text size="xs" c="dimmed">
                  Used when creating new peers
                </Text>
              </Stack>

              <TextInput
                label="Origin-Host suffix"
                placeholder=".test.local"
                required
                key={form.key('originHostSuffix')}
                {...form.getInputProps('originHostSuffix')}
              />

              <TextInput
                label="Origin-Realm"
                placeholder="test.local"
                required
                key={form.key('originRealm')}
                {...form.getInputProps('originRealm')}
              />

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

              <Select
                label="Default transport"
                data={DIAMETER_TRANSPORTS}
                allowDeselect={false}
                checkIconPosition="right"
                key={form.key('defaultTransport')}
                {...form.getInputProps('defaultTransport')}
              />
            </Stack>
          </Card>

          {/* ─── SIM provisioning ──────────────────────────────── */}
          <Card padding="lg" withBorder shadow="xs">
            <Stack gap="md">
              <Stack gap={4}>
                <SectionLabel>SIM provisioning</SectionLabel>
                <Text size="xs" c="dimmed">
                  Operator prefix used when generating an ICCID. First
                  three digits are the MCC; the remainder is the MNC.
                </Text>
              </Stack>
              <TextInput
                label="MCCMNC"
                placeholder="65510"
                description="5 or 6 digits (e.g. 65510 = MTN South Africa)"
                required
                key={form.key('mccmnc')}
                {...form.getInputProps('mccmnc')}
              />
            </Stack>
          </Card>

          {/* ─── AI Assistant ─────────────────────────────────── */}
          <AiAssistantCard />

          {/* ─── AVP Dictionaries ──────────────────────────────── */}
          <DictionariesCard />

          <Divider />

          {/* ─── Footer: Reset + Save ──────────────────────────── */}
          <Group justify="flex-end">
            <Anchor
              component="button"
              type="button"
              size="sm"
              c="dimmed"
              onClick={handleReset}
              style={{
                visibility: form.isDirty() ? 'visible' : 'hidden',
              }}
            >
              Reset changes
            </Anchor>
            <Button
              type="submit"
              disabled={!form.isDirty() || !form.isValid()}
            >
              Save
            </Button>
          </Group>
        </Stack>
      </form>
    </Stack>
  );
}

// ─── Dictionary management ────────────────────────────────────────────────

function DictionariesCard() {
  const { data: customs, isLoading } = useDictionaries();
  const [editTarget, setEditTarget] = useState<CustomDictionary | null>(null);
  const [createOpen, { open: openCreate, close: closeCreate }] = useDisclosure(false);

  return (
    <Card padding="lg" withBorder shadow="xs">
      <Stack gap="md">
        <Group justify="space-between" align="flex-start" wrap="nowrap">
          <Stack gap={4}>
            <SectionLabel>AVP Dictionaries</SectionLabel>
            <Text size="xs" c="dimmed">
              Built-in RFC/3GPP dictionaries plus custom XML vendor extensions.
              Changes take effect after a server restart.
            </Text>
          </Stack>
          <ActionIcon
            variant="filled"
            size="lg"
            aria-label="Add custom dictionary"
            onClick={openCreate}
          >
            <IconPlus size={16} />
          </ActionIcon>
        </Group>

        <Stack gap="xs">
          {/* Built-in (read-only) */}
          {BUILT_IN_DICTIONARIES.map((d) => (
            <DictRow key={d.id} name={d.name} builtIn />
          ))}

          {/* Custom — live from API */}
          {isLoading && <Skeleton height={48} radius="sm" />}
          {customs?.map((d) => (
            <DictRow
              key={d.id}
              name={d.name}
              description={d.description}
              isActive={d.isActive}
              onEdit={() => setEditTarget(d)}
            />
          ))}
          {!isLoading && customs?.length === 0 && (
            <Text size="sm" c="dimmed" ta="center" py="sm">
              No custom dictionaries yet. Click + to add one.
            </Text>
          )}
        </Stack>
      </Stack>

      {/* Create modal */}
      <DictModal
        opened={createOpen}
        onClose={closeCreate}
        title="Add custom dictionary"
      />

      {/* Edit modal */}
      <DictModal
        opened={editTarget !== null}
        onClose={() => setEditTarget(null)}
        title="Edit dictionary"
        existing={editTarget ?? undefined}
      />
    </Card>
  );
}

function DictRow({
  name,
  description,
  isActive,
  builtIn,
  onEdit,
}: {
  name: string;
  description?: string;
  isActive?: boolean;
  builtIn?: boolean;
  onEdit?: () => void;
}) {
  return (
    <Group
      justify="space-between"
      align="center"
      wrap="nowrap"
      p="sm"
      style={{
        border:
          '1px solid light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))',
        borderRadius: 'var(--mantine-radius-sm)',
      }}
    >
      <Group gap="sm" wrap="nowrap">
        <Text size="sm" c={builtIn ? 'teal' : isActive ? 'blue' : 'dimmed'} aria-hidden>
          ●
        </Text>
        <Stack gap={0}>
          <Text size="sm" fw={500}>
            {name}
          </Text>
          {(description || builtIn) && (
            <Text size="xs" c="dimmed">
              {builtIn ? 'Built-in' : description}
            </Text>
          )}
        </Stack>
      </Group>

      {builtIn ? (
        <Badge variant="light" color="gray" radius="sm">Built-in</Badge>
      ) : (
        <Group gap="xs">
          <Badge variant="light" color={isActive ? 'blue' : 'gray'} radius="sm">
            {isActive ? 'Active' : 'Inactive'}
          </Badge>
          <ActionIcon variant="subtle" size="sm" onClick={onEdit} aria-label="Edit">
            <IconRefresh size={14} />
          </ActionIcon>
        </Group>
      )}
    </Group>
  );
}

interface DictModalProps {
  opened: boolean;
  onClose: () => void;
  title: string;
  existing?: CustomDictionary;
}

function DictModal({ opened, onClose, title, existing }: DictModalProps) {
  const createMut = useCreateDictionary();
  const updateMut = useUpdateDictionary();
  const deleteMut = useDeleteDictionary();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const form = useForm<CustomDictionaryInput>({
    initialValues: {
      name: existing?.name ?? '',
      description: existing?.description ?? '',
      xmlContent: existing?.xmlContent ?? '',
      isActive: existing?.isActive ?? true,
    },
  });

  // Re-seed the form when the modal opens or switches to a different row.
  // Cannot use Modal's onTransitionEnd — it bubbles from every child CSS
  // transition (focus rings etc.) and would reset the form on every blur.
  useEffect(() => {
    if (!opened) return;
    form.setValues({
      name: existing?.name ?? '',
      description: existing?.description ?? '',
      xmlContent: existing?.xmlContent ?? '',
      isActive: existing?.isActive ?? true,
    });
    form.resetDirty();
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setConfirmDelete(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened, existing?.id]);

  const handleSubmit = form.onSubmit(async (values) => {
    try {
      if (existing) {
        await updateMut.mutateAsync({ id: existing.id, input: values });
        notifications.show({ color: 'teal', message: 'Dictionary updated. Restart to apply.' });
      } else {
        await createMut.mutateAsync(values);
        notifications.show({ color: 'teal', message: 'Dictionary created. Restart to apply.' });
      }
      onClose();
    } catch (e) {
      notifyError({ title: 'Failed to save dictionary', message: String(e) });
    }
  });

  const handleDelete = async () => {
    if (!existing) return;
    try {
      await deleteMut.mutateAsync(existing.id);
      notifications.show({ color: 'orange', message: 'Dictionary deleted.' });
      onClose();
    } catch (e) {
      notifyError({ title: 'Failed to delete dictionary', message: String(e) });
    }
  };

  const busy = createMut.isPending || updateMut.isPending || deleteMut.isPending;

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={title}
      size="xl"
    >
      <form onSubmit={handleSubmit}>
        <Stack gap="md">
          <Group grow>
            <TextInput
              label="Name"
              placeholder="Huawei-Gy"
              required
              {...form.getInputProps('name')}
            />
            <TextInput
              label="Description"
              placeholder="Huawei vendor-specific IN_INFORMATION AVPs"
              {...form.getInputProps('description')}
            />
          </Group>

          <Switch
            label="Active — load at startup"
            {...form.getInputProps('isActive', { type: 'checkbox' })}
          />

          <Textarea
            label="XML content"
            description="Diameter dictionary XML in go-diameter format"
            placeholder={'<?xml version="1.0" encoding="UTF-8"?>\n<diameter>\n  <application id="4" …>\n    …\n  </application>\n</diameter>'}
            required
            autosize
            minRows={14}
            maxRows={28}
            styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
            {...form.getInputProps('xmlContent')}
          />

          <Group justify="space-between">
            {existing ? (
              confirmDelete ? (
                <Group gap="xs">
                  <Text size="sm" c="red">Delete this dictionary?</Text>
                  <Button size="xs" color="red" loading={deleteMut.isPending} onClick={handleDelete}>
                    Confirm
                  </Button>
                  <Button size="xs" variant="subtle" onClick={() => setConfirmDelete(false)}>
                    Cancel
                  </Button>
                </Group>
              ) : (
                <ActionIcon
                  variant="subtle"
                  color="red"
                  size="lg"
                  aria-label="Delete dictionary"
                  onClick={() => setConfirmDelete(true)}
                >
                  <IconTrash size={16} />
                </ActionIcon>
              )
            ) : (
              <span />
            )}

            <Group gap="xs">
              <Button variant="default" onClick={onClose} disabled={busy}>
                Cancel
              </Button>
              <Button
                type="submit"
                loading={createMut.isPending || updateMut.isPending}
                leftSection={<IconCheck size={14} />}
              >
                {existing ? 'Save' : 'Create'}
              </Button>
            </Group>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}

// ─── AI Assistant settings ────────────────────────────────────────────────────

/**
 * Provider preset definitions.
 *
 * The API endpoint is pre-filled when a provider is selected.  All fields
 * are disabled in this release — they become functional in Feature 3.
 *
 * Anthropic uses its own Messages API rather than the OpenAI-compatible
 * `/v1/chat/completions` path; Feature 3 must implement a separate adapter
 * for it behind the common client interface.
 */
const PROVIDER_PRESETS: Record<string, string> = {
  openai: 'https://api.openai.com',
  anthropic: 'https://api.anthropic.com',
  ollama: 'http://localhost:11434',
  lmstudio: 'http://localhost:1234',
  llamacpp: 'http://localhost:8080',
  custom: '',
};

const PROVIDER_OPTIONS = [
  { value: 'openai',    label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama',    label: 'Ollama' },
  { value: 'lmstudio',  label: 'LM Studio' },
  { value: 'llamacpp',  label: 'llama.cpp' },
  { value: 'custom',    label: 'Custom' },
];

/**
 * AI Assistant settings card.
 *
 * All fields are disabled with an info banner in this release.
 * LLM integration (provider auth, model fetch, live endpoint) is
 * enabled in Feature 3.
 */
function AiAssistantCard() {
  const [provider, setProvider] = useState<string>('openai');
  const endpoint = PROVIDER_PRESETS[provider] ?? '';

  return (
    <Card padding="lg" withBorder shadow="xs">
      <Stack gap="md">
        <Stack gap={4}>
          <SectionLabel>AI Assistant</SectionLabel>
          <Text size="xs" c="dimmed">
            Configure the LLM provider used by the AI Assistant.
          </Text>
        </Stack>

        <Alert
          icon={<IconInfoCircle size={16} />}
          color="blue"
          variant="light"
          radius="sm"
        >
          These settings are non-functional in this release. LLM integration
          is enabled in Feature 3.
        </Alert>

        <Select
          label="Provider"
          description="Preset auto-fills the API endpoint"
          data={PROVIDER_OPTIONS}
          value={provider}
          onChange={(v) => setProvider(v ?? 'openai')}
          allowDeselect={false}
          disabled
          checkIconPosition="right"
        />

        <TextInput
          label="API endpoint"
          placeholder="https://api.openai.com"
          value={endpoint}
          readOnly
          disabled
        />

        <PasswordInput
          label="API key"
          placeholder="sk-…"
          disabled
        />
      </Stack>
    </Card>
  );
}
