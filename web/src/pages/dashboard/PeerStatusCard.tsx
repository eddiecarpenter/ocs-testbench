import { Card, Group, Stack, Text, Title } from '@mantine/core';
import type { PeerStatus, PeerSummary } from '../../types/dashboard';

interface PeerStatusCardProps {
  peers: PeerSummary[];
}

const statusColor: Record<PeerStatus, string> = {
  connected: 'var(--mantine-color-teal-6)',
  disconnected: 'var(--mantine-color-gray-5)',
  error: 'var(--mantine-color-red-6)',
  connecting: 'var(--mantine-color-yellow-6)',
};

const statusLabel: Record<PeerStatus, string> = {
  connected: 'connected',
  disconnected: 'disconnected',
  error: 'CER/CEA timeout',
  connecting: 'connecting',
};

function StatusDot({ status }: { status: PeerStatus }) {
  return (
    <span
      aria-label={statusLabel[status]}
      style={{
        display: 'inline-block',
        width: 8,
        height: 8,
        borderRadius: '50%',
        background: statusColor[status],
      }}
    />
  );
}

export function PeerStatusCard({ peers }: PeerStatusCardProps) {
  return (
    <Card padding="lg" withBorder shadow="xs" h="100%">
      <Stack gap="md">
        <Title order={5} fw={600}>
          Peer Status
        </Title>
        <Stack gap="sm">
          {peers.map((peer) => (
            <Group key={peer.id} justify="space-between" align="center" wrap="nowrap">
              <Group gap="sm" wrap="nowrap">
                <StatusDot status={peer.status} />
                <Stack gap={0}>
                  <Text size="sm" fw={500}>
                    {peer.name}
                  </Text>
                  <Text size="xs" c="dimmed">
                    {peer.detail}
                  </Text>
                </Stack>
              </Group>
              <Text size="sm" c="dimmed">
                {statusLabel[peer.status]}
              </Text>
            </Group>
          ))}
        </Stack>
      </Stack>
    </Card>
  );
}
