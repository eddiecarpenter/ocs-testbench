/**
 * ExpressionInput — a text input for ruleevaluator expressions with an
 * inline "Test" button that evaluates the expression against a sample
 * variable map via POST /expressions/evaluate.
 *
 * Displays the result (true / false / value / error) inline below the input.
 * Both {{VAR}} and bare VAR notation are accepted — the engine normalises them.
 */
import { ActionIcon, Box, Code, Group, Text, TextInput, Tooltip } from '@mantine/core';
import { IconFlask } from '@tabler/icons-react';
import { useState } from 'react';

import { ApiService } from '../../../api/ApiService';

interface ExpressionInputProps {
  label: string;
  description?: string;
  placeholder?: string;
  value: string;
  onChange: (value: string) => void;
  /** Variable map used when testing the expression. */
  testVars?: Record<string, unknown>;
  'data-testid'?: string;
}

interface EvaluateResponse {
  result?: unknown;
  error?: string;
}

type TestState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; result: unknown }
  | { status: 'error'; message: string };

export function ExpressionInput({
  label,
  description,
  placeholder,
  value,
  onChange,
  testVars = {},
  'data-testid': testId,
}: ExpressionInputProps) {
  const [test, setTest] = useState<TestState>({ status: 'idle' });

  const handleTest = async () => {
    if (!value.trim()) return;
    setTest({ status: 'loading' });
    try {
      const res = await ApiService.post<EvaluateResponse>('/expressions/evaluate', {
        expression: value,
        vars: testVars,
      });
      if (res.error) {
        setTest({ status: 'error', message: res.error });
      } else {
        setTest({ status: 'ok', result: res.result });
      }
    } catch (err) {
      setTest({ status: 'error', message: (err as Error).message });
    }
  };

  const resultColor =
    test.status === 'ok'
      ? String(test.result) === 'true'
        ? 'green'
        : 'orange'
      : test.status === 'error'
        ? 'red'
        : 'dimmed';

  const resultText =
    test.status === 'ok'
      ? String(test.result)
      : test.status === 'error'
        ? test.message
        : null;

  return (
    <Box>
      <Group gap="xs" align="flex-end" wrap="nowrap">
        <TextInput
          style={{ flex: 1 }}
          label={label}
          description={description}
          placeholder={placeholder}
          value={value}
          onChange={(e) => {
            onChange(e.currentTarget.value);
            setTest({ status: 'idle' });
          }}
          data-testid={testId}
        />
        <Tooltip label="Test expression" position="top">
          <ActionIcon
            variant="default"
            size="lg"
            mb={2}
            loading={test.status === 'loading'}
            disabled={!value.trim()}
            onClick={() => void handleTest()}
            data-testid={testId ? `${testId}-test` : undefined}
          >
            <IconFlask size={16} />
          </ActionIcon>
        </Tooltip>
      </Group>
      {resultText && (
        <Group gap={4} mt={4}>
          <Text size="xs" c="dimmed">Result:</Text>
          <Code c={resultColor} fz="xs">{resultText}</Code>
        </Group>
      )}
    </Box>
  );
}
