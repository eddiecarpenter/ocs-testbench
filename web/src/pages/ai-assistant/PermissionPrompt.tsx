import { Box, Button, Group, Stack, Text } from '@mantine/core';

import type { PermissionPromptState, PermissionTier } from './types';

// ─── Tier colour map ──────────────────────────────────────────────────────────

const TIER_ACCENT: Record<PermissionTier, string> = {
  readonly: '#228BE6',
  write: '#F08C00',
  destructive: '#E03131',
};

const TIER_FILL: Record<PermissionTier, string> = {
  readonly: '#EBF4FF',
  write: '#FFF4E6',
  destructive: '#FFF0F0',
};

// ─── Component ────────────────────────────────────────────────────────────────

export interface PermissionPromptProps {
  prompt: PermissionPromptState;
  onDeny: (callId: string) => void;
  onOnce: (callId: string) => void;
  onAlways: (callId: string) => void;
}

/**
 * Full-width permission prompt panel.
 *
 * Pinned just above the compose bar when a write or destructive tool call
 * requires user approval. Three actions: Deny (aborts stream), Once
 * (approves this invocation only), Always (suppresses future prompts for
 * this tool for the session).
 *
 * Design: left accent bar in tier colour, tool name + description text,
 * action buttons right-aligned.
 */
export function PermissionPrompt({
  prompt,
  onDeny,
  onOnce,
  onAlways,
}: PermissionPromptProps) {
  const accent = TIER_ACCENT[prompt.tier];
  const fill = TIER_FILL[prompt.tier];

  return (
    <Box
      px="md"
      py="sm"
      style={{
        backgroundColor: fill,
        borderLeft: `4px solid ${accent}`,
        borderTop: `1px solid ${accent}33`,
      }}
    >
      <Group justify="space-between" align="center" wrap="nowrap" gap="md">
        {/* Left: tool info */}
        <Stack gap={2} style={{ minWidth: 0 }}>
          <Text size="sm" fw={600} style={{ color: accent }} truncate>
            {prompt.toolName}
          </Text>
          <Text size="xs" c="dimmed" truncate>
            {prompt.description}
          </Text>
        </Stack>

        {/* Right: action buttons */}
        <Group gap="xs" wrap="nowrap" style={{ flexShrink: 0 }}>
          <Button
            size="xs"
            variant="light"
            color="red"
            onClick={() => onDeny(prompt.callId)}
          >
            Deny
          </Button>
          <Button
            size="xs"
            variant="light"
            color="orange"
            onClick={() => onOnce(prompt.callId)}
          >
            Once
          </Button>
          <Button
            size="xs"
            variant="filled"
            color="green"
            onClick={() => onAlways(prompt.callId)}
          >
            Always
          </Button>
        </Group>
      </Group>
    </Box>
  );
}
