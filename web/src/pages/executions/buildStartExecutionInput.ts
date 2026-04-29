/**
 * Pure helper: assemble a `StartExecutionInput` payload from the
 * Start-Run dialog's form state. Lives outside the component so tests
 * can assert the contract-correctness of the produced object without
 * pulling in React or Mantine.
 */
import type {
  ExecutionMode,
  StartExecutionInput,
} from '../../api/resources/executions';

export interface StartRunFormState {
  scenarioId: string;
  mode: ExecutionMode;
  /** Override peer; null = use scenario default. */
  peerId: string | null;
  /** Override subscriber; null = use scenario default. */
  subscriberId: string | null;
  /** Form value; only used in continuous mode. */
  concurrency: number;
  /** Form value; only used in continuous mode. */
  repeats: number;
}

/**
 * Construct a contract-faithful `StartExecutionInput` for
 * `POST /executions`.
 *
 *   - Interactive forces concurrency = 1, repeats = 1 regardless of the
 *     (disabled) form values — server validation rejects the
 *     combination otherwise.
 *   - Continuous clamps concurrency to [1..10] and repeats to [1..1000]
 *     to match the on-screen NumberInputs.
 *   - `overrides` is omitted when both peer and subscriber are null so
 *     the request body stays small.
 */
export function buildStartExecutionInput(
  form: StartRunFormState,
): StartExecutionInput {
  const isInteractive = form.mode === 'interactive';
  const input: StartExecutionInput = {
    scenarioId: form.scenarioId,
    mode: form.mode,
    concurrency: isInteractive
      ? 1
      : Math.max(1, Math.min(10, form.concurrency)),
    repeats: isInteractive ? 1 : Math.max(1, Math.min(1000, form.repeats)),
  };
  if (form.peerId || form.subscriberId) {
    input.overrides = {
      ...(form.peerId ? { peerId: form.peerId } : {}),
      ...(form.subscriberId ? { subscriberIds: [form.subscriberId] } : {}),
    };
  }
  return input;
}
