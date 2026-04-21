/**
 * Format an ISO timestamp as a short relative time string.
 *
 *   2s ago, 45s ago, 3m ago, 2h ago, 1d ago, 3w ago
 *
 * Kept tiny on purpose — no dependency on date-fns / dayjs for this.
 */
export function relativeTime(iso: string | undefined, now: Date = new Date()): string {
  if (!iso) return '';
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return '';
  const diffMs = now.getTime() - then;
  if (diffMs < 0) return 'just now';

  const s = Math.floor(diffMs / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}d ago`;
  const w = Math.floor(d / 7);
  return `${w}w ago`;
}
