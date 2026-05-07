/**
 * Auto-provisioning rules for user variables.
 *
 * Trigger points (per the design plan):
 *   - adding / renaming an MSCC service in `multi-mscc` mode
 *   - flipping `serviceModel`
 *   - adding a step that introduces new placeholders
 *
 * For `multi-mscc` scenarios, generated names are prefixed `RG<rg>_`
 * (e.g. `RG100_RSU_TOTAL`); for `root` and `single-mscc`, names stay
 * flat (`RSU_TOTAL`). The provisioner only fills *missing* variables —
 * it never overwrites a variable the user has already defined.
 */
import type { Scenario, Variable } from './types';

function ensureVar(
  list: Variable[],
  name: string,
  factory: () => Variable,
): Variable[] {
  if (list.some((v) => v.name === name)) return list;
  return [...list, factory()];
}

/**
 * Return a list of variables that includes everything currently in
 * `scenario.variables` plus any auto-provisioned name implied by the
 * current services / serviceModel that wasn't already declared.
 */
export function provisionVariables(scenario: Scenario): Variable[] {
  let next = scenario.variables;
  const isMulti = scenario.serviceModel === 'multi-mscc';

  if (scenario.serviceModel === 'root') {
    next = ensureVar(next, 'RSU_TOTAL', () => ({
      name: 'RSU_TOTAL',
      description: 'Auto-provisioned: requested service-units quantity.',
      source: {
        kind: 'generator',
        strategy: 'literal',
        refresh: 'once',
        params: { value: 0 },
      },
    }));
    next = ensureVar(next, 'USU_TOTAL', () => ({
      name: 'USU_TOTAL',
      description: 'Auto-provisioned: used service-units quantity.',
      source: {
        kind: 'generator',
        strategy: 'literal',
        refresh: 'once',
        params: { value: 0 },
      },
    }));
    return next;
  }

  scenario.services.forEach((svc, idx) => {
    // Position-based prefix: RG1_, RG2_, … — matches the engine's CCA indexing.
    const pos = idx + 1;
    const prefix = isMulti ? `RG${pos}_` : '';

    if (svc.ratingGroup) {
      const name = `${prefix}RATING_GROUP`;
      next = ensureVar(next, name, () => ({
        name,
        description: `Auto-provisioned rating-group for service ${pos}.`,
        source: {
          kind: 'generator',
          strategy: 'literal',
          refresh: 'once',
          params: { value: Number(svc.id) || 0 },
        },
      }));
    }
    if (svc.requestedUnits) {
      const name = `${prefix}UNITS_REQ`;
      next = ensureVar(next, name, () => ({
        name,
        description: `Auto-provisioned requested service-units for service ${pos}.`,
        source: {
          kind: 'generator',
          strategy: 'literal',
          refresh: 'once',
          params: { value: 0 },
        },
      }));
    }
    if (svc.usedUnits) {
      // System variable written by the engine from the CCA — position-keyed.
      const grantedVar = isMulti ? `RG${pos}_GRANTED_UNITS` : 'RG_GRANTED_UNITS';
      const name = `${prefix}UNITS_USED`;
      next = ensureVar(next, name, () => ({
        name,
        description: `Auto-provisioned used service-units for service ${pos}. Randomised each send, capped to what the OCS granted.`,
        source: {
          kind: 'generator',
          strategy: 'random-int',
          refresh: 'per-send',
          params: { min: 1, max: `{{${grantedVar}}}` },
        },
      }));
    }
  });

  return next;
}
