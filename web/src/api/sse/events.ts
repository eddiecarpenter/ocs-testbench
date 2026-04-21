import type { DashboardKpis } from '../resources/dashboard';
import type {
  Execution,
  ExecutionSummary,
} from '../resources/executions';
import type { Peer } from '../resources/peers';

/**
 * Discriminated union of every SSE event the server may emit. The `type`
 * field mirrors the SSE `event:` header and keys the payload shape.
 *
 * Adding a new event = add a member here + a handler in `SseProvider`.
 */
export type SseEvent =
  | { type: 'peer.updated'; data: Peer }
  | { type: 'execution.created'; data: ExecutionSummary }
  | { type: 'execution.progress'; data: Execution }
  | { type: 'dashboard.kpi'; data: DashboardKpis };

/** All known event-type strings — handy for iterating subscriptions. */
export const SSE_EVENT_TYPES: SseEvent['type'][] = [
  'peer.updated',
  'execution.created',
  'execution.progress',
  'dashboard.kpi',
];
