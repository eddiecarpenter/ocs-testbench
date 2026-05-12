import { ActionIcon, Box, Textarea } from '@mantine/core';
import { IconSend } from '@tabler/icons-react';
import { type KeyboardEvent, useRef, useState } from 'react';

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
  /** Called with the trimmed message text when the user submits. */
  onSubmit: (message: string) => void;
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Compose bar for the AI Assistant chat.
 *
 * - Enter submits; Shift+Enter inserts a newline.
 * - Disabled while a stream is in flight or a permission prompt is active.
 * - Placeholder text switches to "Waiting for tool approval…" while the
 *   prompt is active.
 * - Clears on submit.
 */
export function ComposeBar({ disabled, awaitingPermission, onSubmit }: ComposeBarProps) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const placeholder = awaitingPermission
    ? 'Waiting for tool approval…'
    : 'Ask the AI assistant…';

  function handleSubmit() {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    setValue('');
    onSubmit(trimmed);
  }

  function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
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
        <Textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => setValue(e.currentTarget.value)}
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
        <ActionIcon
          variant="filled"
          size="md"
          radius="sm"
          disabled={disabled || !value.trim()}
          onClick={handleSubmit}
          aria-label="Send message"
          style={{
            position: 'absolute',
            bottom: 6,
            right: 6,
          }}
        >
          <IconSend size={14} />
        </ActionIcon>
      </Box>
    </Box>
  );
}
