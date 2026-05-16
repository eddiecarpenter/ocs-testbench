import { Box, Group, Loader, Stack, Text, Title } from '@mantine/core';
import { useLocalStorage } from '@mantine/hooks';
import { useEffect, useState } from 'react';

import { ChatThread } from './ChatThread';
import { ComposeBar } from './ComposeBar';
import { PermissionPrompt } from './PermissionPrompt';
import { useChatStore } from './chatStore';
import type { SlashCommand } from './slashCommands';
import { useChatSse } from './useChatSse';

/**
 * Streaming status bar.
 *
 * While streaming: animated dots loader + live elapsed seconds + token count.
 * After streaming ends: ✓ checkmark + final elapsed time + token count.
 * Hidden when no stream has run yet (streamStartedAt === null).
 */
function StreamingIndicator() {
  const streaming = useChatStore((s) => s.streaming);
  const streamStartedAt = useChatStore((s) => s.streamStartedAt);
  const streamEndedAt = useChatStore((s) => s.streamEndedAt);
  const inputTokens = useChatStore((s) => s.inputTokens);
  const outputTokens = useChatStore((s) => s.outputTokens);
  const outputChars = useChatStore((s) => s.outputChars);
  // During streaming: live output estimate (chars ÷ 4) + real input count
  // once usage_update fires. After done: accurate figures from the LLM report.
  const liveOutputTokens = streaming ? Math.ceil(outputChars / 4) : outputTokens;
  const liveInputTokens  = inputTokens; // updated live via usage_update
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (streamStartedAt === null) { setElapsed(0); return; } // eslint-disable-line react-hooks/set-state-in-effect
    if (!streaming) {
      const final = streamEndedAt !== null
        ? Math.floor((streamEndedAt - streamStartedAt) / 1000)
        : elapsed;
      setElapsed(final);
      return;
    }
    const id = setInterval(() => {
      setElapsed(Math.floor((Date.now() - streamStartedAt) / 1000));
    }, 1000);
    return () => clearInterval(id);
  }, [streaming, streamStartedAt, streamEndedAt]); // eslint-disable-line react-hooks/exhaustive-deps

  // Nothing to show before any stream has run.
  if (streamStartedAt === null) return null;

  return (
    <Group
      px="md"
      py={6}
      gap="xs"
      align="center"
      style={{
        borderTop: '1px solid var(--mantine-color-gray-2)',
        backgroundColor: 'var(--mantine-color-gray-0)',
      }}
    >
      {streaming ? (
        <>
          <Loader size={14} type="dots" />
          <Text size="xs" c="dimmed">
            {elapsed}s{liveInputTokens > 0 ? ` · ↑${liveInputTokens}` : ''} · ↓~{liveOutputTokens} tokens
          </Text>
        </>
      ) : (
        <>
          <Text size="xs" c="teal" fw={600}>✓</Text>
          <Text size="xs" c="dimmed">{elapsed}s · ↑{inputTokens} ↓{outputTokens} tokens</Text>
        </>
      )}
    </Group>
  );
}

/**
 * AI Assistant page — assembles the chat thread, permission prompt,
 * and compose bar wired to the mock SSE backend (Feature #177).
 *
 * Feature 3 replaces the mock SSE endpoint with a real LLM runtime;
 * no frontend changes are needed on that swap.
 */
export function AiAssistantPage() {
  const thread = useChatStore((s) => s.thread);
  const streaming = useChatStore((s) => s.streaming);
  const pendingPrompt = useChatStore((s) => s.pendingPrompt);
  const togglePlanning = useChatStore((s) => s.togglePlanning);
  const denyPermission = useChatStore((s) => s.denyPermission);
  const approveOnce = useChatStore((s) => s.approveOnce);
  const approveAlways = useChatStore((s) => s.approveAlways);
  const reset = useChatStore((s) => s.reset);

  const { sendMessage, abort } = useChatSse();

  const [hideToolCalls] = useLocalStorage({ key: 'ai-hide-tool-calls', defaultValue: false });

  const composeDisabled = streaming || pendingPrompt !== null;
  const awaitingPermission = pendingPrompt !== null;

  function handleDeny(callId: string) {
    abort();
    denyPermission(callId);
  }

  function handleSubmit(message: string) {
    void sendMessage(message);
  }

  const slashCommands: SlashCommand[] = [
    {
      name: 'clear',
      description: 'Clear the conversation and start fresh',
      onRun: reset,
    },
  ];

  return (
    <Stack gap={0} style={{ height: 'calc(100vh - 60px)' }}>
      {/* Page header — visible when thread is empty */}
      {thread.length === 0 && (
        <Box p="md" pb={0}>
          <Stack gap={4}>
            <Title order={2} fw={600}>
              AI Assistant
            </Title>
            <Text c="dimmed" size="sm">
              Chat with the AI assistant to diagnose faults and verify charging
              configuration.
            </Text>
          </Stack>
        </Box>
      )}

      {/* Chat thread — fills remaining space */}
      <ChatThread items={thread} onTogglePlanning={togglePlanning} hideToolCalls={hideToolCalls} />

      {/* Busy indicator — shown while the agent is streaming */}
      <StreamingIndicator />

      {/* Permission prompt — pinned above compose bar when active */}
      {pendingPrompt !== null && (
        <PermissionPrompt
          prompt={pendingPrompt}
          onDeny={handleDeny}
          onOnce={approveOnce}
          onAlways={approveAlways}
        />
      )}

      {/* Compose bar */}
      <ComposeBar
        disabled={composeDisabled}
        awaitingPermission={awaitingPermission}
        streaming={streaming}
        onSubmit={handleSubmit}
        onStop={abort}
        commands={slashCommands}
      />
    </Stack>
  );
}
