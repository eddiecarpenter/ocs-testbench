/**
 * Validate a JSON blob against the Scenario shape.
 *
 * Two-stage validation:
 *   1. Parse JSON — surface a single error on parse failure.
 *   2. Validate via zod — surface field-level errors keyed by JSON
 *      path so the Builder can render them inline.
 *
 * Returns either the validated value or a structured error object;
 * never throws (the caller is in a UI context).
 */
import { type ScenarioImport, scenarioInputSchema } from './scenarioSchema';

export type ImportError = {
  kind: 'parse';
  message: string;
} | {
  kind: 'schema';
  errors: { path: string; message: string }[];
};

export type ImportResult =
  | { ok: true; value: ScenarioImport }
  | { ok: false; error: ImportError };

export function parseAndValidateScenarioJson(raw: string): ImportResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (e) {
    return {
      ok: false,
      error: {
        kind: 'parse',
        message: (e as Error).message ?? 'Could not parse JSON',
      },
    };
  }

  const result = scenarioInputSchema.safeParse(parsed);
  if (!result.success) {
    return {
      ok: false,
      error: {
        kind: 'schema',
        errors: result.error.issues.map((iss) => ({
          path: iss.path.length === 0 ? '/' : '/' + iss.path.join('/'),
          message: iss.message,
        })),
      },
    };
  }

  return { ok: true, value: result.data };
}
