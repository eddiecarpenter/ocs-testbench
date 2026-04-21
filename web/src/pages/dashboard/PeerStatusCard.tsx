import { Card, Group, Stack, Text, Title } from '@mantine/core';

import type { Peer } from '../../api/resources/peers';
import { STATUS_LABEL } from '../../components/peer/peerStatus';
import { StatusDot } from '../../components/peer/PeerStatusLabel';

interface PeerStatusCardProps {
  peers: Peer[];
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
            <Group
              key={peer.id}
              justify="space-between"
              align="center"
              wrap="nowrap"
            >
              <Group gap="sm" wrap="nowrap">
                <StatusDot status={peer.status} />
                <Stack gap={0}>
                  <Text size="sm" fw={500}>
                    {peer.name}
                  </Text>
                  <Text size="xs" c="dimmed">
                    {peer.host}:{peer.port} · {peer.originHost}
                  </Text>
                </Stack>
              </Group>
              <Text size="sm" c="dimmed">
                {peer.statusDetail ?? STATUS_LABEL[peer.status]}
              </Text>
            </Group>
          ))}
        </Stack>
      </Stack>
    </Card>
  );
}
