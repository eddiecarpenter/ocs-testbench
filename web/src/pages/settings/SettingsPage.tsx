import {
  ActionIcon,
  Anchor,
  Badge,
  Box,
  Button,
  Card,
  Divider,
  Group,
  Modal,
  NumberInput,
  PasswordInput,
  Radio,
  ScrollArea,
  SegmentedControl,
  Select,
  Skeleton,
  Stack,
  Switch,
  Table,
  Collapse,
  Text,
  TextInput,
  useMantineColorScheme,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { useDisclosure, useLocalStorage } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import {
  IconCheck,
  IconDeviceLaptop,
  IconMoon,
  IconPencil,
  IconPlus,
  IconRefresh,
  IconSun,
  IconTrash,
} from '@tabler/icons-react';
import Editor from '@monaco-editor/react';
import { useEffect, useState } from 'react';

import {
  useAIConfig,
  useAIModels,
  useUpdateAIConfig,
} from '../../api/resources/aiConfig';
import {
  type AIToolPermission,
  useAIPermissions,
  useSetAIPermission,
} from '../../api/resources/aiPermissions';
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

// ─── Nav ─────────────────────────────────────────────────────────────────────

const NAV_ITEMS = [
  { id: 'general',      label: 'General' },
  { id: 'diameter',     label: 'Diameter' },
  { id: 'ai',           label: 'AI Assistant' },
  { id: 'permissions',  label: 'AI Permissions' },
  { id: 'dictionaries', label: 'Dictionaries' },
] as const;

type NavId = typeof NAV_ITEMS[number]['id'];

function NavItem({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <Box
      px="md"
      py="xs"
      onClick={onClick}
      style={{
        cursor: 'pointer',
        borderRadius: 'var(--mantine-radius-sm)',
        backgroundColor: active ? 'var(--mantine-color-blue-light)' : 'transparent',
        color: active ? 'var(--mantine-color-blue-filled)' : 'inherit',
        fontWeight: active ? 600 : 400,
        fontSize: 'var(--mantine-font-size-sm)',
        userSelect: 'none',
      }}
    >
      {label}
    </Box>
  );
}

export function SettingsPage() {
  const [activeSection, setActiveSection] = useState<NavId>('general');
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
    notifications.show({ color: 'teal', title: 'Settings saved', message: 'Your preferences were updated.' });
  });

  const handleReset = () => {
    form.setValues(settings);
    form.resetDirty();
  };

  return (
    <Group align="flex-start" gap={0} style={{ height: 'calc(100vh - 60px)' }}>
      {/* ─── Left navigation ─────────────────────────────────── */}
      <Box
        p="sm"
        style={{
          width: 180,
          flexShrink: 0,
          borderRight: '1px solid light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))',
          height: '100%',
        }}
      >
        <Stack gap={2} pt="xs">
          <Text size="xs" fw={600} c="dimmed" tt="uppercase" px="md" pb={4} style={{ letterSpacing: 0.5 }}>
            Settings
          </Text>
          {NAV_ITEMS.map((item) => (
            <NavItem
              key={item.id}
              label={item.label}
              active={activeSection === item.id}
              onClick={() => setActiveSection(item.id)}
            />
          ))}
        </Stack>
      </Box>

      {/* ─── Content area ────────────────────────────────────── */}
      <ScrollArea style={{ flex: 1, height: '100%' }}>
        <Box p="lg" maw={720}>
          <form onSubmit={handleSubmit}>

            {/* ─── General ───────────────────────────────────── */}
            {activeSection === 'general' && (
              <Stack gap="md">
                <Card padding="lg" withBorder shadow="xs">
                  <Stack gap="md">
                    <SectionLabel>General</SectionLabel>

                    <Group justify="space-between" align="center" wrap="nowrap">
                      <Text size="sm" fw={500}>Theme</Text>
                      <SegmentedControl
                        value={colorScheme}
                        onChange={(v) => setColorScheme(v as 'light' | 'dark' | 'auto')}
                        data={[
                          { value: 'light', label: (<Group gap={6} justify="center"><IconSun size={14} /><Text size="xs">Light</Text></Group>) },
                          { value: 'dark',  label: (<Group gap={6} justify="center"><IconMoon size={14} /><Text size="xs">Dark</Text></Group>) },
                          { value: 'auto',  label: (<Group gap={6} justify="center"><IconDeviceLaptop size={14} /><Text size="xs">System</Text></Group>) },
                        ]}
                      />
                    </Group>

                    <Group justify="space-between" align="center" wrap="nowrap">
                      <Text size="sm" fw={500}>Auto-open browser</Text>
                      <Switch key={form.key('autoOpenBrowser')} {...form.getInputProps('autoOpenBrowser', { type: 'checkbox' })} />
                    </Group>

                    <Group justify="space-between" align="center" wrap="nowrap">
                      <Text size="sm" fw={500}>Log level</Text>
                      <Select data={LOG_LEVELS} allowDeselect={false} w={160} checkIconPosition="right"
                        key={form.key('logLevel')} {...form.getInputProps('logLevel')} />
                    </Group>
                  </Stack>
                </Card>

                <Divider />
                <Group justify="flex-end">
                  <Anchor component="button" type="button" size="sm" c="dimmed" onClick={handleReset}
                    style={{ visibility: form.isDirty() ? 'visible' : 'hidden' }}>
                    Reset changes
                  </Anchor>
                  <Button type="submit" disabled={!form.isDirty() || !form.isValid()}>Save</Button>
                </Group>
              </Stack>
            )}

            {/* ─── Diameter ──────────────────────────────────── */}
            {activeSection === 'diameter' && (
              <Stack gap="md">
                <Card padding="lg" withBorder shadow="xs">
                  <Stack gap="md">
                    <Stack gap={4}>
                      <SectionLabel>Diameter defaults</SectionLabel>
                      <Text size="xs" c="dimmed">Used when creating new peers</Text>
                    </Stack>
                    <TextInput label="Origin-Host suffix" placeholder=".test.local" required
                      key={form.key('originHostSuffix')} {...form.getInputProps('originHostSuffix')} />
                    <TextInput label="Origin-Realm" placeholder="test.local" required
                      key={form.key('originRealm')} {...form.getInputProps('originRealm')} />
                    <NumberInput label="Watchdog interval" suffix=" seconds" min={5} max={3600}
                      clampBehavior="strict" required key={form.key('watchdogIntervalSeconds')}
                      {...form.getInputProps('watchdogIntervalSeconds')} />
                    <Select label="Default transport" data={DIAMETER_TRANSPORTS} allowDeselect={false}
                      checkIconPosition="right" key={form.key('defaultTransport')}
                      {...form.getInputProps('defaultTransport')} />
                  </Stack>
                </Card>

                <Card padding="lg" withBorder shadow="xs">
                  <Stack gap="md">
                    <Stack gap={4}>
                      <SectionLabel>SIM provisioning</SectionLabel>
                      <Text size="xs" c="dimmed">
                        Operator prefix used when generating an ICCID. First three digits are the MCC; the remainder is the MNC.
                      </Text>
                    </Stack>
                    <TextInput label="MCCMNC" placeholder="65510"
                      description="5 or 6 digits (e.g. 65510 = MTN South Africa)" required
                      key={form.key('mccmnc')} {...form.getInputProps('mccmnc')} />
                  </Stack>
                </Card>

                <Divider />
                <Group justify="flex-end">
                  <Anchor component="button" type="button" size="sm" c="dimmed" onClick={handleReset}
                    style={{ visibility: form.isDirty() ? 'visible' : 'hidden' }}>
                    Reset changes
                  </Anchor>
                  <Button type="submit" disabled={!form.isDirty() || !form.isValid()}>Save</Button>
                </Group>
              </Stack>
            )}

            {/* ─── AI Assistant ──────────────────────────────── */}
            {activeSection === 'ai' && <AiAssistantCard />}

            {/* ─── AI Permissions ────────────────────────────── */}
            {activeSection === 'permissions' && <AIPermissionsCard />}

            {/* ─── Dictionaries ──────────────────────────────── */}
            {activeSection === 'dictionaries' && <DictionariesCard />}

          </form>
        </Box>
      </ScrollArea>
    </Group>
  );
}

