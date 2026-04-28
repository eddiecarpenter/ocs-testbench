/**
 * Pure selectors for the Scenarios list view — used by
 * `ScenariosListPage` and exercised by unit tests so AC-1 / AC-2 /
 * AC-3 (grouping + search + peer filter) have explicit coverage.
 */
import type { ScenarioSummary, UnitType } from '../store/types';

export const UNIT_GROUP_ORDER: UnitType[] = ['OCTET', 'TIME', 'UNITS'];

export function groupByUnit(rows: ScenarioSummary[]): Record<UnitType, ScenarioSummary[]> {
  const out: Record<UnitType, ScenarioSummary[]> = {
    OCTET: [],
    TIME: [],
    UNITS: [],
  };
  for (const r of rows) out[r.unitType].push(r);
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
