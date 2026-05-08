import {
  Alert,
  Button,
  Card,
  Grid,
  Group,
  Skeleton,
  SimpleGrid,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useEffect } from 'react';

import { useDashboardKpis } from '../../api/resources/dashboard';
import { useExecutions } from '../../api/resources/executions';
import { metricsKeys, useResponseTimeSeries } from '../../api/resources/metrics';
import { usePeers } from '../../api/resources/peers';
import { appConfig } from '../../config/app';
import { KpiCard } from './KpiCard';
import { toKpiStats } from './kpis';
import { PeerStatusCard } from './PeerStatusCard';
import { RecentExecutionsCard } from './RecentExecutionsCard';
import { ResponseTimeCard } from './ResponseTimeCard';

/** Reusable error alert with a retry button for any failed query. */
function QueryError({
  title,
  onRetry,
}: {
  title: string;
  onRetry: () => void;
}) {
  return (
    <Alert
      color="red"
      icon={<IconAlertTriangle size={18} />}
      title={title}
      variant="light"
    >
      <Stack gap="xs" align="flex-start">
        <Text size="sm" c="dimmed">
          Couldn&apos;t load this data. Check the API and try again.
        </Text>
        <Button size="xs" variant="subtle" color="red" onClick={onRetry}>
          Retry
        </Button>
      </Stack>
    </Alert>
  );
}

/** Skeleton wrapper that mimics a card's padding/border for a stable layout. */
function CardSkeleton({ height, children }: { height: number; children?: ReactNode }) {
  return (
    <Card padding="lg" withBorder shadow="xs" mih={height}>
      {children ?? <Skeleton h={height - 40} radius="sm" />}
    </Card>
  );
}

/**
 * Subscribes to the SSE stream of the first currently-running execution and
 * invalidates the response-time metrics query on every step event. This keeps
 * the chart live without polling — silent when nothing is running.
 */
function useMetricsRefreshFromSse(runningExecutionId: string | undefined) {
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!runningExecutionId) return;
    const url = `${appConfig.apiBaseUrl}/events/executions/${encodeURIComponent(runningExecutionId)}`;
    const es = new EventSource(url);
    es.addEventListener('execution.progress', (ev: MessageEvent) => {
      try {
        const raw = JSON.parse(ev.data as string) as { state?: string };
        // Refresh after each completed step or when the run finishes.
        if (raw.state === 'running' || raw.state === 'completed' || raw.state === 'success') {
          void queryClient.invalidateQueries({ queryKey: metricsKeys.all });
        }
      } catch { /* malformed — ignore */ }
    });
    return () => es.close();
  }, [runningExecutionId, queryClient]);
}

export function DashboardPage() {
  const kpis = useDashboardKpis();
  const peers = usePeers();
  const executions = useExecutions({ limit: 5 });
  const responseTime = useResponseTimeSeries({ window: 'PT1H' });

  const runningId = executions.data?.items.find(
    (e) => e.state === 'running',
  )?.id;
  useMetricsRefreshFromSse(runningId);

  return (
    <Stack gap="lg" p="md">
      <Stack gap={4}>
        <Title order={2} fw={600}>
          Dashboard
        </Title>
        <Text c="dimmed" size="sm">
          Overview of testbench state and recent activity
        </Text>
      </Stack>

      {/* KPI tiles — 4 tiles per Figma 01-dashboard.png. */}
      <SimpleGrid cols={{ base: 1, xs: 2, md: 4 }} spacing="md">
        {kpis.isLoading || !kpis.data
          ? Array.from({ length: 4 }).map((_, i) => (
              <CardSkeleton key={i} height={120} />
            ))
          : toKpiStats(kpis.data).map((stat) => (
              <KpiCard key={stat.label} stat={stat} />
            ))}
      </SimpleGrid>
      {kpis.isError && (
        <QueryError title="KPI counters unavailable" onRetry={() => kpis.refetch()} />
      )}

      {/* Peer + Recent Executions */}
      <Grid>
        <Grid.Col span={{ base: 12, md: 6 }}>
          {peers.isError ? (
            <QueryError title="Peer status unavailable" onRetry={() => peers.refetch()} />
          ) : peers.isLoading || !peers.data ? (
            <CardSkeleton height={300} />
          ) : (
            <PeerStatusCard peers={peers.data} />
          )}
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6 }}>
          {executions.isError ? (
            <QueryError
              title="Executions unavailable"
              onRetry={() => executions.refetch()}
            />
          ) : executions.isLoading || !executions.data ? (
            <CardSkeleton height={300} />
          ) : (
            <RecentExecutionsCard executions={executions.data.items} />
          )}
        </Grid.Col>
      </Grid>

      {/* Response time chart */}
      {responseTime.isError ? (
        <QueryError
          title="Response-time metrics unavailable"
          onRetry={() => responseTime.refetch()}
        />
      ) : responseTime.isLoading || !responseTime.data ? (
        <CardSkeleton height={260}>
          <Group justify="space-between" mb="md">
            <Skeleton h={20} w={160} />
            <Skeleton h={16} w={80} />
          </Group>
          <Skeleton h={200} radius="sm" />
        </CardSkeleton>
      ) : (
        <ResponseTimeCard data={responseTime.data.points} />
      )}
    </Stack>
  );
}
