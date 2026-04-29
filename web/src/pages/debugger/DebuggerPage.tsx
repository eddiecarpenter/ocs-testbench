/**
 * Debugger page shell — `/executions/:id`.
 *
 * Replaces F#94's `ExecutionDebuggerStubPage`. Owns the route's
 * page-scoped `executionStore` and renders the three-pane shell
 * (Progress · Step Editor · Last-response). Pane content is filled
 * in by the per-pane tasks (#108 / #109 / #110); this task lands
 * the layout and the store wiring.
 *
 * Loading / error / not-found states are owned here so the panes
 * can assume an execution is loaded.
 */
import {
  Alert,
  Anchor,
  Card,
  Center,
  Grid,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useNavigate, useParams, Link } from 'react-router';

import { ApiError } from '../../api/errors';
import { useExecution } from '../../api/resources/executions';

import { DebuggerStoreProvider } from './DebuggerStoreProvider';
import { DebuggerTopBar } from './DebuggerTopBar';
import { ExecutionSnapshotBridge } from './ExecutionSnapshotBridge';
import { LastResponsePane } from './LastResponsePane';
import { ProgressPane } from './ProgressPane';
import { StepEditorPane } from './StepEditorPane';

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
        <Grid>
          <Grid.Col span={{ base: 12, lg: 3 }}>
            <Skeleton height={400} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, lg: 5 }}>
            <Skeleton height={400} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, lg: 4 }}>
            <Skeleton height={400} />
          </Grid.Col>
        </Grid>
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
      <Stack gap="md" data-testid="executions-debugger">
        <DebuggerTopBar
          execution={execution}
          onBack={() => navigate('/executions')}
        />
        <Grid align="stretch">
          <Grid.Col span={{ base: 12, lg: 3 }}>
            <Card
              withBorder
              padding="md"
              h="100%"
              data-testid="debugger-pane-progress"
            >
              <ProgressPane />
            </Card>
          </Grid.Col>
          <Grid.Col span={{ base: 12, lg: 5 }}>
            <Card
              withBorder
              padding="md"
              h="100%"
              data-testid="debugger-pane-step-editor"
            >
              <StepEditorPane />
            </Card>
          </Grid.Col>
          <Grid.Col span={{ base: 12, lg: 4 }}>
            <Card
              withBorder
              padding="md"
              h="100%"
              data-testid="debugger-pane-last-response"
            >
              <LastResponsePane />
            </Card>
          </Grid.Col>
        </Grid>
      </Stack>
    </DebuggerStoreProvider>
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
