import {
  Anchor,
  Badge,
  Box,
  Card,
  Divider,
  Group,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import {
  IconBrandGithub,
  IconCircleCheck,
  IconRouter,
} from '@tabler/icons-react';

import { useVersion } from '../../api/resources/version';

const TECH_STACK = [
  'Diameter Gy (RFC 4006)',
  'Go + pgx / PostgreSQL',
  'React + Mantine',
  'Wails v2 (macOS)',
];

/**
 * AboutPage — shown when the user selects OCS Testbench → About OCS Testbench
 * from the macOS menu bar. Intentionally minimal: identity, purpose, stack.
 */
export function AboutPage() {
  const { data: versionInfo, isLoading } = useVersion();

  return (
    <Stack gap="lg" p="md" maw={560}>
      {/* ── Identity ─────────────────────────────────────────── */}
      <Group gap="lg" align="center" wrap="nowrap">
        <Box
          w={64}
          h={64}
          style={{
            borderRadius: 16,
            background: 'var(--mantine-color-brand-5)',
            flexShrink: 0,
          }}
        />
        <Stack gap={4}>
          <Title order={2} fw={700} lh={1.2}>
            OCS Testbench
          </Title>
          <Text size="sm" c="dimmed">
            Diameter Gy Credit-Control testing tool
          </Text>
          <Group gap="xs" mt={2}>
            <Badge variant="light" color="brand" radius="sm" w="fit-content">
              Charging Trigger Function (CTF)
            </Badge>
            {isLoading ? (
              <Skeleton height={20} width={60} radius="sm" />
            ) : (
              <Badge variant="outline" color="gray" radius="sm" w="fit-content">
                {versionInfo?.version ?? 'dev'}
              </Badge>
            )}
          </Group>
        </Stack>
      </Group>

      <Divider />

      {/* ── Purpose ──────────────────────────────────────────── */}
      <Card padding="lg" withBorder shadow="xs">
        <Stack gap="sm">
          <Text size="xs" fw={600} c="dimmed" tt="uppercase" style={{ letterSpacing: 0.5 }}>
            What it does
          </Text>
          <Text size="sm">
            OCS Testbench acts as a Diameter{' '}
            <Text span fw={600}>Charging Trigger Function</Text> — it sends
            Credit-Control-Request (CCR) messages to an Online Charging System
            and processes the Credit-Control-Answer (CCA) responses. It is a
            traffic-generation and test tool, not an OCS implementation.
          </Text>
          <Text size="sm" c="dimmed">
            Supports multi-peer connections, scenario authoring, interactive
            step-through, and real-time SSE event streaming.
          </Text>
        </Stack>
      </Card>

      {/* ── Tech stack ───────────────────────────────────────── */}
      <Card padding="lg" withBorder shadow="xs">
        <Stack gap="sm">
          <Text size="xs" fw={600} c="dimmed" tt="uppercase" style={{ letterSpacing: 0.5 }}>
            Built with
          </Text>
          <Stack gap="xs">
            {TECH_STACK.map((item) => (
              <Group key={item} gap="sm" wrap="nowrap">
                <IconCircleCheck size={15} color="var(--mantine-color-teal-5)" />
                <Text size="sm">{item}</Text>
              </Group>
            ))}
          </Stack>
        </Stack>
      </Card>

      {/* ── Links ────────────────────────────────────────────── */}
      <Group gap="md">
        <Anchor
          href="https://github.com/eddiecarpenter/ocs-testbench"
          target="_blank"
          size="sm"
        >
          <Group gap={6} wrap="nowrap">
            <IconBrandGithub size={15} />
            <Text size="sm">GitHub</Text>
          </Group>
        </Anchor>
        <Anchor
          href="https://datatracker.ietf.org/doc/html/rfc4006"
          target="_blank"
          size="sm"
          c="dimmed"
        >
          <Group gap={6} wrap="nowrap">
            <IconRouter size={15} />
            <Text size="sm">RFC 4006</Text>
          </Group>
        </Anchor>
      </Group>
    </Stack>
  );
}
