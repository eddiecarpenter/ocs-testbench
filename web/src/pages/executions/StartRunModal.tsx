/**
 * Start-Run dialog — opens from the Executions page header's "Run as
 * continuous batch…" CTA, or from a row kebab's "Re-run with
 * overrides…" item. The default "Run scenario" path on the page
 * fires silently (no dialog) and uses the scenario's bound peer +
 * subscriber.
 *
 * The dialog stays focused on run-time decisions:
 *   - Mode (Interactive / Continuous)
 *   - Concurrency / Repeats (Continuous only)
 *   - Optional peer / subscriber override (collapsed by default —
 *     the scenario already binds them; overriding is the rare case)
 *
 * Form state lives in `StartRunForm`, which is keyed on the scenario
 * id + an `instance` token so re-opening the dialog from a different
 * source row (e.g. a "Re-run with overrides…" on a row using
 * different mode/concurrency/peer) remounts with fresh defaults.
 */
import {
  Badge,
  Button,
  Collapse,
  Group,
  Modal,
  NumberInput,
  SegmentedControl,
  Select,
  Stack,
  Text,
  UnstyledButton,
} from '@mantine/core';
import {
  IconChevronDown,
  IconChevronRight,
  IconPlayerPlay,
} from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import { usePeers } from '../../api/resources/peers';
import { useSubscribers } from '../../api/resources/subscribers';
import type {
  ExecutionMode,
  StartExecutionInput,
} from '../../api/resources/executions';
import type { ScenarioSummary } from '../scenarios/types';

import { buildStartExecutionInput } from './buildStartExecutionInput';

export interface StartRunInitialValues {
  mode?: ExecutionMode;
  peerId?: string | null;
  subscriberId?: string | null;
  concurrency?: number;
  repeats?: number;
  /**
   * Whether the override section starts expanded. Useful for the
   * "Re-run with overrides…" entry point where the user has already
   * declared intent to override.
   */
  overrideExpanded?: boolean;
  /**
   * Optional dialog title — defaults to "Run scenario". Callers like
   * "Run as continuous batch…" can pass a more specific label.
   */
  title?: string;
  /**
   * Token bumped each time the dialog is reopened from a different
   * source so the form remounts with fresh defaults.
   */
  instance?: string;
}

interface StartRunModalProps {
  /** The pre-filled scenario; the modal stays closed when null. */
  scenario: ScenarioSummary | null;
  /** Pre-fill values + dialog-shape options. */
  initial?: StartRunInitialValues;
  /** Set while a launch is in flight. */
  isPending: boolean;
  onClose(): void;
  onSubmit(input: StartExecutionInput): void;
  fieldErrors?: Record<string, string>;
}

export function StartRunModal({
  scenario,
  initial,
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
      title={initial?.title ?? 'Run scenario'}
      closeOnClickOutside={!isPending}
      closeOnEscape={!isPending}
      size="md"
      data-testid="executions-start-run-modal"
    >
      {scenario && (
        <StartRunForm
          // Remount on scenario change AND on dialog-open instance change
          // (e.g. a different source row triggered "Re-run with overrides").
          key={`${scenario.id}::${initial?.instance ?? 'default'}`}
          scenario={scenario}
          initial={initial}
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
  initial: StartRunInitialValues | undefined;
  isPending: boolean;
  onClose(): void;
  onSubmit(input: StartExecutionInput): void;
  fieldErrors?: Record<string, string>;
}

function StartRunForm({
  scenario,
  initial,
  isPending,
  onClose,
  onSubmit,
  fieldErrors,
}: StartRunFormProps) {
  const peersQuery = usePeers();
  const subscribersQuery = useSubscribers();

  const [mode, setMode] = useState<ExecutionMode>(
    initial?.mode ?? 'interactive',
  );
  const [peerId, setPeerId] = useState<string | null>(
    initial?.peerId ?? null,
  );
  const [subscriberId, setSubscriberId] = useState<string | null>(
    initial?.subscriberId ?? null,
  );
  const [concurrency, setConcurrency] = useState<number>(
    initial?.concurrency ?? 1,
  );
  const [repeats, setRepeats] = useState<number>(initial?.repeats ?? 10);
  const [overrideExpanded, setOverrideExpanded] = useState<boolean>(
    Boolean(
      initial?.overrideExpanded || initial?.peerId || initial?.subscriberId,
    ),
  );

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

  // Resolve the bound names for the read-only override summary line.
  const boundPeerName =
    peerOptions.find((o) => o.value === scenario.peerId)?.label ??
    scenario.peerId ??
    '—';
  const boundSubscriberName =
    subscriberOptions.find((o) => o.value === scenario.subscriberId)?.label ??
    scenario.subscriberId ??
    '—';

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

      {/*
        Override section — collapsed by default since the scenario
        already binds peer + subscriber. The toggle reads as a small
        chevron + summary so the user can see the bound values at a
        glance and one-click into the override editor.
      */}
      <Stack gap={4}>
        <UnstyledButton
          onClick={() => setOverrideExpanded((v) => !v)}
          data-testid="executions-start-run-override-toggle"
          aria-expanded={overrideExpanded}
        >
          <Group gap={4} wrap="nowrap">
            {overrideExpanded ? (
              <IconChevronDown size={14} />
            ) : (
              <IconChevronRight size={14} />
            )}
            <Text size="sm" fw={500}>
              Override peer / subscriber
            </Text>
            {!overrideExpanded && (
              <Text size="xs" c="dimmed">
                · {boundPeerName} · {boundSubscriberName} (from scenario)
              </Text>
            )}
          </Group>
        </UnstyledButton>

        <Collapse expanded={overrideExpanded}>
          <Stack gap="md" pt="xs">
            <Select
              label="Peer"
              placeholder={`Use scenario default (${boundPeerName})`}
              data={peerOptions}
              value={peerId}
              onChange={setPeerId}
              clearable
              searchable
              error={
                fieldErrors?.['/overrides/peerId'] ?? fieldErrors?.['/peerId']
              }
              data-testid="executions-start-run-peer"
            />
            <Select
              label="Subscriber"
              placeholder={`Use scenario default (${boundSubscriberName})`}
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
          </Stack>
        </Collapse>
      </Stack>

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
