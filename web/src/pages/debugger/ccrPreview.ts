/**
 * CCR preview resolver.
 *
 * Pure helper that takes a scenario, a step index, the current variable
 * context, and the services enabled for this step, and returns a fresh
 * AVP tree representing the CCR that *would* be sent.
 *
 * The resolver:
 *   - never mutates the input scenario or AVP tree (§8 invariant 1);
 *   - resolves every `valueRef` against `contextVars`, falling back to
 *     `{{<name>}}` when the variable is not bound;
 *   - splices in service AVPs per the scenario's `serviceModel` —
 *     `root`: RSU/USU at root; `single-mscc`: one MSCC wrapper;
 *     `multi-mscc`: one MSCC per enabled service.
 *
 * The output is a `PreviewAvpNode[]` (a sibling type of the schema's
 * `AvpNode` with a resolved `value` field for display). The view layer
 * walks the tree to render the preview pane.
 *
 * Design contract — referenced by Tasks 5 (Step editor middle pane)
 * and 8 (verification). A real engine port could re-use the same
 * substitution rules without React or Zustand.
 */
import type {
  AvpNode,
  Scenario,
  Service,
} from '../scenarios/types';

import type { components } from '../../api/schema';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type VarValue = components['schemas']['VarValue'];

/**
 * Per-step context map. Flat `name → value` — system + user + extracted
 * variables flattened by the caller.
 */
export type ContextVars = Readonly<Record<string, VarValue>>;

/**
 * AVP node augmented with a resolved `value` field. The `valueRef` is
 * preserved alongside so the view can show the user "this value comes
 * from {variable}".
 */
export interface PreviewAvpNode {
  name: string;
  code: number;
  vendorId?: number;
  children?: PreviewAvpNode[];
  /** Source variable name, when this leaf was a `valueRef`. */
  valueRef?: string;
  /**
   * Resolved value rendered as a display string. When `valueRef` resolves
   * in `contextVars`, this is the stringified value; when missing, this
   * is the literal `{{<name>}}` placeholder so the user can see which
   * variables are unbound.
   */
  value?: string;
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Resolve a CCR preview tree for the step at `stepIndex`.
 *
 * `servicesEnabled` is honoured only for `multi-mscc` scenarios; in
 * `root` and `single-mscc` the single service is implicit and the
 * argument is ignored.
 */
export function resolveCcrPreview(
  scenario: Scenario,
  // `stepIndex` is reserved for per-step variation (e.g. CCR-T hides
  // USU-only services). MVP does not use it but the signature pins
  // the extension point so consumers don't have to adjust later.
  _stepIndex: number,
  contextVars: ContextVars,
  servicesEnabled: ReadonlySet<string>,
): PreviewAvpNode[] {
  // 1. Resolve the scenario's frozen avpTree against contextVars. This
  //    produces a fresh tree — the input is never mutated.
  const root = scenario.avpTree.map((n) => resolveNode(n, contextVars));

  // 2. Splice in the service AVPs per `serviceModel`.
  switch (scenario.serviceModel) {
    case 'root':
      // The single implicit service contributes RSU/USU at root level.
      return [...root, ...buildRootServiceAvps(scenario, contextVars)];
    case 'single-mscc': {
      const single = scenario.services[0];
      if (!single) return root;
      const mscc = buildMsccBlock(single, contextVars);
      return [...root, msi(0), mscc];
    }
    case 'multi-mscc': {
      const enabled = scenario.services.filter((s) => servicesEnabled.has(s.id));
      const blocks = enabled.map((s) => buildMsccBlock(s, contextVars));
      return [...root, msi(1), ...blocks];
    }
    default:
      return root;
  }
}

// ---------------------------------------------------------------------------
// Internals — AVP tree resolution
// ---------------------------------------------------------------------------

function resolveNode(node: AvpNode, contextVars: ContextVars): PreviewAvpNode {
  const out: PreviewAvpNode = {
    name: node.name,
    code: node.code,
    ...(node.vendorId !== undefined ? { vendorId: node.vendorId } : {}),
  };
  if (node.children && node.children.length > 0) {
    out.children = node.children.map((c) => resolveNode(c, contextVars));
  }
  if (node.valueRef !== undefined) {
    out.valueRef = node.valueRef;
    out.value = resolveValue(node.valueRef, contextVars);
  }
  return out;
}

/**
 * Resolve a variable name against `contextVars`. Missing variables
 * render as the literal `{{<name>}}` so the user can see which values
 * are unbound — matching the convention from the Builder.
 */
function resolveValue(name: string, contextVars: ContextVars): string {
  const raw = contextVars[name];
  if (raw === undefined) return `{{${name}}}`;
  if (raw === null) return 'null';
  return String(raw);
}

// ---------------------------------------------------------------------------
// Internals — service AVPs
// ---------------------------------------------------------------------------

/**
 * For `root` mode the single service's RSU (and optionally USU) are
 * spliced directly under the CCR root with no MSCC wrapper.
 */
function buildRootServiceAvps(
  scenario: Scenario,
  contextVars: ContextVars,
): PreviewAvpNode[] {
  const svc = scenario.services[0];
  if (!svc) return [];
  const out: PreviewAvpNode[] = [];
  out.push(rsuLeaf(svc, contextVars));
  if (svc.usedUnits) out.push(usuLeaf(svc, contextVars));
  return out;
}

function buildMsccBlock(
  service: Service,
  contextVars: ContextVars,
): PreviewAvpNode {
  const children: PreviewAvpNode[] = [];
  if (service.ratingGroup) {
    children.push({
      name: 'Rating-Group',
      code: 432,
      valueRef: service.ratingGroup,
      value: resolveValue(service.ratingGroup, contextVars),
    });
  }
  if (service.serviceIdentifier) {
    children.push({
      name: 'Service-Identifier',
      code: 439,
      valueRef: service.serviceIdentifier,
      value: resolveValue(service.serviceIdentifier, contextVars),
    });
  }
  children.push(rsuLeaf(service, contextVars));
  if (service.usedUnits) children.push(usuLeaf(service, contextVars));
  return {
    name: 'Multiple-Services-Credit-Control',
    code: 456,
    children,
  };
}

function rsuLeaf(svc: Service, contextVars: ContextVars): PreviewAvpNode {
  return {
    name: 'Requested-Service-Unit',
    code: 437,
    valueRef: svc.requestedUnits,
    value: resolveValue(svc.requestedUnits, contextVars),
  };
}

function usuLeaf(svc: Service, contextVars: ContextVars): PreviewAvpNode {
  // Caller checked that usedUnits is set, but TypeScript doesn't track
  // the narrowing across the helper boundary. Assert here.
  if (!svc.usedUnits) {
    throw new Error('rsuLeaf called without svc.usedUnits');
  }
  return {
    name: 'Used-Service-Unit',
    code: 446,
    valueRef: svc.usedUnits,
    value: resolveValue(svc.usedUnits, contextVars),
  };
}

/**
 * Multiple-Services-Indicator leaf. Constant-valued; the engine emits
 * it implicitly so it doesn't show in `scenario.avpTree`. The preview
 * surfaces it for honest debugging.
 */
function msi(value: 0 | 1): PreviewAvpNode {
  return {
    name: 'Multiple-Services-Indicator',
    code: 455,
    value: String(value),
  };
}

