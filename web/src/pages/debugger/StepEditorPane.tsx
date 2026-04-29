/**
 * Middle pane — step editor + CCR preview. Filled in by Task #109;
 * this file lands a skeleton that compiles inside the three-pane
 * shell.
 */
import { Stack, Text, Title } from '@mantine/core';

export function StepEditorPane() {
  return (
    <Stack gap="xs" data-testid="debugger-step-editor-pane">
      <Title order={5}>Step editor</Title>
      <Text size="xs" c="dimmed">
        Services panel and CCR preview tree land in Task #109.
      </Text>
    </Stack>
  );
}
