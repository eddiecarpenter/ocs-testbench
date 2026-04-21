/**
 * ICCID helpers — SIM serial composition with Luhn check digit.
 *
 * An ICCID (ITU-T E.118) is 19 or 20 digits:
 *   ┌ 2 ┬ MCC+MNC (5–6) ┬ serial (10–12) ┬ 1 ┐
 *   │89 │  MCCMNC       │   account      │L │
 *   └───┴───────────────┴────────────────┴──┘
 *
 * The real spec uses an E.164 country code plus an issuer identifier,
 * but for a testbench we key off MCCMNC (Mobile Country Code + Mobile
 * Network Code — the same value operators embed in the IMSI) because
 * that's what the operator has in their provisioning system.
 *
 * The final digit is a Luhn mod-10 check over the preceding digits.
 * We reuse `luhnCheckDigit` from `./imei` so both identifiers share a
 * single checksum implementation.
 */

import { luhnCheckDigit } from './imei';

/** True when `iccid` is a 19- or 20-digit string with a valid Luhn check. */
export function isValidIccid(iccid: string): boolean {
  if (!/^[0-9]{19,20}$/.test(iccid)) return false;
  const check = luhnCheckDigit(iccid.slice(0, -1));
  return check === Number(iccid[iccid.length - 1]);
}

/**
 * Compose an ICCID from an MCCMNC (5 or 6 digits) and an optional
 * account serial. When the serial is omitted, it is rolled randomly
 * and the result is a 19-digit ICCID (the common case).
 *
 * Returns `undefined` when the MCCMNC is malformed.
 */
export function buildIccid(
  mccmnc: string,
  serial?: string,
): string | undefined {
  if (!/^[0-9]{5,6}$/.test(mccmnc)) return undefined;
  // Default to a 19-digit ICCID: 2 (89) + mccmnc + serial + 1 (check).
  const serialLen = 19 - 1 - 2 - mccmnc.length;
  const s =
    serial ??
    Array.from({ length: serialLen }, () =>
      Math.floor(Math.random() * 10).toString(),
    ).join('');
  if (!new RegExp(`^[0-9]{${serialLen}}$`).test(s)) return undefined;
  const base = `89${mccmnc}${s}`;
  return base + luhnCheckDigit(base).toString();
}
