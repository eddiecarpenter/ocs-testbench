import { ActionIcon, Box, Text, Textarea } from '@mantine/core';
import { IconPlayerStopFilled, IconSend } from '@tabler/icons-react';
import { type KeyboardEvent, useRef, useState } from 'react';

import type { SlashCommand } from './slashCommands';

// ─── Props ────────────────────────────────────────────────────────────────────

export interface ComposeBarProps {
  /**
   * Whether the bar is disabled.  True while a stream is in flight or a
   * permission prompt is active.
   */
  disabled: boolean;
  /**
   * Whether a permission prompt is currently active.  Drives the
   * placeholder text: "Waiting for tool approval…" vs the default.
   */
  awaitingPermission: boolean;
  /** True while the AI is actively streaming — shows Stop instead of Send. */
  streaming: boolean;
  /** Called with the trimmed message text when the user submits. */
  onSubmit: (message: string) => void;
  /** Called when the user clicks Stop during an active stream. */
  onStop: () => void;
  /** Optional slash commands surfaced when the user types "/". */
  commands?: SlashCommand[];
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Compose bar for the AI Assistant chat.
 *
 * - Enter submits; Shift+Enter inserts a newline.
 * - Typing "/" opens a command palette filtered by what follows.
 * - Disabled while a stream is in flight or a permission prompt is active.
 * - Clears on submit.
 */
export function ComposeBar({ disabled, awaitingPermission, streaming, onSubmit, onStop, commands = [] }: ComposeBarProps) {
  const [value, setValue] = useState('');
  const [selectedIdx, setSelectedIdx] = useState(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const placeholder = awaitingPermission
    ? 'Waiting for tool approval…'
    : 'Ask the AI assistant… (type / for commands)';

  // ── Command palette logic ────────────────────────────────────────────────

  const isSlashMode = value.startsWith('/');
  const query = isSlashMode ? value.slice(1).toLowerCase() : '';
  const filtered = isSlashMode
    ? commands.filter((c) => c.name.toLowerCase().startsWith(query))
    : [];

  const paletteVisible = isSlashMode && filtered.length > 0;
  const clampedIdx = Math.min(selectedIdx, filtered.length - 1);

  function runCommand(cmd: SlashCommand) {
    setValue('');
    setSelectedIdx(0);
    cmd.onRun();
  }

  // ── Submit / keyboard ────────────────────────────────────────────────────

  function handleSubmit() {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;

    // If the user typed exactly a matching command and hit Enter, run it.
    if (paletteVisible) {
      const exact = filtered[clampedIdx];
      if (exact) {
        runCommand(exact);
        return;
      }
    }

    setValue('');
    setSelectedIdx(0);
    onSubmit(trimmed);
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (paletteVisible) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIdx((i) => Math.min(i + 1, filtered.length - 1));
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIdx((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setValue('');
        setSelectedIdx(0);
        return;
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  }

  function handleChange(v: string) {
    setValue(v);
    // Reset selection index whenever the query changes.
    setSelectedIdx(0);
  }

  return (
    <Box
      px="md"
      py="sm"
      style={{
        borderTop:
          '1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4))',
      }}
    >
      <Box style={{ position: 'relative' }}>
        {/* ── Command palette ─────────────────────────────────────────── */}
        {paletteVisible && (
          <Box
            style={{
              position: 'absolute',
              bottom: '100%',
              left: 0,
              right: 44,
              marginBottom: 4,
              backgroundColor: 'light-dark(var(--mantine-color-white), var(--mantine-color-dark-7))',
              border: '1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4))',
              borderRadius: 'var(--mantine-radius-sm)',
              boxShadow: 'var(--mantine-shadow-sm)',
              overflow: 'hidden',
              zIndex: 10,
            }}
          >
            {filtered.map((cmd, i) => (
              <Box
                key={cmd.name}
                px="sm"
                py={6}
                onClick={() => runCommand(cmd)}
                style={{
                  cursor: 'pointer',
                  backgroundColor:
                    i === clampedIdx
                      ? 'light-dark(var(--mantine-color-blue-0), var(--mantine-color-dark-5))'
                      : 'transparent',
                  display: 'flex',
                  alignItems: 'baseline',
                  gap: 10,
                }}
                onMouseEnter={() => setSelectedIdx(i)}
              >
                <Text size="sm" fw={600} c="blue" style={{ fontFamily: 'monospace', minWidth: 80 }}>
                  /{cmd.name}
                </Text>
                <Text size="xs" c="dimmed">
                  {cmd.description}
                </Text>
              </Box>
            ))}
          </Box>
        )}

        {/* ── Textarea ────────────────────────────────────────────────── */}
        <Textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => handleChange(e.currentTarget.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          autosize
          minRows={1}
          maxRows={6}
          styles={{
            input: {
              paddingRight: 44,
              resize: 'none',
            },
          }}
          aria-label="Message input"
        />

        {/* ── Send / Stop button ──────────────────────────────────────── */}
        {streaming ? (
          <ActionIcon
            variant="filled"
            color="red"
            size="md"
            radius="sm"
            onClick={onStop}
            aria-label="Stop generation"
            style={{ position: 'absolute', bottom: 6, right: 6 }}
          >
            <IconPlayerStopFilled size={14} />
          </ActionIcon>
        ) : (
          <ActionIcon
            variant="filled"
            size="md"
            radius="sm"
            disabled={disabled || !value.trim()}
            onClick={handleSubmit}
            aria-label="Send message"
            style={{ position: 'absolute', bottom: 6, right: 6 }}
          >
            <IconSend size={14} />
          </ActionIcon>
        )}
      </Box>
    </Box>
  );
}
