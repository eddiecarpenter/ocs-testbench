/**
 * Client-side settings. Persisted to `localStorage` so a reload keeps
 * your preferences. Kept client-side for now — if any of these values
 * ever need to be shared across machines we'd push them server-side
 * via a `/settings` endpoint; none of them do today.
 *
 * The Mantine colour scheme (light/dark/auto) is NOT stored here — it
 * lives in Mantine's own `useMantineColorScheme` store. Keeping the
 * settings store free of duplicate state means the Theme control on the
 * Settings page is a thin pass-through to the framework's source of
 * truth.
 */

import { useSyncExternalStore } from 'react';

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';
export type DiameterTransport = 'TCP' | 'TLS';

export interface Settings {
  /**
   * Mobile Country Code + Mobile Network Code used when generating an
   * ICCID. 5 digits (3-digit MCC + 2-digit MNC, e.g. "65510" for MTN
   * South Africa) or 6 digits (3 + 3, e.g. "310410" for AT&T Wireless).
   */
  mccmnc: string;

  // --- General --------------------------------------------------------

  /**
   * Whether the binary should launch the default browser when the HTTP
   * server is ready. The real setting lives on the Go side; this value
   * is persisted client-side so the Settings page UI is the authoritative
   * display and the next boot config can pick it up.
   */
  autoOpenBrowser: boolean;

  /** Log verbosity. Surfaces as a dropdown on the Settings page. */
  logLevel: LogLevel;

  // --- Diameter defaults ---------------------------------------------
  // Applied when creating a new peer. The user can still override each
  // field on the peer form itself; these are purely starting values.

  /** Suffix appended to a peer's generated Origin-Host, e.g. ".test.local". */
  originHostSuffix: string;

  /** Origin-Realm used as the default on a new peer. */
  originRealm: string;

  /** Default watchdog interval (seconds) for a new peer. */
  watchdogIntervalSeconds: number;

  /** Default Diameter transport for a new peer. */
  defaultTransport: DiameterTransport;
}

// "655 10" is MTN South Africa — a plausible default for the testbench.
// Chosen because the Figma fixtures use South African MSISDNs (27…).
const DEFAULT_SETTINGS: Settings = {
  mccmnc: '65510',
  autoOpenBrowser: true,
  logLevel: 'info',
  originHostSuffix: '.test.local',
  originRealm: 'test.local',
  watchdogIntervalSeconds: 30,
  defaultTransport: 'TCP',
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

/** Canonical log-level list for UI controls. */
export const LOG_LEVELS: { value: LogLevel; label: string }[] = [
  { value: 'debug', label: 'Debug' },
  { value: 'info', label: 'Info' },
  { value: 'warn', label: 'Warn' },
  { value: 'error', label: 'Error' },
];

/** Canonical transport list for UI controls. */
export const DIAMETER_TRANSPORTS: { value: DiameterTransport; label: string }[] = [
  { value: 'TCP', label: 'TCP' },
  { value: 'TLS', label: 'TLS' },
];