// ─── AI Tool Permissions ──────────────────────────────────────────────────

/** Maps tier names to Mantine Badge colors. */
const TIER_COLOR: Record<string, string> = {
  readonly: 'blue',
  write: 'orange',
  destructive: 'red',
};

/**
 * Inline row for a single MCP tool permission.
 *
 * The SegmentedControl fires the PATCH mutation immediately when the
 * operator changes the value — no separate Save button is needed.
 */
function PermissionRow({ perm }: { perm: AIToolPermission }) {
  const setPermMut = useSetAIPermission();

  const handleChange = (value: string) => {
    setPermMut.mutate({
      toolName: perm.toolName,
      decision: value as AIToolPermission['decision'],
    });
  };

  return (
    <Table.Tr>
      <Table.Td>
        <Text size="sm" ff="monospace">
          {perm.toolName}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="sm" c="dimmed" lineClamp={2}>
          {perm.description ?? '—'}
        </Text>
      </Table.Td>
      <Table.Td>
        {perm.tier ? (
          <Badge
            variant="light"
            color={TIER_COLOR[perm.tier] ?? 'gray'}
            radius="sm"
            size="sm"
          >
            {perm.tier}
          </Badge>
        ) : (
          <Text size="xs" c="dimmed">
            —
          </Text>
        )}
      </Table.Td>
      <Table.Td>
        <SegmentedControl
          size="xs"
          value={perm.decision ?? 'ask'}
          onChange={handleChange}
          disabled={setPermMut.isPending}
          data={[
            { value: 'ask', label: 'Ask' },
            { value: 'allow', label: 'Allow' },
            { value: 'deny', label: 'Deny' },
          ]}
        />
      </Table.Td>
    </Table.Tr>
  );
}

