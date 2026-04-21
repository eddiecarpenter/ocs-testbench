import {
  Button,
  Card,
  Group,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { notifications } from '@mantine/notifications';

import {
  isValidMccmnc,
  setSettings,
  useSettings,
} from '../../settings/settings';

/**
 * Client-side preferences. Currently just the MCCMNC used to generate
 * ICCIDs for new subscribers, but this is the natural home for any
 * future UI-side defaults (notification toggles, table densities,
 * theme overrides, etc.).
 */
export function SettingsPage() {
  const settings = useSettings();

  const form = useForm<{ mccmnc: string }>({
    initialValues: { mccmnc: settings.mccmnc },
    validateInputOnChange: true,
    validate: {
      mccmnc: (v) =>
        isValidMccmnc(v.trim())
          ? null
          : 'MCCMNC must be 5 or 6 digits (MCC + MNC)',
    },
  });

  const handleSubmit = form.onSubmit((values) => {
    setSettings({ mccmnc: values.mccmnc.trim() });
    form.resetDirty();
    notifications.show({
      color: 'teal',
      title: 'Settings saved',
      message: `MCCMNC set to ${values.mccmnc.trim()}.`,
    });
  });

  return (
    <Stack gap="lg" p="md">
      <Stack gap={4}>
        <Title order={2} fw={600}>
          Settings
        </Title>
        <Text c="dimmed" size="sm">
          Client-side preferences, stored in this browser.
        </Text>
      </Stack>

      <Card padding="lg" withBorder shadow="xs" maw={560}>
        <form onSubmit={handleSubmit}>
          <Stack gap="md">
            <Stack gap={2}>
              <Text
                size="xs"
                fw={600}
                c="dimmed"
                tt="uppercase"
                style={{ letterSpacing: 0.5 }}
              >
                SIM provisioning
              </Text>
              <Text size="sm" c="dimmed">
                Used as the operator prefix when generating an ICCID for a
                new subscriber. The first three digits are the Mobile
                Country Code (MCC); the remainder is the Mobile Network
                Code (MNC), two or three digits depending on the country.
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

            <Group justify="flex-end">
              <Button
                type="submit"
                disabled={!form.isDirty() || !form.isValid()}
              >
                Save
              </Button>
            </Group>
          </Stack>
        </form>
      </Card>
    </Stack>
  );
}
