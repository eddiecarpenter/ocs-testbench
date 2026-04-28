/**
 * Cross-tab selectors that reach into a `Scenario` to compute
 * derived data — currently variable usage. Pure functions, easy
 * to test, no dependency on React or the store.
 */
import type { AvpNode, Scenario } from './types';

export interface UsageRef {
  /** Where the reference was found. */
  location:
    | { kind: 'avp'; path: number[]; nodeName: string }
    | { kind: 'service'; index: number; field: 'ratingGroup' | 'serviceIdentifier' | 'requestedUnits' | 'usedUnits' }
    | { kind: 'step-override'; stepIndex: number };
  /** Tab to open when the user clicks through. */
  tab: 'frame' | 'services' | 'steps';
  /** Stable URL params (`select=…&field=…`) for deep linking. */
  select?: string;
  field?: string;
  /** Human-readable label for the Usage row. */
  label: string;
}

/** Walk the AVP tree, calling `cb` for every leaf with a valueRef. */
function walkAvp(
  tree: AvpNode[],
  path: number[],
  cb: (node: AvpNode, path: number[]) => void,
) {
  tree.forEach((node, i) => {
    const here = [...path, i];
    cb(node, here);
    if (node.children) walkAvp(node.children, here, cb);
  });
}

export function findUsages(scenario: Scenario, varName: string): UsageRef[] {
  const refs: UsageRef[] = [];

  // 1. AVP tree value refs
  walkAvp(scenario.avpTree, [], (node, path) => {
    if (node.valueRef === varName) {
      refs.push({
        location: { kind: 'avp', path, nodeName: node.name },
        tab: 'frame',
        select: `avp:${path.join('.')}`,
        label: `Frame · ${node.name} (code ${node.code})`,
      });
    }
  });

  // 2. Service fields
  scenario.services.forEach((svc, i) => {
    (
      ['ratingGroup', 'serviceIdentifier', 'requestedUnits', 'usedUnits'] as const
    ).forEach((field) => {
      if (svc[field] === varName) {
        refs.push({
          location: { kind: 'service', index: i, field },
          tab: 'services',
          select: `mscc:${svc.id}`,
          field,
          label: `Services · ${svc.id || 'root'} · ${field}`,
        });
      }
    });
  });

  // 3. Step overrides
  scenario.steps.forEach((step, i) => {
    if (
      (step.kind === 'request' || step.kind === 'consume') &&
      step.overrides &&
      Object.prototype.hasOwnProperty.call(step.overrides, varName)
    ) {
      refs.push({
        location: { kind: 'step-override', stepIndex: i },
        tab: 'steps',
        select: `step:${i}`,
        label: `Steps · step ${i + 1} · override`,
      });
    }
  });

  return refs;
}

/**
 * System variables — auto-provisioned at run time, do not appear in
 * `scenario.variables` per the OpenAPI contract. The Variables tab
 * surfaces them for completeness so the user knows what names the
 * runtime will populate.
 */
export interface SystemVariable {
  name: string;
  kind: 'generator' | 'bound';
  description: string;
}

export function listSystemVariables(): SystemVariable[] {
  return [
    {
      name: 'SESSION_ID',
      kind: 'generator',
      description: 'Per-session monotonic UUID assigned by the engine.',
    },
    {
      name: 'CHARGING_ID',
      kind: 'generator',
      description: 'Engine-assigned Diameter charging session id.',
    },
    {
      name: 'CC_REQUEST_NUMBER',
      kind: 'generator',
      description: 'Auto-incrementing CCR sequence — engine-managed.',
    },
    {
      name: 'ORIGIN_HOST',
      kind: 'bound',
      description: 'Engine origin-host from the bound peer.',
    },
    {
      name: 'ORIGIN_REALM',
      kind: 'bound',
      description: 'Engine origin-realm from the bound peer.',
    },
    {
      name: 'DEST_REALM',
      kind: 'bound',
      description: 'Destination-realm from the bound peer.',
    },
  ];
}
