/**
 * Pure-logic tests for the Progress pane.
 *
 * The pane's React render is exercised through Storybook visual regression
 * (out of MVP per the Feature body); these unit tests pin the click /
 * state / formatting rules so the rendered behaviour can't drift.
 */
import { describe, expect, it } from 'vitest';

import type { StepRecord } from '../../api/resources/executions';

import {
  clickAction,
  computeRowState,
  formatDuration,
  isStepCompleted,
  kindColor,
  kindLabel,
} from './progressPaneLogic';

function step(state: StepRecord['state'], extras: Partial<StepRecord> = {}): StepRecord {
  return {
    n: 1,
    kind: 'request',
    state,
    ...extras,
  };
}

describe('isStepCompleted', () => {
  it('treats success / failure / error as completed', () => {
    expect(isStepCompleted(step('success'))).toBe(true);
    expect(isStepCompleted(step('failure'))).toBe(true);
    expect(isStepCompleted(step('error'))).toBe(true);
  });

  it('treats running / pending / skipped / undefined as not completed', () => {
    expect(isStepCompleted(step('running'))).toBe(false);
    expect(isStepCompleted(step('pending'))).toBe(false);
    expect(isStepCompleted(step('skipped'))).toBe(false);
    expect(isStepCompleted(undefined)).toBe(false);
  });
});

describe('clickAction', () => {
  it('returns viewHistorical for completed steps', () => {
    expect(clickAction(step('success'), 3)).toEqual({
      kind: 'viewHistorical',
      stepIndex: 3,
    });
    expect(clickAction(step('failure'), 0)).toEqual({
      kind: 'viewHistorical',
      stepIndex: 0,
    });
  });

  it('returns null for non-completed steps', () => {
    expect(clickAction(step('pending'), 1)).toBeNull();
    expect(clickAction(step('running'), 1)).toBeNull();
    expect(clickAction(step('skipped'), 1)).toBeNull();
    expect(clickAction(undefined, 1)).toBeNull();
  });
});

describe('computeRowState', () => {
  it('maps step record states to row states 1:1 for non-cursor rows', () => {
    expect(computeRowState(step('success'), false, 'running')).toBe('success');
    expect(computeRowState(step('failure'), false, 'running')).toBe('failure');
    expect(computeRowState(step('error'), false, 'running')).toBe('error');
    expect(computeRowState(step('skipped'), false, 'running')).toBe('skipped');
    expect(computeRowState(step('running'), false, 'running')).toBe('running');
    expect(computeRowState(step('pending'), false, 'running')).toBe('pending');
  });

  it('cursor on a paused run is rendered as paused regardless of step state', () => {
    expect(computeRowState(step('running'), true, 'paused')).toBe('paused');
    expect(computeRowState(step('pending'), true, 'paused')).toBe('paused');
    expect(computeRowState(undefined, true, 'paused')).toBe('paused');
  });

  it('cursor on a running run keeps the natural state', () => {
    expect(computeRowState(step('running'), true, 'running')).toBe('running');
    expect(computeRowState(step('pending'), true, 'running')).toBe('pending');
  });

  it('missing record renders as pending unless cursor on a paused run', () => {
    expect(computeRowState(undefined, false, 'running')).toBe('pending');
    expect(computeRowState(undefined, true, 'paused')).toBe('paused');
  });
});

describe('formatDuration', () => {
  it('renders sub-1s as ms', () => {
    expect(formatDuration(0)).toBe('0ms');
    expect(formatDuration(450)).toBe('450ms');
  });
  it('renders sub-1m as seconds', () => {
    expect(formatDuration(1_000)).toBe('1s');
    expect(formatDuration(12_300)).toBe('12s');
  });
  it('renders >=1m as MmSSs with zero-padding', () => {
    expect(formatDuration(60_000)).toBe('1m 00s');
    expect(formatDuration(125_000)).toBe('2m 05s');
  });
});

describe('kindLabel / kindColor', () => {
  it('labels each step kind', () => {
    expect(kindLabel('request')).toBe('Request');
    expect(kindLabel('consume')).toBe('Consume');
    expect(kindLabel('wait')).toBe('Wait');
    expect(kindLabel('pause')).toBe('Pause');
  });
  it('colours each step kind', () => {
    expect(kindColor('request')).toBe('blue');
    expect(kindColor('consume')).toBe('grape');
    expect(kindColor('wait')).toBe('gray');
    expect(kindColor('pause')).toBe('yellow');
  });
});
