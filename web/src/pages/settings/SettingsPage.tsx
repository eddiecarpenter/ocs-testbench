import {
  ActionIcon,
  Anchor,
  Badge,
  Button,
  Card,
  Divider,
  Group,
  NumberInput,
  SegmentedControl,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
  Title,
  Tooltip,
  useMantineColorScheme,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { notifications } from '@mantine/notifications';
import {
  IconDeviceLaptop,
  IconMoon,
  IconSun,
  IconUpload,
} from '@tabler/icons-react';

import {
  DIAMETER_TRANSPORTS,
  LOG_LEVELS,
  isValidMccmnc,
  setSettings,
  useSettings,
  type Settings as AppSettings,
} from '../../settings/settings';

/** RFC-7807-shaped placeholder list for built-in dictionaries — matches
 * the catalogue documented in ARCHITECTURE.md §11 plus a couple of
 * illustrative custom entries from Figma 06-settings.png. The v0.2
 * OpenAPI does not expose a /dictionaries endpoint yet, so this list is
 * rendered as a visual reference; Upload XML is disabled with a tooltip
 * explaining the gap. When the endpoint lands, swap the static fixture
 * for a `useDictionaries()` query. */
interface DictionaryRow {
  id: string;
  name: string;
  origin: 'built-in' | 'custom';
  avpCount: number;
}

const DICTIONARY_FIXTURES: DictionaryRow[] = [
  { id: 'rfc6733', name: 'RFC 6733 base', origin: 'built-in', avpCount: 147 },
  { id: 'rfc4006', name: 'RFC 4006 credit control', origin: 'built-in', avpCount: 89 },
  { id: 'ts32299', name: '3GPP TS 32.299 Ro/Rf', origin: 'built-in', avpCount: 312 },
  { id: 'nokia-vendor', name: 'Nokia vendor-specific', origin: 'custom', avpCount: 42 },
  { id: 'ericsson-sms', name: 'Ericsson SMS extensions', origin: 'custom', avpCount: 18 },
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

          {/* ─── AVP Dictionaries ──────────────────────────────── */}
          <Card padding="lg" withBorder shadow="xs">
            <Stack gap="md">
              <Group justify="space-between" align="flex-start" wrap="nowrap">
                <Stack gap={4}>
                  <SectionLabel>AVP Dictionaries</SectionLabel>
                  <Text size="xs" c="dimmed">
                    Built-in RFC dictionaries plus custom XML uploads
                  </Text>
                </Stack>
                <Tooltip
                  label="Upload endpoint not available in OpenAPI v0.2 yet"
                  withArrow
                  position="left"
                >
                  <ActionIcon
                    variant="default"
                    size="lg"
                    disabled
                    aria-label="Upload custom dictionary XML"
                  >
                    <IconUpload size={16} />
                  </ActionIcon>
                </Tooltip>
              </Group>

              <Stack gap="xs">
                {DICTIONARY_FIXTURES.map((d) => (
                  <DictionaryRow key={d.id} row={d} />
                ))}
              </Stack>
            </Stack>
          </Card>

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

/** Single row in the AVP Dictionaries section. */
function DictionaryRow({ row }: { row: DictionaryRow }) {
  const originLabel = row.origin === 'built-in' ? 'Built-in' : 'Custom';
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
        <Text
          size="sm"
          fw={500}
          c={row.origin === 'built-in' ? 'teal' : 'blue'}
          aria-hidden="true"
        >
          ●
        </Text>
        <Stack gap={0}>
          <Text size="sm" fw={500}>
            {row.name}
          </Text>
          <Text size="xs" c="dimmed">
            {originLabel} · {row.avpCount} AVPs
          </Text>
        </Stack>
      </Group>
      {row.origin === 'built-in' ? (
        <Badge variant="light" color="gray" radius="sm">
          Built-in
        </Badge>
      ) : (
        <Tooltip
          label="Remove endpoint not available in OpenAPI v0.2 yet"
          withArrow
          position="left"
        >
          <Anchor
            component="button"
            type="button"
            size="sm"
            c="red"
            disabled
            style={{ opacity: 0.5, cursor: 'not-allowed' }}
          >
            Remove
          </Anchor>
        </Tooltip>
      )}
    </Group>
  );
}
