/**
 * Start-Run dialog — opens from the Executions page header's Run
 * scenario CTA. Pre-fills from the active sidebar scenario; the
 * operator can override the peer / subscriber, switch mode, and (for
 * Continuous mode) tune concurrency × repeats before launching.
 *
 * Form state lives in `StartRunForm`, which is keyed on `scenario.id`
 * so a fresh scenario remounts the form with the right defaults
 * (avoiding the `setState-in-effect` lint rule and removing a class
 * of stale-state bugs).
 *
 * The submit wiring (POST /executions, error → field errors,
 * Interactive vs Continuous post-success behaviour) ships in Task 7.
 * This task ships the dialog UI shell, fields, and validation.
 *
 * Reuse: refactor — same centered-modal pattern as
 * RerunConfirmModal.tsx; reuses usePeers / useSubscribers hooks for
 * the override dropdowns; reuses Mantine NumberInput /
 * SegmentedControl primitives.
 */
import {
  Badge,
  Button,
  Group,
  Modal,
  NumberInput,
  SegmentedControl,
  Select,
  Stack,
  Text,
} from '@mantine/core';
import { IconPlayerPlay } from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import { usePeers } from '../../api/resources/peers';
import { useSubscribers } from '../../api/resources/subscribers';
import type {
  ExecutionMode,
  StartExecutionInput,
} from '../../api/resources/executions';
import type { ScenarioSummary } from '../scenarios/types';

import { buildStartExecutionInput } from './buildStartExecutionInput';

interface StartRunModalProps {
  /** The pre-filled scenario; the modal stays closed when null. */
  scenario: ScenarioSummary | null;
  /** Set while a launch is in flight (Task 7 wires this). */
  isPending: boolean;
  onClose(): void;
  /**
   * Fired with the validated `StartExecutionInput` payload. Task 7
   * implements the actual POST + post-success behaviour.
   */
  onSubmit(input: StartExecutionInput): void;
  /**
   * Optional field-level error map keyed by JSON-Pointer (`/peerId`,
   * `/concurrency`, …). Task 7 populates this from the server's
   * 422 response.
   */
  fieldErrors?: Record<string, string>;
}

export function StartRunModal({
  scenario,
  isPending,
  onClose,
  onSubmit,
  fieldErrors,
}: StartRunModalProps) {
  return (
    <Modal
      opened={Boolean(scenario)}
      onClose={onClose}
      centered
      title="Run scenario"
      closeOnClickOutside={!isPending}
      closeOnEscape={!isPending}
      size="md"
      data-testid="executions-start-run-modal"
    >
      {scenario && (
        <StartRunForm
          key={scenario.id}
          scenario={scenario}
          isPending={isPending}
          onClose={onClose}
          onSubmit={onSubmit}
          fieldErrors={fieldErrors}
        />
      )}
    </Modal>
  );
}

interface StartRunFormProps {
  scenario: ScenarioSummary;
  isPending: boolean;
  onClose(): void;
  onSubmit(input: StartExecutionInput): void;
  fieldErrors?: Record<string, string>;
}

function StartRunForm({
  scenario,
  isPending,
  onClose,
  onSubmit,
  fieldErrors,
}: StartRunFormProps) {
  const peersQuery = usePeers();
  const subscribersQuery = useSubscribers();

  const [mode, setMode] = useState<ExecutionMode>('interactive');
  const [peerId, setPeerId] = useState<string | null>(scenario.peerId ?? null);
  const [subscriberId, setSubscriberId] = useState<string | null>(
    scenario.subscriberId ?? null,
  );
  const [concurrency, setConcurrency] = useState<number>(1);
  const [repeats, setRepeats] = useState<number>(10);

  const peerOptions = useMemo(
    () => (peersQuery.data ?? []).map((p) => ({ value: p.id, label: p.name })),
    [peersQuery.data],
  );

  const subscriberOptions = useMemo(
    () =>
      (subscribersQuery.data ?? []).map((s) => ({
        value: s.id,
        label: s.msisdn,
      })),
    [subscribersQuery.data],
  );

  const isInteractive = mode === 'interactive';

  const handleSubmit = () => {
    onSubmit(
      buildStartExecutionInput({
        scenarioId: scenario.id,
        mode,
        peerId,
        subscriberId,
        concurrency,
        repeats,
      }),
    );
  };

  return (
    <Stack gap="md">
      <Stack gap={4}>
        <Text size="xs" c="dimmed">
          Scenario
        </Text>
        <Badge
          variant="light"
          size="lg"
          data-testid="executions-start-run-scenario"
        >
          {scenario.name}
        </Badge>
      </Stack>

      <Select
        label="Peer"
        placeholder="Use scenario default"
        data={peerOptions}
        value={peerId}
        onChange={setPeerId}
        clearable
        searchable
        error={fieldErrors?.['/overrides/peerId'] ?? fieldErrors?.['/peerId']}
        data-testid="executions-start-run-peer"
      />

      <Select
        label="Subscriber"
        placeholder="Use scenario default"
        data={subscriberOptions}
        value={subscriberId}
        onChange={setSubscriberId}
        clearable
        searchable
        error={
          fieldErrors?.['/overrides/subscriberIds'] ??
          fieldErrors?.['/subscriberId']
        }
        data-testid="executions-start-run-subscriber"
      />

      <Stack gap={4}>
        <Text size="sm" fw={500}>
          Mode
        </Text>
        <SegmentedControl
          value={mode}
          onChange={(v) => setMode(v as ExecutionMode)}
          data={[
            { value: 'interactive', label: 'Interactive' },
            { value: 'continuous', label: 'Continuous' },
          ]}
          data-testid="executions-start-run-mode"
        />
        {fieldErrors?.['/mode'] && (
          <Text size="xs" c="red">
            {fieldErrors['/mode']}
          </Text>
        )}
      </Stack>

      <Group grow>
        <NumberInput
          label="Concurrency"
          min={1}
          max={10}
          value={concurrency}
          onChange={(v) => setConcurrency(typeof v === 'number' ? v : 1)}
          disabled={isInteractive}
          error={fieldErrors?.['/concurrency']}
          data-testid="executions-start-run-concurrency"
        />
        <NumberInput
          label="Repeats"
          min={1}
          max={1000}
          value={repeats}
          onChange={(v) => setRepeats(typeof v === 'number' ? v : 1)}
          disabled={isInteractive}
          error={fieldErrors?.['/repeats']}
          data-testid="executions-start-run-repeats"
        />
      </Group>

      {isInteractive && (
        <Text size="xs" c="dimmed">
          Concurrency and repeats are only available in Continuous mode.
        </Text>
      )}

      <Group justify="flex-end" gap="sm">
        <Button
          variant="default"
          onClick={onClose}
          disabled={isPending}
          data-testid="executions-start-run-cancel"
        >
          Cancel
        </Button>
        <Button
          leftSection={<IconPlayerPlay size={14} />}
          onClick={handleSubmit}
          loading={isPending}
          data-testid="executions-start-run-submit"
        >
          Run
        </Button>
      </Group>
    </Stack>
  );
}
