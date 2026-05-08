/**
 * Pure validators for the Scenario Builder.
 *
 * The matrix lives here so it is exercised both by the UI (disable
 * segments + hint) and by the Save flow (block + surface). Keeping
 * one source of truth prevents the two layers drifting.
 */
import type {
  Service,
  ServiceModel,
  ServiceType,
  SessionMode,
  UnitType,
} from './types';

export interface MatrixCell {
  allowed: boolean;
  /** Human-readable hint shown when `allowed === false`. */
  hint?: string;
}

/** Derive the logical unit type (for the service-model matrix) from service type. */
export function deriveUnitType(serviceType: ServiceType | undefined): UnitType {
  switch (serviceType) {
    case 'VOICE':
    case 'USSD2_SESSION':
      return 'TIME';
    case 'DATA':
      return 'VOLUME';
    default:
      return 'EVENT';
  }
}

/**
 * `serviceModel × unitType` allowed combinations.
 *
 *                | VOLUME | TIME  | EVENT |
 *   ------------ | ------ | ----- | ----- |
 *   root         | n/a    | OK    | OK    |
 *   single-mscc  | OK     | OK    | OK    |
 *   multi-mscc   | OK     | n/a   | n/a   |
 */
export function matrix(
  unit: UnitType,
  model: ServiceModel,
): MatrixCell {
  if (model === 'root' && unit === 'VOLUME') {
    return {
      allowed: false,
      hint: 'VOLUME requires MSCC. Use Single or Multi MSCC.',
    };
  }
  if (model === 'multi-mscc' && unit !== 'VOLUME') {
    return {
      allowed: false,
      hint: 'Multi-MSCC is only valid with VOLUME unit type.',
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
  serviceType: ServiceType | undefined;
  serviceModel: ServiceModel;
  sessionMode: SessionMode;
  services: Service[];
  steps: { kind: string; requestType?: string }[];
}): ValidationIssue[] {
  const issues: ValidationIssue[] = [];

  const unitType = deriveUnitType(input.serviceType);
  const cell = matrix(unitType, input.serviceModel);
  if (!cell.allowed) {
    issues.push({
      path: '/serviceModel',
      message: cell.hint ?? 'serviceModel × serviceType combination not allowed',
    });
  }

  // Rating-Group is mandatory for MSCC service models. The OCS uses it to
  // identify the quota block in the CCA, and the engine uses it to correlate
  // granted units with the correct MSCC block.
  if (input.serviceModel === 'single-mscc' || input.serviceModel === 'multi-mscc') {
    input.services.forEach((svc, i) => {
      if (!svc.ratingGroup) {
        issues.push({
          path: `/services/${i}/ratingGroup`,
          message: `Service ${i + 1}: Rating-Group is required for ${input.serviceModel} scenarios`,
        });
      }
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
