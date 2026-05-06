/**
 * Pure selectors for the Scenarios list view — used by
 * `ScenariosListPage` and exercised by unit tests so AC-1 / AC-2 /
 * AC-3 (grouping + search + peer filter) have explicit coverage.
 */
import type { ScenarioSummary, ServiceType } from './types';

export const SERVICE_TYPE_GROUP_ORDER: ServiceType[] = [
  'VOICE', 'DATA', 'SMS', 'USSD1_EVENT', 'USSD1_SESSION', 'USSD2_SESSION',
];

export const SERVICE_TYPE_LABELS: Record<ServiceType, string> = {
  VOICE: 'Voice',
  DATA: 'Data',
  SMS: 'SMS',
  USSD1_EVENT: 'USSD1 Event',
  USSD1_SESSION: 'USSD1 Session',
  USSD2_SESSION: 'USSD2 Session',
};

const UNGROUPED = '__ungrouped__' as const;
type GroupKey = ServiceType | typeof UNGROUPED;

export function groupByServiceType(
  rows: ScenarioSummary[],
): Record<GroupKey, ScenarioSummary[]> {
  const out = Object.fromEntries([
    ...SERVICE_TYPE_GROUP_ORDER.map((t) => [t, [] as ScenarioSummary[]]),
    [UNGROUPED, [] as ScenarioSummary[]],
  ]) as Record<GroupKey, ScenarioSummary[]>;
  for (const r of rows) out[r.serviceType ?? UNGROUPED].push(r);
  return out;
}

/**
 * Apply the search (case-insensitive across name + peer name) and
 * peer-id filter. The peer-name lookup is supplied by the caller as a
 * `Map<peerId, peerName>` — list page uses the live peers query for
 * this; tests pass an in-memory map.
 */
export function filterScenarios(
  rows: ScenarioSummary[],
  opts: {
    search: string;
    peerFilter: string | null;
    peerNameById: Map<string, string>;
  },
): ScenarioSummary[] {
  const needle = opts.search.trim().toLowerCase();
  return rows.filter((row) => {
    if (opts.peerFilter && row.peerId !== opts.peerFilter) return false;
    if (!needle) return true;
    const peerName = (row.peerId && opts.peerNameById.get(row.peerId)) ?? '';
    return (
      row.name.toLowerCase().includes(needle) ||
      peerName.toLowerCase().includes(needle)
    );
  });
}
