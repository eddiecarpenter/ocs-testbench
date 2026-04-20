import { Card, Group, Stack, Text, ThemeIcon, Title } from '@mantine/core';
import { IconCheck, IconPlayerPlay, IconX } from '@tabler/icons-react';
import type {
  ExecutionResult,
  ExecutionSummary,
} from '../../types/dashboard';

interface RecentExecutionsCardProps {
  executions: ExecutionSummary[];
}

const resultColor: Record<ExecutionResult, string> = {
  success: 'teal',
  failure: 'red',
  running: 'blue',
};

function ResultIcon({ result }: { result: ExecutionResult }) {
  const Icon =
    result === 'success' ? IconCheck : result === 'failure' ? IconX : IconPlayerPlay;
  return (
    <ThemeIcon color={resultColor[result]} variant="subtle" size="sm">
      <Icon size={14} stroke={2.5} />
    </ThemeIcon>
  );
}

export function RecentExecutionsCard({ executions }: RecentExecutionsCardProps) {
  return (
    <Card padding="lg" withBorder shadow="xs" h="100%">
      <Stack gap="md">
        <Title order={5} fw={600}>
          Recent Executions
        </Title>
        <Stack gap="sm">
          {executions.map((exec) => (
            <Group
              key={exec.id}
              justify="space-between"
              align="center"
              wrap="nowrap"
            >
              <Group gap="sm" wrap="nowrap">
                <ResultIcon result={exec.result} />
                <Stack gap={0}>
                  <Text size="sm" fw={500}>
                    {exec.name}
                  </Text>
                  <Text size="xs" c="dimmed">
                    {exec.mode} · {exec.peer}
                  </Text>
                </Stack>
              </Group>
              <Text size="sm" c="dimmed">
                {exec.relativeTime}
              </Text>
            </Group>
          ))}
        </Stack>
      </Stack>
    </Card>
  );
}
