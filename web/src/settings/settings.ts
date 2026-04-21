/**
 * Client-side settings. Persisted to `localStorage` so a reload keeps
 * your preferences. Kept client-side for now — if any of these values
 * ever need to be shared across machines we'd push them server-side
 * via a `/settings` endpoint; none of them do today.
 */

import { useSyncExternalStore } from 'react';

export interface Settings {
  /**
   * Mobile Country Code + Mobile Network Code used when generating an
   * ICCID. 5 digits (3-digit MCC + 2-digit MNC, e.g. "65510" for MTN
   * South Africa) or 6 digits (3 + 3, e.g. "310410" for AT&T Wireless).
   */
  mccmnc: string;
}

// "655 10" is MTN South Africa — a plausible default for the testbench.
// Chosen because the Figma fixtures use South African MSISDNs (27…).
const DEFAULT_SETTINGS: Settings = {
  mccmnc: '65510',
};

const STORAGE_KEY = 'ocs.settings.v1';

function read(): Settings {
  if (typeof localStorage === 'undefined') return DEFAULT_SETTINGS;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(raw) as Partial<Settings>;
    return { ...DEFAULT_SETTINGS, ...parsed };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

let cached: Settings = read();
const listeners = new Set<() => void>();

function emit() {
  for (const l of listeners) l();
}

export function getSettings(): Settings {
  return cached;
}

export function setSettings(next: Partial<Settings>): void {
  cached = { ...cached, ...next };
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(cached));
  } catch {
    // localStorage may be unavailable (private mode, quota) — keep the
    // in-memory copy so the UI still reflects the change this session.
  }
  emit();
}

function subscribe(fn: () => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** React hook — re-renders when any setting changes. */
export function useSettings(): Settings {
  return useSyncExternalStore(subscribe, getSettings, getSettings);
}

/** MCCMNC is valid when it's 5 or 6 digits. */
export function isValidMccmnc(mccmnc: string): boolean {
  return /^[0-9]{5,6}$/.test(mccmnc);
}
