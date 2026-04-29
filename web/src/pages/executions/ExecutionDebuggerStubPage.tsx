/**
 * Stub for the per-execution debugger route (`/executions/:id`).
 *
 * Feature #95 owns the real Debugger surface — replay, step controls,
 * SSE-driven event log, context inspector. Until that lands, this stub
 * keeps the route reachable so the list / Start-Run flow can navigate
 * to a freshly-started Interactive run without breaking.
 *
 * Reuse: as-is — the existing `PlaceholderPage` already provides the
 * "coming soon" affordance; this stub specialises the title and adds a
 * row id badge so the test reaches a known content target.
 */
import { Center, Stack, Text, Title } from '@mantine/core';
import { useParams } from 'react-router';

export function ExecutionDebuggerStubPage() {
  const { id } = useParams<{ id?: string }>();
  return (
    <Center py="xl" style={{ minHeight: 400 }} data-testid="executions-debugger-stub">
      <Stack align="center" gap="xs">
        <Title order={3}>Execution debugger</Title>
        <Text c="dimmed" size="sm">
          Debugger surface for run #{id ?? '(missing id)'} ships in Feature #95.
        </Text>
      </Stack>
    </Center>
  );
}
