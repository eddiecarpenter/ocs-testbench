/**
 * Pure validation helpers for the Steps tab. Lives outside the tab
 * component file so React Fast Refresh does not complain about mixing
 * components and helpers.
 */
import type { RequestType, SessionMode } from '../../store/types';

/** Request types legal under each session mode (architecture §4). */
export function legalRequestTypes(mode: SessionMode): RequestType[] {
  return mode === 'session'
    ? ['INITIAL', 'UPDATE', 'TERMINATE']
    : ['EVENT'];
}

export function isLegalRequestType(
  mode: SessionMode,
  type: RequestType,
): boolean {
  return legalRequestTypes(mode).includes(type);
}
