/**
 * Debugger page shell — `/executions/:id`.
 *
 * Two modes driven by store state:
 *
 *   Debugger mode (paused + no historical selection):
 *     History(320px) | StepEditorPane (two-column, full width)
 *
 *   Viewer mode (terminal, running, or historical step selected):
 *     History(320px) | RequestPane | LastResponsePane
 *
 * Loading / error / not-found states are owned here so the panes
 * can assume an execution is loaded.
 */
import {
  Alert,
  Anchor,
  Box,
  Card,
  Center,
  Flex,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconAlertTriangle } from '@tabler/icons-react';
import { Link, useNavigate, useParams } from 'react-router';

import { ApiError } from '../../api/errors';
import { useExecution } from '../../api/resources/executions';

import { DebuggerStoreProvider } from './DebuggerStoreProvider';
import { DebuggerTopBar } from './DebuggerTopBar';
import { ExecutionHistoryPane } from './ExecutionHistoryPane';
import { ExecutionSnapshotBridge } from './ExecutionSnapshotBridge';
import { ExecutionSseBridge } from './ExecutionSseBridge';
import { FailureBanner } from './FailureBanner';
import { LastResponsePane } from './LastResponsePane';
import { RequestPane } from './RequestPane';
import { StepEditorPane } from './StepEditorPane';
import { useExecutionStore } from './useDebuggerStore';

export function DebuggerPage() {
  const { id } = useParams<{ id?: string }>();
  const navigate = useNavigate();
  const executionQuery = useExecution(id ?? '');

  if (!id) {
    return (
      <NotFoundPanel
        title="Execution id missing"
        message="The route is missing an execution id. Go back and pick a run."
      />
    );
  }

  if (executionQuery.isLoading) {
    return (
      <Stack gap="md" data-testid="executions-debugger-loading">
        <Skeleton height={64} />
        <Flex gap="md">
          <Skeleton height={400} w={320} style={{ flexShrink: 0 }} />
          <Skeleton height={400} style={{ flex: 1 }} />
          <Skeleton height={400} style={{ flex: 1 }} />
        </Flex>
      </Stack>
    );
  }

  if (executionQuery.isError) {
    const err = executionQuery.error as ApiError | Error;
    if (err instanceof ApiError && err.status === 404) {
      return (
        <NotFoundPanel
          title={`Execution #${id} not found`}
          message="No execution exists with this id. It may have been pruned, or the URL is wrong."
        />
      );
    }
    return (
      <Alert
        icon={<IconAlertTriangle size={16} />}
        color="red"
        title="Failed to load execution"
        data-testid="executions-debugger-error"
      >
        {err.message}
        <Stack mt="sm" gap={4}>
          <Anchor component={Link} to="/executions">
            ← Back to executions
          </Anchor>
        </Stack>
      </Alert>
    );
  }

  const execution = executionQuery.data;
  if (!execution) {
    return (
      <NotFoundPanel
        title={`Execution #${id} not found`}
        message="The server returned no execution body for this id."
      />
    );
  }

  return (
    <DebuggerStoreProvider executionId={id}>
      <ExecutionSnapshotBridge execution={execution} />
      <ExecutionSseBridge executionId={id} execution={execution} />
      {/*
        position: fixed so the debugger always fills the exact viewport
        area below the header and inside the AppShell padding, regardless
        of the parent Main element's flex/scroll context.
      */}
      <Stack
        gap="md"
        data-testid="executions-debugger"
        style={{
          position: 'fixed',
          top: 'calc(var(--app-shell-header-offset, 3.75rem) + var(--app-shell-padding, 1rem))',
          bottom: 'var(--app-shell-padding, 1rem)',
          left: 'calc(var(--app-shell-navbar-width, 15rem) + var(--app-shell-padding, 1rem))',
          right: 'var(--app-shell-padding, 1rem)',
          overflow: 'hidden',
        }}
      >
        <DebuggerTopBar
          execution={execution}
          onBack={() => navigate('/executions')}
        />
        <FailureBanner />
        <DebuggerContent />
      </Stack>
    </DebuggerStoreProvider>
  );
}

/** Inner component — reads the store to drive mode switching. */
function DebuggerContent() {
  const runState = useExecutionStore((s) => s.state);
  const historicalIndex = useExecutionStore((s) => s.historicalIndex);
  const debuggerMode = runState === 'paused' && historicalIndex === null;

  return (
    <Flex gap="md" align="stretch" style={{ flex: 1, minHeight: 0 }} data-testid="debugger-content">
      {/* History — fixed 320 px */}
      <Box w={320} style={{ flexShrink: 0 }}>
        <Card withBorder padding="md" h="100%" data-testid="debugger-pane-progress">
          <ExecutionHistoryPane />
        </Card>
      </Box>

      {debuggerMode ? (
        /* Debugger mode: wide two-column step editor */
        <Card
          withBorder
          padding="md"
          style={{ flex: 1 }}
          data-testid="debugger-pane-step-editor"
        >
          <StepEditorPane />
        </Card>
      ) : (
        /* Viewer mode: REQUEST + RESPONSE side by side */
        <>
          <Card
            withBorder
            padding="md"
            style={{ flex: 1 }}
            data-testid="debugger-pane-request"
          >
            <RequestPane />
          </Card>
          <Card
            withBorder
            padding="md"
            style={{ flex: 1 }}
            data-testid="debugger-pane-last-response"
          >
            <LastResponsePane />
          </Card>
        </>
      )}
    </Flex>
  );
}

interface NotFoundPanelProps {
  title: string;
  message: string;
}

function NotFoundPanel({ title, message }: NotFoundPanelProps) {
  return (
    <Center py="xl" data-testid="executions-debugger-not-found">
      <Stack align="center" gap="xs" maw={520}>
        <Title order={3}>{title}</Title>
        <Text c="dimmed" size="sm" ta="center">
          {message}
        </Text>
        <Anchor component={Link} to="/executions">
          ← Back to executions
        </Anchor>
      </Stack>
    </Center>
  );
}