// Tool-name prefix → group label. Order determines display order.
const PERMISSION_GROUPS: { label: string; match: (name: string) => boolean }[] = [
  { label: 'Scenarios',   match: (n) => n.includes('scenario') },
  { label: 'Executions',  match: (n) => n.includes('execution') || n.includes('step') || n.includes('resume') || n.includes('start_') || n.includes('stop_') || n.includes('wait_') },
  { label: 'Peers',       match: (n) => n.includes('peer') },
  { label: 'Subscribers', match: (n) => n.includes('subscriber') },
  { label: 'AVPs',        match: (n) => n.includes('avp') },
  { label: 'System',      match: () => true },
];

function groupPermissions(perms: AIToolPermission[]): { label: string; items: AIToolPermission[] }[] {
  const assigned = new Set<string>();
  return PERMISSION_GROUPS.map(({ label, match }) => {
    const items = perms.filter((p) => !assigned.has(p.toolName) && match(p.toolName));
    items.forEach((p) => assigned.add(p.toolName));
    return { label, items };
  }).filter((g) => g.items.length > 0);
}

function PermissionGroup({ label, items }: { label: string; items: AIToolPermission[] }) {
  const [open, setOpen] = useState(true);
  return (
    <Card padding={0} withBorder shadow="xs">
      <Box
        px="md"
        py="xs"
        onClick={() => setOpen((o) => !o)}
        style={{
          cursor: 'pointer',
          borderBottom: open ? '1px solid light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))' : 'none',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Text size="sm" fw={600}>{label}</Text>
        <Text size="xs" c="dimmed">{open ? '▲' : '▼'} {items.length} tools</Text>
      </Box>
      <Collapse expanded={open}>
        <Table striped highlightOnHover withColumnBorders>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Tool</Table.Th>
              <Table.Th>Description</Table.Th>
              <Table.Th>Tier</Table.Th>
              <Table.Th>Permission</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {items.map((p) => (
              <PermissionRow key={p.toolName} perm={p} />
            ))}
          </Table.Tbody>
        </Table>
      </Collapse>
    </Card>
  );
}

function AIPermissionsCard() {
  const { data: permissions, isLoading } = useAIPermissions();
  const groups = permissions ? groupPermissions(permissions) : [];

  return (
    <Stack gap="md">
      <Stack gap={4}>
        <SectionLabel>AI Tool Permissions</SectionLabel>
        <Text size="xs" c="dimmed">
          Configure how the AI assistant handles each MCP tool call.
          Changes take effect immediately — no restart required.
        </Text>
      </Stack>

      {isLoading ? (
        <Skeleton height={120} radius="sm" />
      ) : groups.length === 0 ? (
        <Text size="sm" c="dimmed" ta="center" py="sm">
          No MCP tools available. Start the server with a configured AI endpoint to see tools here.
        </Text>
      ) : (
        <Stack gap="sm">
          {groups.map((g) => (
            <PermissionGroup key={g.label} label={g.label} items={g.items} />
          ))}
        </Stack>
      )}
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
            <IconPencil size={14} />
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
  const { colorScheme } = useMantineColorScheme();

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
      size="xxl"
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

          <Stack gap={4}>
            <Text size="sm" fw={500}>
              XML content <Text span c="red" aria-hidden>*</Text>
            </Text>
            <Text size="xs" c="dimmed">Diameter dictionary XML in go-diameter format</Text>
            <Box
              style={(theme) => ({
                border: `1px solid ${theme.colors.dark[4]}`,
                borderRadius: theme.radius.sm,
                overflow: 'hidden',
              })}
            >
              <Editor
                height={400}
                language="xml"
                theme={colorScheme === 'dark' ? 'vs-dark' : 'light'}
                value={form.values.xmlContent}
                onChange={(v: string | undefined) => form.setFieldValue('xmlContent', v ?? '')}
                options={{
                  minimap: { enabled: false },
                  scrollBeyondLastLine: false,
                  fontSize: 13,
                  tabSize: 2,
                  wordWrap: 'on',
                  lineNumbersMinChars: 3,
                }}
              />
            </Box>
            {form.errors.xmlContent && (
              <Text size="xs" c="red">{form.errors.xmlContent}</Text>
            )}
          </Stack>

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
  anthropic: 'https://api.anthropic.com',
  openai:    'https://api.openai.com',
  ollama:    'http://localhost:11434',
  lmstudio:  'http://localhost:1234',
  llamacpp:  'http://localhost:8080',
  custom:    '',
};

const PROVIDER_OPTIONS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai',    label: 'OpenAI' },
  { value: 'ollama',    label: 'Ollama' },
  { value: 'lmstudio',  label: 'LM Studio' },
  { value: 'llamacpp',  label: 'llama.cpp' },
  { value: 'custom',    label: 'Custom' },
];

/** Height of five visible rows in the scrollable model list. */
const MODEL_LIST_HEIGHT = 40 * 5; // ~40px per radio row × 5 rows

/**
 * Detects the closest PROVIDER_PRESETS key for a given endpoint URL.
 * Falls back to 'custom' when no preset matches.
 */
function detectProvider(endpoint: string): string {
  for (const [key, url] of Object.entries(PROVIDER_PRESETS)) {
    if (url && endpoint.startsWith(url)) return key;
  }
  return endpoint ? 'custom' : 'openai';
}

/**
 * AI Assistant settings card.
 *
 * Wired to the live backend via GET/PATCH /v1/config/ai and
 * GET /v1/config/ai/models. Settings are self-contained — Save/Reset
 * are independent of the global Settings page form.
 */
function AiAssistantCard() {
  const { data: serverCfg, isLoading: cfgLoading } = useAIConfig();
  const updateMut = useUpdateAIConfig();

  // Local form state — seeded from the server on first load.
  const [provider, setProvider] = useState<string>('openai');
  const [endpoint, setEndpoint] = useState('');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [thinking, setThinking] = useState(false);
  const [modelFilter, setModelFilter] = useState('');
  const [seeded, setSeeded] = useState(false);

  // UI-only preference persisted in localStorage — no backend needed.
  const [hideToolCalls, setHideToolCalls] = useLocalStorage({
    key: 'ai-hide-tool-calls',
    defaultValue: false,
  });

  // Seed local state from server config on first successful fetch.
  useEffect(() => {
    if (!serverCfg || seeded) return;
    /* eslint-disable react-hooks/set-state-in-effect */
    setProvider(detectProvider(serverCfg.endpoint));
    setEndpoint(serverCfg.endpoint);
    setModel(serverCfg.model);
    setThinking(serverCfg.thinking ?? false);
    setSeeded(true);
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [serverCfg, seeded]);

  // Fetch model list when an endpoint is configured; pass the unsaved API key
  // so models can be fetched before Save (important for new key entry).
  const {
    data: models,
    isFetching: modelsFetching,
    isError: modelsError,
    error: modelsErrorDetail,
    refetch: refetchModels,
  } = useAIModels(endpoint, apiKey || undefined);

  const filteredModels = (models ?? []).filter((m) =>
    m.toLowerCase().includes(modelFilter.toLowerCase()),
  );

  const handleProviderChange = (v: string | null) => {
    const next = v ?? 'openai';
    setProvider(next);
    const preset = PROVIDER_PRESETS[next];
    if (preset !== undefined) {
      setEndpoint(preset);
    }
    setModel(''); // Clear model when provider changes.
  };

  const handleSave = async () => {
    try {
      await updateMut.mutateAsync({
        endpoint,
        model,
        thinking,
        // Send the key only when the user typed something; otherwise blank
        // tells the backend to preserve the existing key.
        apiKey: apiKey || undefined,
      });
      setApiKey(''); // Clear the field after save so it shows "unchanged" placeholder.
      notifications.show({
        color: 'teal',
        title: 'AI settings saved',
        message: 'Active immediately — no restart required.',
      });
    } catch (e) {
      notifyError({ title: 'Failed to save AI settings', message: String(e) });
    }
  };

  const handleReset = () => {
    if (!serverCfg) return;
    setProvider(detectProvider(serverCfg.endpoint));
    setEndpoint(serverCfg.endpoint);
    setModel(serverCfg.model);
    setThinking(serverCfg.thinking ?? false);
    setApiKey('');
  };

  const apiKeyPlaceholder = serverCfg?.apiKeySet && !apiKey
    ? '•••• (unchanged)'
    : 'sk-…';

  return (
    <Card padding="lg" withBorder shadow="xs">
      <Stack gap="md">
        <Stack gap={4}>
          <SectionLabel>AI Assistant</SectionLabel>
          <Text size="xs" c="dimmed">
            Configure the LLM provider used by the AI Assistant.
          </Text>
        </Stack>

        {cfgLoading ? (
          <Skeleton height={200} radius="sm" />
        ) : (
          <>
            <Select
              label="Provider"
              description="Preset auto-fills the API endpoint"
              data={PROVIDER_OPTIONS}
              value={provider}
              onChange={handleProviderChange}
              allowDeselect={false}
              checkIconPosition="right"
            />

            <TextInput
              label="API endpoint"
              placeholder="https://api.openai.com"
              value={endpoint}
              onChange={(e) => setEndpoint(e.currentTarget.value)}
            />

            <PasswordInput
              label="API key"
              placeholder={apiKeyPlaceholder}
              value={apiKey}
              onChange={(e) => setApiKey(e.currentTarget.value)}
              description={
                provider === 'anthropic'
                  ? 'Requires an Anthropic API key from console.anthropic.com.'
                  : undefined
              }
            />

            {/* ─── Model list sub-section ──────────────────────── */}
            <Box
              style={{
                border:
                  '1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))',
                borderRadius: 'var(--mantine-radius-sm)',
              }}
            >
              {/* Sub-heading row */}
              <Group
                justify="space-between"
                align="center"
                px="sm"
                py="xs"
                style={{
                  borderBottom:
                    '1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))',
                }}
              >
                <Text size="sm" fw={600}>
                  Models
                </Text>
                <ActionIcon
                  variant="subtle"
                  size="sm"
                  aria-label="Refresh model list"
                  loading={modelsFetching}
                  disabled={!endpoint}
                  onClick={() => void refetchModels()}
                  title={endpoint ? 'Refresh model list from endpoint' : 'Configure an endpoint first'}
                >
                  <IconRefresh size={14} />
                </ActionIcon>
              </Group>

              {/* Status bar */}
              <Box px="sm" py={6}>
                {!endpoint ? (
                  <Text size="xs" c="dimmed">
                    Configure endpoint to fetch models
                  </Text>
                ) : modelsFetching ? (
                  <Text size="xs" c="dimmed">
                    Fetching models…
                  </Text>
                ) : modelsError ? (
                  <Text size="xs" c="red">
                    {String((modelsErrorDetail as Error)?.message ?? modelsErrorDetail ?? 'Failed to fetch models')}
                  </Text>
                ) : models && models.length > 0 ? (
                  <Text size="xs" c="dimmed">
                    {models.length} model{models.length !== 1 ? 's' : ''} available
                  </Text>
                ) : (
                  <Text size="xs" c="dimmed">
                    Click Refresh to load models from the endpoint
                  </Text>
                )}
              </Box>

              {/* Filter input */}
              <Box px="sm" pb="xs">
                <TextInput
                  size="xs"
                  placeholder="Filter models…"
                  value={modelFilter}
                  onChange={(e) => setModelFilter(e.currentTarget.value)}
                  disabled={!models || models.length === 0}
                  aria-label="Filter models"
                />
              </Box>

              {/* Scrollable radio list — fixed height showing 5 rows */}
              <ScrollArea h={MODEL_LIST_HEIGHT} px="sm" pb="xs">
                {modelsFetching ? (
                  <Stack gap={4}>
                    {[1, 2, 3].map((i) => (
                      <Skeleton key={i} height={28} radius="sm" />
                    ))}
                  </Stack>
                ) : (
                  <Radio.Group
                    value={model}
                    onChange={(v) => setModel(v)}
                  >
                    <Stack gap={4}>
                      {filteredModels.map((m) => (
                        <Radio
                          key={m}
                          value={m}
                          label={m}
                          size="xs"
                          styles={{ label: { fontFamily: 'monospace', fontSize: 12 } }}
                        />
                      ))}
                      {filteredModels.length === 0 && (
                        <Text size="xs" c="dimmed" ta="center" py="sm">
                          {models && models.length > 0
                            ? 'No models match filter.'
                            : 'No models loaded — click Refresh.'}
                        </Text>
                      )}
                    </Stack>
                  </Radio.Group>
                )}
              </ScrollArea>
            </Box>

            {/* ─── Behaviour toggles ───────────────────────────── */}
            <Stack gap="xs">
              <Switch
                label="Enable thinking mode"
                description="Allows the model to reason internally before responding (slower but more accurate for complex questions)"
                checked={thinking}
                onChange={(e) => setThinking(e.currentTarget.checked)}
                size="sm"
              />
              <Switch
                label="Hide tool execution"
                description="Suppress tool-call blocks in the chat thread — only show the final answer"
                checked={hideToolCalls}
                onChange={(e) => setHideToolCalls(e.currentTarget.checked)}
                size="sm"
              />
            </Stack>

            {/* ─── Self-contained Save / Reset ─────────────────── */}
            <Group justify="flex-end">
              <Button
                variant="subtle"
                size="sm"
                onClick={handleReset}
                disabled={updateMut.isPending}
              >
                Reset
              </Button>
              <Button
                size="sm"
                loading={updateMut.isPending}
                leftSection={<IconCheck size={14} />}
                onClick={() => void handleSave()}
              >
                Save AI settings
              </Button>
            </Group>
          </>
        )}
      </Stack>
    </Card>
  );
}
