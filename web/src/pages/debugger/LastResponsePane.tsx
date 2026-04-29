/**
 * Right pane — last-response panel. Filled in by Task #110; this file
 * lands a skeleton that compiles inside the three-pane shell.
 */
import { Stack, Text, Title } from '@mantine/core';

export function LastResponsePane() {
  return (
    <Stack gap="xs" data-testid="debugger-last-response-pane">
      <Title order={5}>Last response</Title>
      <Text size="xs" c="dimmed">
        Result chip + assertions list land in Task #110.
      </Text>
    </Stack>
  );
}
