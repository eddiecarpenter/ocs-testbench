import { Stack, Text, Title } from '@mantine/core';

/**
 * AI Assistant page — shell component for Feature #177.
 *
 * Tasks 2–6 fill in the chat thread, permission prompt, compose bar,
 * and SSE client.  Task 1 delivers the navigation entry point;
 * this shell renders until those tasks land.
 */
export function AiAssistantPage() {
  return (
    <Stack gap="lg" p="md">
      <Stack gap={4}>
        <Title order={2} fw={600}>
          AI Assistant
        </Title>
        <Text c="dimmed" size="sm">
          Chat with the AI assistant to diagnose faults and verify charging
          configuration.
        </Text>
      </Stack>
    </Stack>
  );
}
