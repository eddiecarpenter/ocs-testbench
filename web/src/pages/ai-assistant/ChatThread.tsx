import { Box, Group, Stack, Text } from '@mantine/core';
import { useEffect, useRef } from 'react';

import type {
  AssistantMessage,
  PlanningBlock,
  ThreadItem,
  ToolCall,
  ToolCallBlock,
  UserMessage,
} from './types';

// ─── Colour constants (per design rationale) ─────────────────────────────────

/** User message bubble background — #228BE6 Mantine blue. */
const USER_BG = '#228BE6';

/** Planning block accent / text colour. */
const PLANNING_ACCENT = '#7048E8';
/** Planning block fill. */
const PLANNING_FILL = '#F3F0FF';

/** Tool call backgrounds keyed by tier. */
const TOOL_BG: Record<string, string> = {
  readonly: '#EBF4FF',
  write: '#FFF4E6',
  destructive: '#FFF0F0',
  running: '#EBFBEE',
};

/** Tool call border/accent keyed by tier. */
const TOOL_ACCENT: Record<string, string> = {
  readonly: '#228BE6',
  write: '#F08C00',
  destructive: '#E03131',
  running: '#2F9E44',
};

// ─── Individual message renderers ─────────────────────────────────────────────

function UserBubble({ msg }: { msg: UserMessage }) {
  return (
    <Group justify="flex-end" align="flex-end">
      <Box
        maw="70%"
        px="md"
        py="sm"
        style={{
          backgroundColor: USER_BG,
          borderRadius: 14,
          borderBottomRightRadius: 4,
        }}
      >
        <Text size="sm" c="white" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
          {msg.content}
        </Text>
      </Box>
    </Group>
  );
}

function AssistantBubble({ msg }: { msg: AssistantMessage }) {
  return (
    <Box maw="85%">
      <Text size="sm" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', lineHeight: 1.7 }}>
        {msg.content}
        {/* Blinking cursor while streaming (content ends without a trailing space) */}
      </Text>
    </Box>
  );
}

function PlanningBlockItem({
  block,
  onToggle,
}: {
  block: PlanningBlock;
  onToggle: (id: string) => void;
}) {
  const preview = block.content.split('\n')[0]?.slice(0, 80) ?? '';

  return (
    <Box
      maw="85%"
      mb={4}
      style={{
        backgroundColor: PLANNING_FILL,
        borderLeft: `3px solid ${PLANNING_ACCENT}`,
        borderRadius: 6,
        overflow: 'hidden',
      }}
    >
      {/* Header row — always visible */}
      <Group
        px="sm"
        py={6}
        gap="xs"
        align="center"
        style={{ cursor: 'pointer', userSelect: 'none' }}
        onClick={() => onToggle(block.id)}
      >
        <Text
          size="xs"
          fw={600}
          c={PLANNING_ACCENT}
          style={{ letterSpacing: 0.3 }}
        >
          {block.collapsed ? '›' : '⌄'} Thinking
        </Text>
        {block.collapsed && (
          <Text size="xs" c="dimmed" truncate>
            {preview}
          </Text>
        )}
      </Group>

      {/* Expanded body */}
      {!block.collapsed && (
        <Box px="sm" pb="sm">
          <Text
            size="xs"
            c="dimmed"
            style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}
          >
            {block.content}
          </Text>
        </Box>
      )}
    </Box>
  );
}

function tierLabel(call: ToolCall): string {
  if (call.status === 'running') return 'Running…';
  if (call.status === 'denied') return 'Denied';
  if (call.status === 'done') return 'Done';
  return call.tier === 'readonly'
    ? 'Read-only'
    : call.tier === 'write'
    ? 'Write'
    : 'Destructive';
}

function ToolCallItem({ block }: { block: ToolCallBlock }) {
  const { call } = block;
  const effectiveTier = call.status === 'running' ? 'running' : call.tier;
  const bg = TOOL_BG[effectiveTier] ?? TOOL_BG.readonly;
  const accent = TOOL_ACCENT[effectiveTier] ?? TOOL_ACCENT.readonly;

  return (
    <Box
      maw="85%"
      mb={4}
      style={{
        backgroundColor: bg,
        borderLeft: `3px solid ${accent}`,
        borderRadius: 6,
        marginLeft: 16,
      }}
    >
      <Group px="sm" py={6} justify="space-between" align="center" wrap="nowrap">
        <Stack gap={0}>
          <Text size="xs" fw={600} style={{ color: accent }}>
            {call.name}
          </Text>
          <Text size="xs" c="dimmed">
            {call.description}
          </Text>
        </Stack>
        <Text
          size="xs"
          fw={500}
          style={{
            color: accent,
            whiteSpace: 'nowrap',
            fontSize: 11,
          }}
        >
          {tierLabel(call)}
        </Text>
      </Group>

      {/* Inline result */}
      {call.result !== null && call.status === 'done' && (
        <Box
          px="sm"
          pb="xs"
          style={{
            borderTop: `1px solid ${accent}22`,
          }}
        >
          <Text
            size="xs"
            c="dimmed"
            style={{
              fontFamily: 'monospace',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            {call.result}
          </Text>
        </Box>
      )}
    </Box>
  );
}

// ─── Thread container ─────────────────────────────────────────────────────────

export interface ChatThreadProps {
  items: ThreadItem[];
  onTogglePlanning: (id: string) => void;
}

/**
 * Scrollable chat thread container. Renders all message types per
 * the design rationale from Feature #177.
 *
 * Scrolls to the bottom whenever `items` changes so new tokens
 * stay visible during streaming.
 */
export function ChatThread({ items, onTogglePlanning }: ChatThreadProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [items]);

  return (
    <Box
      style={{
        flex: 1,
        overflowY: 'auto',
        paddingInline: 'var(--mantine-spacing-md)',
        paddingBlock: 'var(--mantine-spacing-sm)',
      }}
    >
      <Stack gap="sm">
        {items.map((item) => {
          switch (item.kind) {
            case 'user':
              return <UserBubble key={item.id} msg={item} />;
            case 'assistant':
              return <AssistantBubble key={item.id} msg={item} />;
            case 'planning':
              return (
                <PlanningBlockItem
                  key={item.id}
                  block={item}
                  onToggle={onTogglePlanning}
                />
              );
            case 'tool_call':
              return <ToolCallItem key={item.id} block={item} />;
          }
        })}
        <div ref={bottomRef} />
      </Stack>
    </Box>
  );
}
