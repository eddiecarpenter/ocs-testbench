/**
 * Pure helper for the Executions sidebar — render a startedAt
 * timestamp as a human-readable "Last run: …" sub-line.
 *
 * Lives in its own module so the React-component file (Sidebar)
 * stays free of non-component exports (react-refresh rule).
 */
export function formatLastRun(
  startedAt: string | undefined,
  now: number = Date.now(),
): string {
  if (!startedAt) return 'Never run';
  const diffMs = now - Date.parse(startedAt);
  if (Number.isNaN(diffMs)) return 'Never run';
  if (diffMs < 60_000) return 'Last run: just now';
  if (diffMs < 3_600_000)
    return `Last run: ${Math.floor(diffMs / 60_000)}m ago`;
  if (diffMs < 86_400_000)
    return `Last run: ${Math.floor(diffMs / 3_600_000)}h ago`;
  return `Last run: ${Math.floor(diffMs / 86_400_000)}d ago`;
}
