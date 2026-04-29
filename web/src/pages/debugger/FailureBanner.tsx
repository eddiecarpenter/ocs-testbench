/**
 * Failure-reason banner — sits above the three panes whenever the run
 * is in a non-success terminal state. The component is a no-op for
 * `pending`, `running`, `paused`, and `success`.
 *
 * Reads `state` and `failureReason` from the page-scoped store so the
 * banner reacts live when an SSE `execution.failed` event lands.
 */
import { Alert } from '@mantine/core';
import { IconAlertTriangle } from '@tabler/icons-react';

import { useExecutionStore } from './useDebuggerStore';

export function FailureBanner() {
  const state = useExecutionStore((s) => s.state);
  const failureReason = useExecutionStore((s) => s.failureReason);
  const failureDetail = useFailureDetail(state, failureReason);
  if (!failureDetail) return null;

  return (
    <Alert
      icon={<IconAlertTriangle size={16} />}
      color="red"
      title={failureDetail.title}
      data-testid="debugger-failure-banner"
    >
      {failureDetail.message}
    </Alert>
  );
}

interface FailureDetail {
  title: string;
  message: string;
}

function useFailureDetail(
  state: ReturnType<typeof useExecutionStore<string>>,
  failureReason: string | null,
): FailureDetail | null {
  if (state === 'failure') {
    return {
      title: 'Run failed',
      message:
        failureReason ?? 'The engine reported a failure. See the last response for details.',
    };
  }
  if (state === 'error') {
    return {
      title: 'Run errored',
      message:
        failureReason ?? 'The engine returned an error before reaching a terminal state.',
    };
  }
  if (state === 'aborted') {
    return {
      title: 'Run stopped',
      message: 'The run was aborted before completing. CCR-TERMINATE was sent best-effort.',
    };
  }
  return null;
}
