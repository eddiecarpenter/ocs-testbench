/**
 * Pure helpers for the Step Editor pane.
 *
 * Lifted out of the React component so the resolver wiring, the
 * default-services computation, and the step-header rendering are
 * unit-testable without a DOM.
 */
import type { ContextVars } from './ccrPreview';
import { resolveCcrPreview, type PreviewAvpNode } from './ccrPreview';
import type {
  ExecutionContextSnapshot,
} from '../../api/resources/executions';
import type {
  Scenario,
  ScenarioStep,
} from '../scenarios/types';

// ---------------------------------------------------------------------------
// Default services for a given step
// ---------------------------------------------------------------------------

/**
 * The set of service ids that are *implicitly enabled* for `stepIndex`
 * based on the scenario. Used to seed `edit.servicesEnabled` when the
 * cursor advances and as the target of "Revert all".
 *
 * - root / single-mscc: the single service is always implicitly
 *   selected (the user cannot toggle it off).
 * - multi-mscc + step has `services.mode === 'fixed'`: the listed
 *   ids form the default.
 * - multi-mscc + step has `services.mode === 'random'`: the `from`
 *   pool is the default; the engine picks a sub-set at send time.
 *   Mocking that randomness is out of scope here.
 * - non-request kinds (consume / wait / pause): default to all
 *   services so the preview renders something coherent.
 */
export function defaultServicesForStep(
  scenario: Scenario,
  stepIndex: number,
): Set<string> {
  const step: ScenarioStep | undefined = scenario.steps[stepIndex];
  const allIds = scenario.services.map((s) => s.id);

  if (scenario.serviceModel !== 'multi-mscc') {
    return new Set(allIds);
  }
  if (!step || step.kind !== 'request' || !('services' in step) || !step.services) {
    return new Set(allIds);
  }
  if (step.services.mode === 'fixed') {
    return new Set(step.services.serviceIds);
  }
  // random: pre-select the candidate pool — the engine resolves the
  // count at send time. The preview shows what *could* be sent.
  return new Set(step.services.from);
}

// ---------------------------------------------------------------------------
// Context flattening
// ---------------------------------------------------------------------------

/**
 * Flatten the `Execution.context` snapshot into the flat `name → value`
 * map the resolver expects. Keys collide-resolve in
 * `extracted` < `system` < `user` order — user-supplied values win.
 */
export function flattenContext(
  snapshot: ExecutionContextSnapshot,
): ContextVars {
  return {
    ...snapshot.extracted,
    ...snapshot.system,
    ...snapshot.user,
  };
}

// ---------------------------------------------------------------------------
// Preview resolution wrapper
// ---------------------------------------------------------------------------

/**
 * One-call wrapper around the Task 3 resolver — exists so the pane and
 * the unit test reach for the same code path. Not just a thin alias:
 * it builds the `Set<string>` (Task 3 takes a `ReadonlySet`) and
 * normalises the inputs.
 */
export function resolvePreview(
  scenario: Scenario,
  stepIndex: number,
  contextVars: ContextVars,
  servicesEnabled: ReadonlySet<string>,
): PreviewAvpNode[] {
  return resolveCcrPreview(scenario, stepIndex, contextVars, servicesEnabled);
}

// ---------------------------------------------------------------------------
// Step header rendering
// ---------------------------------------------------------------------------

export interface StepHeader {
  title: string;
  kindLabel: string;
  /** Mantine palette key for the kind chip. */
  kindColor: string;
}

export function buildStepHeader(
  scenario: Scenario,
  stepIndex: number,
): StepHeader | null {
  const step = scenario.steps[stepIndex];
  if (!step) return null;
  switch (step.kind) {
    case 'request':
      return {
        title: requestTitle(step.requestType),
        kindLabel: 'Request',
        kindColor: 'blue',
      };
    case 'consume':
      return {
        title: 'Consume loop',
        kindLabel: 'Consume',
        kindColor: 'grape',
      };
    case 'wait':
      return {
        title: `Wait ${step.durationMs} ms`,
        kindLabel: 'Wait',
        kindColor: 'gray',
      };
    case 'pause':
      return {
        title: step.label ?? 'Pause',
        kindLabel: 'Pause',
        kindColor: 'yellow',
      };
    default:
      return null;
  }
}

function requestTitle(rt: 'INITIAL' | 'UPDATE' | 'TERMINATE' | 'EVENT'): string {
  switch (rt) {
    case 'INITIAL':
      return 'CCR-INITIAL — start of session';
    case 'UPDATE':
      return 'CCR-UPDATE — interim charge / refill';
    case 'TERMINATE':
      return 'CCR-TERMINATE — end of session';
    case 'EVENT':
      return 'CCR-EVENT — one-shot charge';
    default:
      return rt;
  }
}
