/**
 * IMEI helpers — Luhn check-digit and IMEI composition.
 *
 * An IMEI is 15 digits:
 *   ┌ 8 digits ┬ 6 digits ┬ 1 digit ┐
 *   │   TAC    │  serial  │  Luhn   │
 *   └──────────┴──────────┴─────────┘
 *
 * The TAC (Type Allocation Code) identifies the device manufacturer +
 * model. The serial is assigned by the manufacturer. The final digit is
 * a Luhn mod-10 check over the first 14 digits.
 *
 * These helpers live under `api/` rather than a UI component because the
 * same rules are enforced by the backend — parity is easier to keep when
 * the definitions sit alongside the API schema types.
 */

/**
 * Compute the Luhn check digit (single digit, 0–9) for the given digit
 * string. Pass in the first 14 digits of an IMEI to derive the 15th.
 * Returns -1 for non-digit input.
 */
export function luhnCheckDigit(digits: string): number {
  if (!/^[0-9]+$/.test(digits)) return -1;
  // Walk right-to-left doubling every second digit (the ones in the
  // even-indexed positions from the right, i.e. the ones immediately
  // "below" the check-digit position).
  let sum = 0;
  for (let i = 0; i < digits.length; i++) {
    const d = digits.charCodeAt(digits.length - 1 - i) - 48;
    const doubled = i % 2 === 0 ? d * 2 : d;
    sum += doubled > 9 ? doubled - 9 : doubled;
  }
  return (10 - (sum % 10)) % 10;
}

/** True when `imei` is a 15-digit string with a valid Luhn check digit. */
export function isValidImei(imei: string): boolean {
  if (!/^[0-9]{15}$/.test(imei)) return false;
  const check = luhnCheckDigit(imei.slice(0, 14));
  return check === Number(imei[14]);
}

/**
 * Compose a full IMEI from an 8-digit TAC and an optional 6-digit
 * serial. When the serial is omitted, six random digits are rolled.
 * Returns `undefined` when the TAC is malformed.
 */
export function buildImei(tac: string, serial?: string): string | undefined {
  if (!/^[0-9]{8}$/.test(tac)) return undefined;
  const s = serial ?? String(Math.floor(Math.random() * 1_000_000)).padStart(6, '0');
  if (!/^[0-9]{6}$/.test(s)) return undefined;
  const base = tac + s;
  return base + luhnCheckDigit(base).toString();
}
