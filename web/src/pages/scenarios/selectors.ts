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
 * Rename every reference to a variable inside a Scenario, returning a
 * new (deeply-cloned where necessary) Scenario. Applied to the same
 * surfaces that `findUsages` scans:
 *
 *   - AVP tree leaf `valueRef`s (recursive)
 *   - Service `ratingGroup`, `serviceIdentifier`, `requestedUnits`,
 *     `usedUnits`
 *   - Step override map keys (the value is preserved)
 *
 * The variable list itself is NOT modified — callers are expected to
 * also rename the `Variable` entry. Pure function; no-op when
 * `oldName === newName`.
 */
export function renameUsages(
  scenario: Scenario,
  oldName: string,
  newName: string,
): Scenario {
  if (oldName === newName) return scenario;

  const renameAvpTree = (tree: AvpNode[]): AvpNode[] =>
    tree.map((node) => ({
      ...node,
      valueRef: node.valueRef === oldName ? newName : node.valueRef,
      children: node.children ? renameAvpTree(node.children) : node.children,
    }));

  const renameField = (v: string | undefined): string | undefined =>
    v === oldName ? newName : v;

  return {
    ...scenario,
    avpTree: renameAvpTree(scenario.avpTree),
    services: scenario.services.map((svc) => ({
      ...svc,
      ratingGroup: renameField(svc.ratingGroup),
      serviceIdentifier: renameField(svc.serviceIdentifier),
      requestedUnits: svc.requestedUnits === oldName ? newName : svc.requestedUnits,
      usedUnits: renameField(svc.usedUnits),
    })),
    steps: scenario.steps.map((step) => {
      if (
        (step.kind === 'request' || step.kind === 'consume') &&
        step.overrides &&
        Object.prototype.hasOwnProperty.call(step.overrides, oldName)
      ) {
        const next: Record<string, unknown> = {};
        for (const [k, v] of Object.entries(step.overrides)) {
          next[k === oldName ? newName : k] = v;
        }
        return { ...step, overrides: next as typeof step.overrides };
      }
      return step;
    }),
  };
}

/**
 * Mantine `Select` group spec produced by `buildVariableOptions`.
 * Keys match the shape Mantine expects for grouped data.
 */
export interface VariableOptionGroup {
  group: string;
  items: { value: string; label: string }[];
}

/**
 * Build the grouped variable options used by every place that lets a
 * user pick a variable name (Frame's Value reference, Services' RSU /
 * USU / RG / Service-id fields). Stored value is the bare name; the
 * displayed label is wrapped in `{{ }}` so the user can tell at a
 * glance that the field holds a variable reference.
 *
 * User names that collide with a System name are dropped from the
 * User group — System wins, the dropdown stays unambiguous.
 */
export function buildVariableOptions(
  userVariables: { name: string }[],
): { options: VariableOptionGroup[]; hasAny: boolean } {
  const systemNames = listSystemVariables().map((v) => v.name);
  const systemSet = new Set(systemNames);
  const userNames = userVariables
    .map((v) => v.name)
    .filter((name) => !systemSet.has(name));

  const wrap = (name: string) => ({ value: name, label: `{{${name}}}` });

  const options: VariableOptionGroup[] = [];
  if (systemNames.length > 0) {
    options.push({ group: 'System', items: systemNames.map(wrap) });
  }
  if (userNames.length > 0) {
    options.push({ group: 'User', items: userNames.map(wrap) });
  }
  return { options, hasAny: systemNames.length + userNames.length > 0 };
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
    {
      name: 'SERVICE_CONTEXT_ID',
      kind: 'bound',
      description:
        'Service-Context-Id (RFC 4006 §5.1.1.4) — defaults to 32251@3gpp.org.',
    },
    {
      name: 'AUTH_APP_ID',
      kind: 'bound',
      description: 'Auth-Application-Id — always 4 for Diameter Credit-Control (Gy).',
    },
    {
      name: 'SUB_ID_TYPE',
      kind: 'generator',
      description: 'Subscription-Id-Type — 0 = END_USER_E164 (MSISDN).',
    },
    {
      name: 'MSISDN',
      kind: 'bound',
      description: 'Subscriber MSISDN from the bound subscriber.',
    },
    {
      name: 'CALLING_PARTY_ADDRESS',
      kind: 'bound',
      description: 'Calling-party address — auto-set from subscriber MSISDN.',
    },

    // — Accumulation variables — updated by the engine after each send —

    {
      name: 'TOTAL_USU',
      kind: 'generator',
      description: 'Cumulative Used-Service-Unit total for the session — sum of USU_TOTAL across all UPDATE and TERMINATE steps sent so far.',
    },

    // — CCA auto-variables — written by the engine after each send —

    {
      name: 'RESULT_CODE',
      kind: 'generator',
      description: 'Top-level Result-Code from the last CCA (e.g. 2001 = SUCCESS, 4012 = QUOTA_EXHAUSTED).',
    },
    {
      name: 'FUI_ACTION',
      kind: 'generator',
      description: 'Root-level Final-Unit-Action from the last CCA (-1 = absent, 0 = TERMINATE).',
    },
    {
      name: 'RGn_GRANTED',
      kind: 'generator',
      description: 'Granted-Service-Unit (time, seconds) for rating group n — e.g. RG1_GRANTED.',
    },
    {
      name: 'RGn_GRANTED_OCTETS',
      kind: 'generator',
      description: 'Granted-Service-Unit (total octets) for rating group n — e.g. RG1_GRANTED_OCTETS.',
    },
    {
      name: 'RGn_VALIDITY',
      kind: 'generator',
      description: 'Validity-Time (seconds) granted for rating group n — e.g. RG1_VALIDITY.',
    },
    {
      name: 'RGn_RESULT_CODE',
      kind: 'generator',
      description: 'Per-MSCC Result-Code for rating group n — e.g. RG1_RESULT_CODE (0 = absent).',
    },
    {
      name: 'RGn_FUI_ACTION',
      kind: 'generator',
      description: 'Per-MSCC Final-Unit-Action for rating group n — e.g. RG1_FUI_ACTION (-1 = absent, 0 = TERMINATE).',
    },
  ];
}
