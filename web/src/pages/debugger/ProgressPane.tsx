/**
 * Left pane — step progress list. Filled in by Task #108; this file
 * lands a skeleton that compiles inside the three-pane shell.
 */
import { Stack, Text, Title } from '@mantine/core';

export function ProgressPane() {
  return (
    <Stack gap="xs" data-testid="debugger-progress-pane">
      <Title order={5}>Progress</Title>
      <Text size="xs" c="dimmed">
        Step list lands in Task #108.
      </Text>
    </Stack>
  );
}
