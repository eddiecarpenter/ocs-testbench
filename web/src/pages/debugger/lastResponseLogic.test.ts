/**
 * Pure-logic tests for the Last-response pane.
 *
 * The pane's React render is exercised through Storybook (out of MVP);
 * these unit tests pin the displayed-step picker, the result-code
 * palette, and the response-field extraction helpers.
 */
import { describe, expect, it } from 'vitest';

import type { StepRecord } from '../../api/resources/executions';

import {
  approximateSize,
  extractExtractions,
  extractResultCode,
  formatRtt,
  formatSize,
  lastCompletedStep,
  pickDisplayedStep,
  resultCodeColor,
  resultCodeLabel,
} from './lastResponseLogic';

function step(
  i: number,
  state: StepRecord['state'],
  extras: Partial<StepRecord> = {},
): StepRecord {
  return { n: i + 1, kind: 'request', state, ...extras };
}

describe('lastCompletedStep', () => {
  it('returns the most recently completed step', () => {
    const steps: StepRecord[] = [
      step(0, 'success'),
      step(1, 'success'),
      step(2, 'pending'),
    ];
    expect(lastCompletedStep(steps, 2)?.n).toBe(2);
  });
  it('skips skipped / pending steps', () => {
    const steps: StepRecord[] = [
      step(0, 'success'),
      step(1, 'skipped'),
      step(2, 'pending'),
    ];
    expect(lastCompletedStep(steps, 3)?.n).toBe(1);
  });
  it('returns undefined when no step has completed', () => {
    expect(lastCompletedStep([step(0, 'pending')], 0)).toBeUndefined();
    expect(lastCompletedStep([], 0)).toBeUndefined();
  });
});

describe('pickDisplayedStep', () => {
  const steps: StepRecord[] = [
    step(0, 'success', { label: 'CCR-I' }),
    step(1, 'success', { label: 'CCR-U-1' }),
    step(2, 'pending', { label: 'CCR-U-2' }),
  ];

  it('historical view overrides live view', () => {
    expect(pickDisplayedStep(steps, 2, 0)?.label).toBe('CCR-I');
  });

  it('falls back to lastCompletedStep when historical is null', () => {
    expect(pickDisplayedStep(steps, 2, null)?.label).toBe('CCR-U-1');
  });

  it('historical pointing at a missing index — falls through to live', () => {
    expect(pickDisplayedStep(steps, 2, 99)?.label).toBe('CCR-U-1');
  });
});

describe('resultCodeColor', () => {
  it('2xxx → teal', () => {
    expect(resultCodeColor(2001)).toBe('teal');
    expect(resultCodeColor(2002)).toBe('teal');
  });
  it('4xxx → yellow', () => {
    expect(resultCodeColor(4010)).toBe('yellow');
    expect(resultCodeColor(4012)).toBe('yellow');
  });
  it('5xxx → red', () => {
    expect(resultCodeColor(5012)).toBe('red');
  });
  it('unknown / undefined → gray', () => {
    expect(resultCodeColor(undefined)).toBe('gray');
    expect(resultCodeColor(9999)).toBe('gray');
    expect(resultCodeColor(1000)).toBe('gray');
  });
});

describe('resultCodeLabel', () => {
  it('renders friendly labels for known codes', () => {
    expect(resultCodeLabel(2001)).toMatch(/SUCCESS/);
    expect(resultCodeLabel(5012)).toMatch(/UNABLE_TO_COMPLY/);
  });
  it('renders the bare number for unknown codes', () => {
    expect(resultCodeLabel(9999)).toBe('9999');
  });
  it('renders dash for undefined', () => {
    expect(resultCodeLabel(undefined)).toBe('—');
  });
});

describe('extractResultCode', () => {
  it('reads `resultCode` (mock convention)', () => {
    expect(extractResultCode({ resultCode: 2001 })).toBe(2001);
  });
  it('reads `Result-Code` (Diameter AVP key)', () => {
    expect(extractResultCode({ 'Result-Code': 5012 })).toBe(5012);
  });
  it('returns undefined when missing or non-numeric', () => {
    expect(extractResultCode({})).toBeUndefined();
    expect(extractResultCode(undefined)).toBeUndefined();
    expect(extractResultCode({ resultCode: 'oops' })).toBeUndefined();
  });
});

describe('extractExtractions', () => {
  it('returns the extractions map when present', () => {
    expect(
      extractExtractions({ extractions: { GRANTED_TOTAL: 1024, EXPIRY: null } }),
    ).toEqual({ GRANTED_TOTAL: 1024, EXPIRY: null });
  });
  it('returns {} when missing or malformed', () => {
    expect(extractExtractions(undefined)).toEqual({});
    expect(extractExtractions({})).toEqual({});
    expect(extractExtractions({ extractions: 'oops' })).toEqual({});
  });
});

describe('approximateSize / formatSize', () => {
  it('returns 0 for missing payloads', () => {
    expect(approximateSize(undefined)).toBe(0);
  });
  it('roughly tracks JSON string length', () => {
    expect(approximateSize({ a: 'hello' })).toBeGreaterThan(8);
  });
  it('formats size below KiB threshold as bytes', () => {
    expect(formatSize(123)).toBe('123 B');
  });
  it('formats size at-or-above KiB threshold as KiB', () => {
    expect(formatSize(2048)).toBe('2.0 KiB');
  });
});

describe('formatRtt', () => {
  it('renders sub-1ms as <1 ms', () => {
    expect(formatRtt(0.4)).toBe('<1 ms');
  });
  it('renders integer ms', () => {
    expect(formatRtt(42)).toBe('42 ms');
  });
  it('renders dash for undefined', () => {
    expect(formatRtt(undefined)).toBe('—');
  });
});
