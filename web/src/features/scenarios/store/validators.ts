/**
 * Pure validators for the Scenario Builder.
 *
 * The matrix lives here so it is exercised both by the UI (disable
 * segments + hint) and by the Save flow (block + surface). Keeping
 * one source of truth prevents the two layers drifting.
 */
import type {
  ServiceModel,
  SessionMode,
  UnitType,
} from './types';

export interface MatrixCell {
  allowed: boolean;
  /** Human-readable hint shown when `allowed === false`. */
  hint?: string;
}

/**
 * `serviceModel × unitType` allowed combinations (architecture §4).
 *
 *                | OCTET | TIME  | UNITS |
 *   ------------ | ----- | ----- | ----- |
 *   root         | n/a   | OK    | OK    |
 *   single-mscc  | OK    | OK    | OK    |
 *   multi-mscc   | OK    | n/a   | n/a   |
 */
export function matrix(
  unit: UnitType,
  model: ServiceModel,
): MatrixCell {
  if (model === 'root' && unit === 'OCTET') {
    return {
      allowed: false,
      hint: 'OCTET requires MSCC. Use Single or Multi MSCC.',
    };
  }
  if (model === 'multi-mscc' && unit !== 'OCTET') {
    return {
      allowed: false,
      hint: 'Multi-MSCC is only valid with OCTET unit type.',
    };
  }
  return { allowed: true };
}

export interface ValidationIssue {
  /** Field path — uses JSON-Pointer-style key for compatibility with ApiError.errors. */
  path: string;
  message: string;
}

/** Collect every UI-side validation issue into a flat list. */
export function validateScenario(input: {
  unitType: UnitType;
  serviceModel: ServiceModel;
  sessionMode: SessionMode;
  steps: { kind: string; requestType?: string }[];
}): ValidationIssue[] {
  const issues: ValidationIssue[] = [];

  const cell = matrix(input.unitType, input.serviceModel);
  if (!cell.allowed) {
    issues.push({
      path: '/serviceModel',
      message: cell.hint ?? 'serviceModel × unitType combination not allowed',
    });
  }

  // sessionMode × requestType — the Steps tab also surfaces this inline.
  for (let i = 0; i < input.steps.length; i += 1) {
    const s = input.steps[i];
    if (s.kind !== 'request' || !s.requestType) continue;
    if (input.sessionMode === 'session' && s.requestType === 'EVENT') {
      issues.push({
        path: `/steps/${i}/requestType`,
        message: 'EVENT is only legal under sessionMode=event',
      });
    }
    if (input.sessionMode === 'event' && s.requestType !== 'EVENT') {
      issues.push({
        path: `/steps/${i}/requestType`,
        message: 'Only EVENT is legal under sessionMode=event',
      });
    }
  }

  return issues;
}
