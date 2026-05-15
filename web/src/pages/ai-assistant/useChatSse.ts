/**
 * useChatSse — hook that opens a POST SSE connection to /api/v1/ai/chat
 * and feeds events into the chat store.
 *
 * The AI chat endpoint uses POST (not GET) so we can't use EventSource.
 * We use fetch with response body streaming instead.
 */
import { useCallback, useRef } from 'react';

import { useChatStore } from './chatStore';
import type { ChatRequest, SseEvent } from './types';

/** Parse a single SSE "data:" line into an SseEvent. Returns null on failure. */
function parseDataLine(line: string): SseEvent | null {
  const prefix = 'data: ';
  if (!line.startsWith(prefix)) return null;
  try {
    return JSON.parse(line.slice(prefix.length)) as SseEvent;
  } catch {
    return null;
  }
}

/**
 * Returns a `sendMessage` function.  Calling it opens the SSE stream,
 * appends the user message to the thread, and processes each incoming
 * event until the stream ends or is aborted.
 *
 * The hook holds an AbortController so callers can abort mid-stream
 * (e.g. on Deny).
 */
export function useChatSse() {
  const store = useChatStore;
  const abortRef = useRef<AbortController | null>(null);

  const sendMessage = useCallback(
    async (userText: string) => {
      const {
        pushUserMessage,
        startStream,
        endStream,
        appendError,
        applyEvent,
        thread,
      } = store.getState();

      // Abort any in-flight stream.
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      pushUserMessage(userText);
      startStream();

      // Build the conversation history for the request body.
      // Aborted items stay in the thread for display but are excluded here.
      const messages: ChatRequest['messages'] = thread
        .filter((i) => (i.kind === 'user' || i.kind === 'assistant') && !i.aborted)
        .map((i) => ({
          role: i.kind as 'user' | 'assistant',
          content: i.kind === 'user' || i.kind === 'assistant' ? i.content : '',
        }));
      // Append the new user message (already pushed to store).
      messages.push({ role: 'user', content: userText });

      try {
        const response = await fetch('/api/v1/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ messages } satisfies ChatRequest),
          signal: controller.signal,
        });

        if (!response.ok || !response.body) {
          appendError(`Request failed: ${response.status} ${response.statusText}`);
          return;
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });

          // SSE messages are delimited by double newlines.
          const parts = buffer.split('\n\n');
          buffer = parts.pop() ?? '';

          for (const part of parts) {
            const lines = part.split('\n');
            for (const line of lines) {
              const event = parseDataLine(line);
              if (event) {
                applyEvent(event);
                if (event.type === 'done') {
                  reader.cancel();
                  endStream();
                  return;
                }
              }
            }
          }
        }

        endStream();
      } catch (err) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          endStream();
          return;
        }
        appendError(err instanceof Error ? err.message : 'Unknown error');
      }
    },
    [store],
  );

  const abort = useCallback(() => {
    abortRef.current?.abort();
    store.getState().abortStream();
  }, [store]);

  return { sendMessage, abort };
}
