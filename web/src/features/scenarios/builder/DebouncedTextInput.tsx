/**
 * Text inputs that commit only on blur, so the history middleware
 * records one snapshot per coherent edit instead of one per keystroke.
 *
 * Internally tracks a local string while the user is typing; on blur
 * (or Enter for the single-line input) calls `onCommit` with the
 * final value. The Mantine label / description / error / placeholder /
 * `data-testid` props pass through.
 */
import { TextInput, Textarea } from '@mantine/core';
import { useState } from 'react';

interface DebouncedTextInputProps {
  label?: string;
  description?: string;
  placeholder?: string;
  error?: string | null;
  value: string;
  onCommit: (next: string) => void;
  'data-testid'?: string;
}

export function DebouncedTextInput({
  label,
  description,
  placeholder,
  error,
  value,
  onCommit,
  'data-testid': testId,
}: DebouncedTextInputProps) {
  // Local state mirrors the prop while the user types. To keep the
  // mirror in sync after external changes (Discard / Undo / load) we
  // adjust state during render — see React docs "Adjusting state when
  // a prop changes" — rather than calling setState inside an effect.
  const [local, setLocal] = useState(value);
  const [lastSyncedValue, setLastSyncedValue] = useState(value);
  if (value !== lastSyncedValue) {
    setLastSyncedValue(value);
    setLocal(value);
  }

  return (
    <TextInput
      label={label}
      description={description}
      placeholder={placeholder}
      error={error ?? undefined}
      value={local}
      onChange={(e) => setLocal(e.currentTarget.value)}
      onBlur={() => {
        if (local !== value) onCommit(local);
      }}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.currentTarget.blur();
        }
      }}
      data-testid={testId}
    />
  );
}

interface DebouncedTextareaProps {
  label?: string;
  description?: string;
  placeholder?: string;
  error?: string | null;
  value: string;
  onCommit: (next: string) => void;
  minRows?: number;
  maxRows?: number;
  'data-testid'?: string;
}

export function DebouncedTextarea({
  label,
  description,
  placeholder,
  error,
  value,
  onCommit,
  minRows,
  maxRows,
  'data-testid': testId,
}: DebouncedTextareaProps) {
  const [local, setLocal] = useState(value);
  const [lastSyncedValue, setLastSyncedValue] = useState(value);
  if (value !== lastSyncedValue) {
    setLastSyncedValue(value);
    setLocal(value);
  }

  return (
    <Textarea
      label={label}
      description={description}
      placeholder={placeholder}
      error={error ?? undefined}
      autosize
      minRows={minRows}
      maxRows={maxRows}
      value={local}
      onChange={(e) => setLocal(e.currentTarget.value)}
      onBlur={() => {
        if (local !== value) onCommit(local);
      }}
      data-testid={testId}
    />
  );
}
