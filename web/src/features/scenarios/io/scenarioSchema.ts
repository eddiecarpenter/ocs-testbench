/**
 * Runtime zod schema mirroring the OpenAPI v0.2 Scenario shape.
 *
 * Used at the Import boundary — TypeScript verifies the shape at
 * compile time, zod verifies it at run time when the user uploads
 * a JSON file. Keep this in sync with `schema.d.ts`. If OpenAPI
 * changes, regenerate types AND update this schema in the same
 * commit (per the task notes).
 */
import { z } from 'zod';

export const unitTypeSchema = z.enum(['OCTET', 'TIME', 'UNITS']);
export const sessionModeSchema = z.enum(['session', 'event']);
export const serviceModelSchema = z.enum([
  'root',
  'single-mscc',
  'multi-mscc',
]);
export const scenarioOriginSchema = z.enum(['system', 'user']);
export const requestTypeSchema = z.enum([
  'INITIAL',
  'UPDATE',
  'TERMINATE',
  'EVENT',
]);

const varValueSchema = z.union([
  z.string(),
  z.number(),
  z.boolean(),
  z.null(),
]);

const variableSourceGeneratorSchema = z.object({
  kind: z.literal('generator'),
  strategy: z.enum([
    'literal',
    'uuid',
    'incrementer',
    'random-int',
    'random-string',
    'random-choice',
  ]),
  refresh: z.enum(['once', 'per-send']),
  params: z.record(z.string(), z.unknown()).optional(),
});

const variableSourceBoundSchema = z.object({
  kind: z.literal('bound'),
  from: z.enum(['subscriber', 'peer', 'config', 'step']),
  field: z.string(),
});

const variableSourceExtractedSchema = z.object({
  kind: z.literal('extracted'),
  path: z.string(),
  transform: z.string().optional(),
});

const variableSchema = z.object({
  name: z.string(),
  description: z.string().optional(),
  source: z.discriminatedUnion('kind', [
    variableSourceGeneratorSchema,
    variableSourceBoundSchema,
    variableSourceExtractedSchema,
  ]),
});

// Recursive schema for AvpNode — z.lazy for self-reference.
type AvpNodeSchemaShape = {
  name: string;
  code: number;
  vendorId?: number;
  children?: AvpNodeSchemaShape[];
  valueRef?: string;
};
export const avpNodeSchema: z.ZodType<AvpNodeSchemaShape> = z.lazy(() =>
  z.object({
    name: z.string(),
    code: z.number(),
    vendorId: z.number().optional(),
    children: z.array(avpNodeSchema).optional(),
    valueRef: z.string().optional(),
  }),
);

const serviceSchema = z.object({
  id: z.string(),
  ratingGroup: z.string().optional(),
  serviceIdentifier: z.string().optional(),
  requestedUnits: z.string(),
  usedUnits: z.string().optional(),
});

const serviceSelectionSchema = z.union([
  z.object({
    mode: z.literal('fixed'),
    serviceIds: z.array(z.string()),
  }),
  z.object({
    mode: z.literal('random'),
    from: z.array(z.string()),
    count: z.union([
      z.number(),
      z.object({ min: z.number(), max: z.number() }),
    ]),
  }),
]);

const requestStepSchema = z.object({
  kind: z.literal('request'),
  requestType: requestTypeSchema,
  services: serviceSelectionSchema.optional(),
  overrides: z.record(z.string(), varValueSchema).optional(),
  assertions: z.array(z.string()).optional(),
  guards: z.array(z.string()).optional(),
  resultHandlers: z
    .array(
      z.object({
        when: z.string(),
        action: z.enum(['continue', 'stop', 'retry']),
      }),
    )
    .optional(),
});

const consumeStepSchema = z.object({
  kind: z.literal('consume'),
  services: serviceSelectionSchema.optional(),
  windowMs: z.number(),
  maxRounds: z.number().optional(),
  terminateWhen: z.string().optional(),
  overrides: z.record(z.string(), varValueSchema).optional(),
  assertions: z.array(z.string()).optional(),
});

const waitStepSchema = z.object({
  kind: z.literal('wait'),
  durationMs: z.number(),
});

const pauseStepSchema = z.object({
  kind: z.literal('pause'),
  label: z.string().optional(),
  prompt: z.string().optional(),
});

export const scenarioStepSchema = z.discriminatedUnion('kind', [
  requestStepSchema,
  consumeStepSchema,
  waitStepSchema,
  pauseStepSchema,
]);

export const scenarioSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string().optional(),
  unitType: unitTypeSchema,
  sessionMode: sessionModeSchema,
  serviceModel: serviceModelSchema,
  origin: scenarioOriginSchema,
  favourite: z.boolean().optional(),
  subscriberId: z.string().optional(),
  peerId: z.string().optional(),
  stepCount: z.number(),
  updatedAt: z.string(),
  avpTree: z.array(avpNodeSchema),
  services: z.array(serviceSchema),
  variables: z.array(variableSchema),
  steps: z.array(scenarioStepSchema),
});

/** Looser schema accepted on import — server fields stripped on the way in. */
export const scenarioInputSchema = scenarioSchema.partial({
  id: true,
  origin: true,
  stepCount: true,
  updatedAt: true,
  favourite: true,
});

export type ScenarioImport = z.infer<typeof scenarioInputSchema>;
