/**
 * Re-exports of every Scenario-shaped type from the generated OpenAPI
 * client so feature code never hand-rolls a scenario type.
 *
 * Source of truth: `src/api/schema.d.ts` (regenerated from OpenAPI v0.2
 * via `npm run gen:api`). If a field is wrong here, fix the spec and
 * re-run the generator — do not patch the type locally.
 */
import type { components } from '../../api/schema';

export type Scenario = components['schemas']['Scenario'];
export type ScenarioInput = components['schemas']['ScenarioInput'];
export type ScenarioSummary = components['schemas']['ScenarioSummary'];
export type ScenarioDuplicateInput =
  components['schemas']['ScenarioDuplicateInput'];
export type ScenarioOrigin = components['schemas']['ScenarioOrigin'];

export type UnitType = components['schemas']['UnitType'];
export type SessionMode = components['schemas']['SessionMode'];
export type ServiceModel = components['schemas']['ServiceModel'];
export type RequestType = components['schemas']['RequestType'];

export type AvpNode = components['schemas']['AvpNode'];
export type Service = components['schemas']['Service'];
export type Variable = components['schemas']['Variable'];
export type VariableSource = components['schemas']['VariableSource'];
export type VariableSourceGenerator =
  components['schemas']['VariableSourceGenerator'];
export type VariableSourceBound = components['schemas']['VariableSourceBound'];
export type VariableSourceExtracted =
  components['schemas']['VariableSourceExtracted'];
export type GeneratorStrategy = components['schemas']['GeneratorStrategy'];
export type GeneratorRefresh = components['schemas']['GeneratorRefresh'];

export type ScenarioStep = components['schemas']['ScenarioStep'];
export type RequestStep = components['schemas']['RequestStep'];
export type ConsumeStep = components['schemas']['ConsumeStep'];
export type WaitStep = components['schemas']['WaitStep'];
export type PauseStep = components['schemas']['PauseStep'];
export type ServiceSelection = components['schemas']['ServiceSelection'];
export type VarValue = components['schemas']['VarValue'];

export type Problem = components['schemas']['Problem'];
