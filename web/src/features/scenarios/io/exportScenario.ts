/**
 * Export the current scenario as a JSON download.
 *
 * Called from the Builder header's `Export JSON` menu item. The
 * download is always offered to the user — no prompt, no upload, no
 * server round-trip. The shape mirrors the OpenAPI Scenario schema
 * exactly; consumers (including the Import flow) can round-trip it.
 */
import type { Scenario } from '../store/types';

export function exportScenarioAsFile(scenario: Scenario): void {
  const json = JSON.stringify(scenario, null, 2);
  const blob = new Blob([json], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${scenario.name || 'scenario'}.json`;
  a.style.display = 'none';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
