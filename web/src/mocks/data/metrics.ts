import type {
  ResponseTimePoint,
  ResponseTimeSeries,
} from '../../api/resources/metrics';

/** Deterministic PRNG (Numerical Recipes LCG). */
function rng(seed: number) {
  return () => {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    return seed / 0xffffffff;
  };
}

function series(seed: number, base: number, amp: number, n: number) {
  const r = rng(seed);
  let v = base;
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    v += (r() - 0.5) * amp * 0.8;
    v = Math.max(base - amp, Math.min(base + amp, v));
    out.push(Math.round(v));
  }
  return out;
}

const NOW = Date.parse('2026-04-21T09:15:00Z');

/**
 * Build a response-time series spanning `windowMs` ending at NOW, with
 * `n` buckets. p50/p95/p99 are independently generated but stable.
 */
export function buildResponseTimeSeries(
  windowIso: string,
  windowMs: number,
  n = 30,
): ResponseTimeSeries {
  const bucketMs = Math.round(windowMs / n);
  const p50 = series(42, 35, 15, n);
  const p95 = series(17, 80, 25, n);
  const p99 = series(91, 130, 40, n);

  const points: ResponseTimePoint[] = p50.map((_, i) => ({
    t: new Date(NOW - (n - 1 - i) * bucketMs).toISOString(),
    p50: p50[i],
    p95: p95[i],
    p99: p99[i],
  }));

  return {
    window: windowIso,
    bucketSize: `PT${Math.round(bucketMs / 1000)}S`,
    points,
  };
}

/** Parse a subset of ISO 8601 durations: PT\d+[HMS]. Good enough for MVP. */
export function parseIsoDurationMs(iso: string): number {
  const m = /^PT(\d+)([HMS])$/.exec(iso);
  if (!m) return 60 * 60 * 1000; // default: 1h
  const n = Number(m[1]);
  switch (m[2]) {
    case 'H':
      return n * 60 * 60 * 1000;
    case 'M':
      return n * 60 * 1000;
    case 'S':
      return n * 1000;
    default:
      return 60 * 60 * 1000;
  }
}
