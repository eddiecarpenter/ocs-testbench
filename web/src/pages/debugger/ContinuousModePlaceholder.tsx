/**
 * Placeholder route for `mode === 'continuous'` executions.
 *
 * Continuous batches don't have a step cursor — the three-pane shell
 * is Interactive-only in MVP per the design rationale. The follow-up
 * Feature will replace this with a batch-progress view; for now the
 * placeholder keeps the route navigable from the Executions list
 * without crashing.
 */
import { Anchor, Center, Stack, Text, Title } from '@mantine/core';
import { Link } from 'react-router';

interface ContinuousModePlaceholderProps {
  executionId: string;
}

export function ContinuousModePlaceholder({
  executionId,
}: ContinuousModePlaceholderProps) {
  return (
    <Center py="xl" data-testid="executions-debugger-continuous">
      <Stack align="center" gap="xs" maw={520}>
        <Title order={3}>Continuous-run debugger</Title>
        <Text c="dimmed" size="sm" ta="center">
          Run #{executionId} is a continuous-mode batch. The three-pane
          step-by-step debugger only applies to interactive runs;
          batch progress is rendered on the Executions list while a
          dedicated batch-progress view ships in a follow-up Feature.
        </Text>
        <Anchor component={Link} to="/executions">
          ← Back to executions
        </Anchor>
      </Stack>
    </Center>
  );
}
