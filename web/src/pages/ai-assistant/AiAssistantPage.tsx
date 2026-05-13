import { Box, Stack, Text, Title } from '@mantine/core';

import { ChatThread } from './ChatThread';
import { ComposeBar } from './ComposeBar';
import { PermissionPrompt } from './PermissionPrompt';
import { useChatStore } from './chatStore';
import { useChatSse } from './useChatSse';

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

  const { sendMessage, abort } = useChatSse();

  const composeDisabled = streaming || pendingPrompt !== null;
  const awaitingPermission = pendingPrompt !== null;

  function handleDeny(callId: string) {
    abort();
    denyPermission(callId);
  }

  function handleSubmit(message: string) {
    void sendMessage(message);
  }

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
      <ChatThread items={thread} onTogglePlanning={togglePlanning} />

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
        onSubmit={handleSubmit}
      />
    </Stack>
  );
}
