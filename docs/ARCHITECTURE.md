# ARCHITECTURE.md — OCS Testbench

## Purpose

This document captures the architectural design of the OCS Testbench. It is
the single source of truth that drives the Figma designs, the OpenAPI spec,
and the implementation.

**Status:** Design locked. Implementation proceeds in layers (see §20). The
first public release ships with the full designed surface in §3–§10.

---

## 1. System Role

The OCS Testbench acts as a **Charging Trigger Function (CTF)**, sending
Diameter Credit-Control-Request (CCR) messages to an OCS via the **Gy
interface** (RFC 4006) and processing Credit-Control-Answer (CCA) responses.

It is a **traffic generator and verifier** — it does not implement OCS logic.
The authoring surface is optimised for **interactive exploration** of OCS
behaviour (pause mid-session, mutate state, resume) as well as repeatable
batch runs.

---

## 2. Charging Models

Two charging models are supported, expressed on a scenario as **session
mode**:

| `sessionMode` | Request types allowed | Lifecycle |
|---|---|---|
| `session` | INITIAL, UPDATE, TERMINATE | Long-lived session; Session-Id stable, CC-Request-Number increments |
| `event` | EVENT | One-shot; Session-Id per-request, no lifecycle |

The engine validates request types against the scenario's `sessionMode` at
save time and at send time.

---

## 3. Domain Model Overview

Four persistent resources, one enum, one variable system.

```
           ┌────────────────┐
           │  Subscriber    │  msisdn (PK), iccid, imei?, tac?
           └────────┬───────┘
                    │ referenced by
           ┌────────▼───────┐
  Peer ───▶│  Scenario      │  origin ∈ {system, user}
           │                │  unitType, sessionMode
           │   envelope     │  avpTree (CCR minus MSCC)
           │   mscc catalog │  mscc[ratingGroup] → MsccDefinition
           │   variables    │  Variable[] (user-scope)
           │   steps        │  ScenarioStep[]
           └────────┬───────┘
                    │ instantiated as
           ┌────────▼───────┐
           │  Execution     │  state machine, context, step records
           └────────────────┘
```

- **Subscribers** and **Peers** are catalogues of identities and transports.
- **Scenarios** are the primary authoring surface. System starters (immutable,
  one per common service flavour) and user scenarios coexist; users create by
  duplicating.
- **Executions** are runtime records of a scenario running against a specific
  subscriber+peer binding. They carry live mutable context during runs.

A scenario is the sole authoring unit. It is fully self-contained — it owns
its CCR envelope, its MSCC catalogue, its variables, and its step list.
Starter scenarios are curated starting points the user duplicates and edits;
no other authoring resource exists.

### Unit type

A scenario declares `unitType ∈ {OCTET, TIME, UNITS}`. This drives which CC-*
inner AVP the engine places into `Requested-Service-Units`, `Used-Service-Units`,
and (on extraction) `Granted-Service-Units`:

| `unitType` | Inner AVP | Diameter type |
|---|---|---|
| OCTET | CC-Total-Octets | Unsigned64 |
| TIME | CC-Time | Unsigned32 |
| UNITS | CC-Service-Specific-Units | Unsigned32 |

All MSCCs in a scenario share the same unit type.

---

## 4. Scenarios

### Shape

```
Scenario {
  id
  name, description?
  unitType:     "OCTET" | "TIME" | "UNITS"
  sessionMode:  "session" | "event"
  serviceModel: "root" | "single-mscc" | "multi-mscc"
  origin:       "system" | "user"
  favourite?:   boolean

  subscriberId, peerId

  avpTree            // CCR envelope without service-unit AVPs
  services: Service[] // one entry in root/single-mscc; 1..N in multi-mscc

  variables: Variable[]   // user-scope; system variables implicit
  steps:     ScenarioStep[]
}
```

### The AVP tree

The scenario's `avpTree` is the **complete CCR envelope minus the service-
unit AVPs**: Session-Id slot, Origin-Host/Realm, Destination-Host/Realm,
Subscription-Id block(s), Service-Context-Id, CC-Request-Type,
CC-Request-Number, User-Equipment-Info, Service-Information (873) subtree
(PS/IMS/SMS/generic), Event-Timestamp, and any vendor-specific AVPs.

Leaf values in the tree are **always variable references** (`{{TOKEN}}`).
Literals at leaf level are represented as variables with a `literal`
generator strategy (see §5). There is no dual representation.

The tree's **structure is frozen at execution start** — AVPs cannot be added
or removed at run time (see §8, rule 1). Authoring-time editing is
unrestricted.

### Services

Service units (Requested-Service-Unit / Used-Service-Unit) are modelled
separately from the AVP tree because the engine controls their presence and
inner-AVP construction (see §7). **Where** those AVPs live in the CCR is
governed by `serviceModel`:

| `serviceModel` | Shape | Typical use |
|---|---|---|
| `root` | RSU/USU sit directly under the CCR root. No MSCC block is emitted. No Rating-Group or Service-Identifier AVPs. | Legacy / simple TIME or UNITS deployments (voice, SMS, USSD on OCSes that don't use MSCC). |
| `single-mscc` | Exactly one MSCC block, always present. May carry a Rating-Group and/or Service-Identifier. MSI is emitted as `0` (MULTIPLE_SERVICES_NOT_SUPPORTED). | Most TIME/UNITS scenarios on MSCC-aware OCSes; also OCTET sessions that only ever charge one RG. |
| `multi-mscc` | One-to-many MSCC blocks, selectable per step. MSI is emitted as `1` (MULTIPLE_SERVICES_SUPPORTED). | OCTET data sessions with per-APN / per-RG accounting. |

**Compatibility matrix:**

| `unitType` | `root` | `single-mscc` | `multi-mscc` |
|---|---|---|---|
| OCTET | — | ✓ | ✓ |
| TIME  | ✓ | ✓ | — |
| UNITS | ✓ | ✓ | — |

OCTET without MSCC is not supported. `multi-mscc` is OCTET-exclusive —
multi-service reporting only makes sense for data volume.

**Default per unitType:** OCTET → `multi-mscc`; TIME → `single-mscc`;
UNITS → `single-mscc`. Authors can switch to any compatible value.

**Service shape** (entries in `scenario.services[]`):

```
Service {
  id                           // stable key; also the catalogue key for multi-mscc
  ratingGroup?:       VarRef   // required for *-mscc; omitted for root
  serviceIdentifier?: VarRef   // optional for *-mscc; omitted for root
  requestedUnits:     VarRef   // engine decides emission per §7 presence rules
  usedUnits?:         VarRef   // engine decides emission per §7 presence rules
}
```

- In **root** mode, `scenario.services` contains exactly one entry with no
  `ratingGroup` / `serviceIdentifier`. The engine splices RSU/USU directly
  under the CCR root.
- In **single-mscc** mode, `scenario.services` contains exactly one entry.
  The engine wraps it in a single MSCC block at send time. MSI=0 is added
  to the root.
- In **multi-mscc** mode, `scenario.services` is the full catalogue (1..N).
  Steps reference entries by `id` (typically the Rating-Group string) via
  `MsccSelection`. The engine emits one MSCC per selection and adds MSI=1
  to the root.

Each `VarRef` is a variable name resolved per-send from the current context.

Authoring UI note: the Services tab surfaces the `serviceModel` segmented
control at the top, then shows either a single-entry editor (root /
single-mscc) or a list + editor (multi-mscc).

### Step kinds

```
ScenarioStep =
  | { kind: "request",
      requestType: "INITIAL" | "UPDATE" | "TERMINATE" | "EVENT",
      services?:   ServiceSelection,    // required in multi-mscc; omitted otherwise
      overrides:   { [varName]: string },
      assertions?: string[],
      guards?:     string[],
      resultHandlers?: ResultHandler[] }

  | { kind: "consume",
      services?:  ServiceSelection,     // required in multi-mscc; omitted otherwise
      windowMs:   number,
      maxRounds?: number,
      terminateWhen?: string,
      overrides:  { [varName]: string },
      assertions?: string[] }

  | { kind: "wait",  durationMs: number }
  | { kind: "pause", label?: string, prompt?: string }

ServiceSelection =
  | { mode: "fixed",  serviceIds: string[] }
  | { mode: "random", from: string[], count: number | { min: number, max: number } }
```

- **request** — sends a single CCR of the given type. In `multi-mscc`,
  `services` decides which MSCC blocks go in; in `root` / `single-mscc` the
  selection is implicit (the single service) and `services` is omitted.
  `overrides` is a per-step transient map of variable-name → value that
  shadows the scenario defaults for this step only.
- **consume** — legal in `sessionMode: session` only. Expands at run time
  into an auto-generated sequence of CCR-UPDATE messages, re-rolling random
  selections each round, until `terminateWhen` fires or every selected
  service becomes `exhausted` / `error` (see §6). In non-multi-mscc modes
  the selection is the single service.
- **wait** — passes time. Useful for testing Validity-Time expiry.
- **pause** — authored breakpoint. Suspends execution until resumed. See §9.

### System starter scenarios

Five system-owned scenarios ship with the product as immutable starting
points. They cannot be edited or deleted; they can only be duplicated.

| Name | unitType | sessionMode | Service-Information subtree |
|---|---|---|---|
| Starter · Data Session | OCTET | session | PS-Information (874) |
| Starter · Voice Session | TIME | session | IMS-Information (876) |
| Starter · USSD Session | TIME | session | ServiceInformation (generic) |
| Starter · SMS Event | UNITS | event | SMS-Information (2000) |
| Starter · USSD Event | UNITS | event | ServiceInformation (generic) |

Users create scenarios by duplicating a starter (or any other scenario). The
duplicate is fully independent — no lineage tracking, no drift warnings. If
a new starter ships in a later release, existing user scenarios are
unaffected.

Users may mark their own scenarios as `favourite` to surface them at the top
of the "New scenario from…" picker alongside the system starters.

---

## 5. Variables

Variables are the **only user-mutable surface**. Every value in a CCR — leaf
AVP content, MSCC fields, even timestamps — resolves through the variable
system. Literals exist as a degenerate case (`generator: literal`). Changing
a value means changing a variable; there is no alternative mutation path.

### Scope

| Scope | Owned by | Declared on scenario? | Editable at run time? |
|---|---|---|---|
| `system` | Engine | No — implicit, auto-provisioned | Depends on source; see below |
| `user` | Scenario author | Yes — `scenario.variables` | Depending on source |

### Source

Every variable has exactly one source:

```
VariableSource =
  | { kind: "generator",  strategy: GeneratorStrategy, refresh: "once" | "per-send" }
  | { kind: "bound",      from: "subscriber" | "peer" | "config" | "step", field: string }
  | { kind: "extracted",  path: string, transform?: string }
```

- **generator** — engine computes the value from a strategy. `refresh` controls
  recomputation frequency: `once` fixes the value at execution start,
  `per-send` recomputes for every CCR.
- **bound** — value read from another resource at resolve time. Subscriber
  and peer bindings resolve at execution start; step bindings resolve per-send;
  config bindings from the running testbench process.
- **extracted** — value read from a CCA response after each send. Engine
  applies the `path` to the response AVP tree and writes the result. Latest
  extraction wins; if a later CCA lacks the path, the previous value persists.

### Generator strategies

User-available:

| Strategy | Params | Typical `refresh` |
|---|---|---|
| `literal` | `value` | once |
| `uuid` | — | once |
| `incrementer` | `start?`, `step?` | per-send |
| `random-int` | `min?`, `max?` | per-send or once |
| `random-string` | `length`, `charset ∈ {alpha,numeric,alphanumeric,hex}` | per-send or once |
| `random-choice` | `options: string[]` | per-send or once |

System-reserved (user variables cannot pick these):

| Strategy | Semantics |
|---|---|
| `session-id` | Engine-generated RFC 6733 `<OriginHost>;<hi32>;<lo32>` with UUID-derived halves |
| `charging-id` | 32-bit unsigned int, fresh per execution |
| `correlation-id` | UUID or counter; present only if the scenario's AVP tree references it |
| `sub-session-id` | For multi-sub-session flows; present only when referenced |
| `request-number` | Per-session incrementer starting at 0 |
| `event-timestamp` | Current time at send |

### System variable catalogue

These are always present at run time regardless of whether the scenario's
`avpTree` references them. Authors can reference any of them by name via
`{{TOKEN}}` substitution.

Session-wide (one value per execution):

| Variable | Source |
|---|---|
| `SESSION_ID` | generator: session-id, refresh: once |
| `CHARGING_ID` | generator: charging-id, refresh: once |
| `ORIGIN_HOST`, `ORIGIN_REALM` | bound: config.* |
| `DESTINATION_HOST`, `DESTINATION_REALM` | bound: peer.* |
| `SERVICE_CONTEXT_ID` | bound: config.service-context (derived from subscriber/peer/scenario config) |
| `MSISDN`, `IMSI`, `IMEI`, `ICCID` | bound: subscriber.* |

Per-send:

| Variable | Source |
|---|---|
| `CC_REQUEST_TYPE` | bound: step.requestType (mapped to Diameter enum 1..4) |
| `CC_REQUEST_NUMBER` | generator: request-number, refresh: per-send (session mode only) |
| `EVENT_TIMESTAMP` | generator: event-timestamp, refresh: per-send |

Extracted from CCA (per-session state):

| Variable | Source |
|---|---|
| `RESULT_CODE` | extracted: top-level Result-Code |
| `SESSION_STATE` | derived: `active` \| `terminated` \| `error` |

Extracted per service (auto-provisioned for every entry in
`scenario.services`). Naming depends on `serviceModel`:

| `serviceModel` | Naming |
|---|---|
| `root` | Flat — no wrapping MSCC, values come from root RSU/USU |
| `single-mscc` | Flat — one MSCC, no disambiguation needed |
| `multi-mscc` | Prefixed `RG<rg>_` — one name per Rating-Group |

| Variable (flat / prefixed) | Meaning |
|---|---|
| `GRANTED` / `RG<rg>_GRANTED` | Granted-Service-Units value (unit-type-aware) |
| `VALIDITY` / `RG<rg>_VALIDITY` | Validity-Time AVP |
| `FUI_ACTION` / `RG<rg>_FUI_ACTION` | Final-Unit-Action code (0=TERMINATE, 1=REDIRECT, 2=RESTRICT) or null |
| `SERVICE_RESULT_CODE` / `RG<rg>_RESULT_CODE` | Per-MSCC Result-Code if present; in `root` mode this mirrors top-level `RESULT_CODE` |
| `SERVICE_STATE` / `RG<rg>_STATE` | Derived: `active` \| `exhausted` \| `error` |

### Override compatibility matrix

During pause, the debugger allows users to override variable values. Two
paths (see §9):

| Source | Context override (permanent) | Payload override (one-shot) |
|---|---|---|
| generator: literal | ✓ | ✓ |
| generator: uuid / random-* / incrementer | ✓ (freezes generator to current value) | ✓ |
| generator: session-id / charging-id / request-number / event-timestamp / correlation-id / sub-session-id | ✗ | ✗ |
| bound: subscriber / peer | ✓ (warn) | ✓ |
| bound: config / step | ✗ (change the source instead) | ✓ |
| extracted | no-op (re-extracted next round) | ✓ |

Engine-reserved system generators stay immutable because they carry
protocol-critical semantics; overriding them would corrupt Diameter state.

### Resolution order

At send time, the engine resolves values in this order:

1. Payload override (if present for this send) → wins
2. Context value at send time → otherwise
3. Declared source (generator / bound / extracted) → otherwise

Context values themselves update from: initial resolution at execution start,
extraction after each CCA, and context overrides applied during pause.

### Authoring affordance — auto-promotion

The editor never asks the author to open a separate Variables panel before
typing a value. Any numeric or text input in the AVP tree editor or MSCC
catalogue editor accepts a raw value and transparently creates a variable
backing it (named from context, e.g. `MSCC_100_REQUESTED`, with source
`generator: literal, refresh: once`). Auto-created variables appear in the
Variables panel under an "Auto" group, renameable at any time. This keeps
simple authoring fluent while preserving the variables-only mutation
invariant underneath.

---

## 6. Execution Engine

### Execution lifecycle

An Execution is a durable resource with a state machine:

```
pending → running → paused → running → … → (success | failure | aborted | error)
```

- **pending** — created; not yet dispatched.
- **running** — engine is actively stepping.
- **paused** — hit a pause step, a runtime-set breakpoint, or the user
  clicked Pause. Execution persists in this state across process restarts.
  Users can inspect and mutate the live context via the debugger (§9).
- **success / failure / aborted / error** — terminal. `success` means every
  step completed and no default failure condition was tripped. `failure`
  means an assertion failed or a handler returned `stop`. `aborted` means
  the user stopped it. `error` means the engine itself faulted (e.g.
  transport loss with no handler policy).

### Execution context

Created at run start, mutated throughout:

```
ExecutionContext {
  executionId
  sessionId              // from system variable generator, stable within run
  chargingId
  requestNumber          // session-mode only
  startedAt
  currentStepIndex
  state                  // the state-machine value
  variables: Map<name, value>       // live substitution map
  history:   StepRecord[]           // append-only, per completed step
  breakpoints: Set<stepIndex>       // runtime-set; independent of pause steps
}
```

### Per-step loop

For each step at `currentStepIndex`:

1. **Pause check** — if the step is a `pause` step, or `breakpoints` contains
   this index, transition to `paused` and yield. Resume picks up at step 2.
2. **Apply overrides** — merge `step.overrides` onto the live context to
   produce a per-step context view.
3. **Guards** (if any) — evaluate pre-send predicates against the per-step
   context. If any fail → consult result handlers.
4. **For `request` steps:**
   1. Resolve `MsccSelection` — if `fixed`, take the list; if `random`, roll
      `count` from `from`. Apply the filter: exclude any RG whose `RG<rg>_STATE`
      is `exhausted` or `error` (auto-exclusion, §7).
   2. Construct the CCR:
      - Resolve the AVP tree by substituting every `{{TOKEN}}` from the
        per-step context.
      - For each selected RG, construct the MSCC AVP block per §7 presence
        rules.
      - Splice MSCCs into the correct position in the CCR.
   3. Send via the `Sender` interface (§15); receive CCA with a bounded
      timeout.
   4. Update the context from the CCA:
      - Extract session-level values (Result-Code → `RESULT_CODE`,
        `SESSION_STATE`).
      - For each RG present in the response, extract `RG<rg>_GRANTED`,
        `RG<rg>_VALIDITY`, `RG<rg>_FUI_ACTION`, `RG<rg>_RESULT_CODE`, and
        derive `RG<rg>_STATE` (§7).
   5. Evaluate assertions against the updated context. If any fail → consult
      result handlers.
   6. Emit `execution.progress` SSE event with the StepRecord.
5. **For `consume` steps:** enter the consume loop (§6.4 below).
6. **For `wait` steps:** sleep `durationMs`. Emit `execution.progress` with a
   wait record.
7. **Advance** `currentStepIndex`. If past end → transition to `success`.

### Consume step loop

Consume steps expand into an inner sequence of CCR-UPDATEs. For each round:

1. Pause / breakpoint check (a `pause` UI action pauses between rounds).
2. Resolve MSCC selection, applying auto-exclusion.
3. If the resulting selection is empty → exit the loop (transitions to the
   next step).
4. Evaluate `terminateWhen` (if set) against the current context. If true →
   exit the loop.
5. Construct and send the CCR-UPDATE. For `usedUnits` values, the engine
   applies the consumption formula:

   ```
   usedUnits = ratePerSec × (windowMs / 1000) × reportPercent
   ```

   where `ratePerSec` and `reportPercent` are variables (either scenario-
   level or overridden).
6. Update context from CCA as for a request step.
7. If `maxRounds` reached → exit loop.
8. Sleep `windowMs`. Next round.

Default termination (when `terminateWhen` is absent): the loop exits when no
RG in the selection pool is in state `active`.

### SSE event stream

The engine broadcasts execution events over an SSE channel the UI subscribes
to:

| Event | Payload |
|---|---|
| `execution.created` | `{executionId, scenarioId, startedAt}` |
| `execution.progress` | `{executionId, stepIndex, stepRecord, contextSnapshot}` |
| `execution.paused` | `{executionId, atStepIndex, reason: "breakpoint" \| "pause-step" \| "user"}` |
| `execution.resumed` | `{executionId, fromStepIndex}` |
| `execution.completed` | `{executionId, result, finishedAt}` |

`contextSnapshot` is the live variables map at event emission time — used by
the UI to keep the Variables panel live during a run.

### Batched executions (Run invocation)

Kicking off a scenario via `POST /executions` accepts optional load-shape
overrides:

```
{ scenarioId,
  overrides?: {
    subscriberIds?: string[],   // pool, round-robin across iterations
    peerIds?: string[]          // future: load-balance across peers
  },
  concurrency?: number,         // parallel workers, default 1
  repeats?: number              // iterations per worker, default 1
}
```

With `concurrency: 4, repeats: 25`, the engine spawns 4 parallel workers, each
running the scenario 25 times. Each iteration is its own Execution record; a
shared `batchId` groups them in the list view.

The simple single-run case (concurrency 1, repeats 1) remains one click from
the Scenario detail; batching is surfaced under an "Advanced" disclosure.

---

## 7. Engine-managed AVPs

Some AVPs the engine owns outright — their values, presence, or structure
are constrained by Diameter protocol semantics that the authoring surface
should not expose as free fields.

### Session-Id (263)

Generated by the engine per Diameter RFC 6733. Format:

```
<Origin-Host>;<hi32>;<lo32>
```

where `hi32` and `lo32` are the high and low halves of a fresh UUID. Stable
within an execution; regenerated for each new execution. Refresh: `once`.

### CC-Request-Type (416) and CC-Request-Number (415)

`CC-Request-Type` is derived from `step.requestType` (INITIAL=1, UPDATE=2,
TERMINATE=3, EVENT=4), surfaced as the bound variable `CC_REQUEST_TYPE`.

`CC-Request-Number` is a per-session incrementer starting at 0 on the
INITIAL request, +1 per subsequent request. Surfaced as `CC_REQUEST_NUMBER`.
In `sessionMode: event`, the variable is 0 and the AVP is typically absent.

### Charging-Id, Correlation-Id, Sub-Session-Id

Engine-generated at execution start with `refresh: once`. Correlation-Id and
Sub-Session-Id are only materialised when the scenario's AVP tree references
them — they are not unconditionally added.

### Event-Timestamp (55)

Current wall-clock time at send; `refresh: per-send`.

### Multiple-Services-Indicator (455)

Controls whether the CCR uses MSCC wrapping. The engine emits (or suppresses)
this AVP based on `scenario.serviceModel`:

| `serviceModel` | MSI | Effect |
|---|---|---|
| `root` | — (AVP omitted) | RSU/USU sit at CCR root. No MSCC. |
| `single-mscc` | `0` (MULTIPLE_SERVICES_NOT_SUPPORTED) | Exactly one MSCC block. |
| `multi-mscc` | `1` (MULTIPLE_SERVICES_SUPPORTED) | One-to-many MSCC blocks. |

Authors never set this AVP directly; it is derived. In `root` mode the AVP
is not present on the wire at all.

### Service-Unit AVPs and MSCC (456)

The engine fully owns the placement and construction of the service-unit
AVPs (`Requested-Service-Units` 437, `Used-Service-Units` 446) and the
surrounding `Multiple-Services-Credit-Control` (456) block when one is used.

Scenarios never place service-unit AVPs or MSCC blocks in their AVP tree;
they declare them in `scenario.services` (§4) and the engine splices them
into the correct position in the CCR at send time per the `serviceModel`:

- **root** — RSU and USU are spliced directly under the CCR root, with the
  inner CC-* AVP selected by `unitType`. No wrapping MSCC, no MSI.
- **single-mscc** — one MSCC block is always emitted, wrapping RSU/USU plus
  any declared Rating-Group / Service-Identifier. MSI=0 is added to the root.
- **multi-mscc** — one MSCC block per selected service (resolved via
  `ServiceSelection`, §4). MSI=1 is added to the root.

**Presence rules:**

| Request-Type | Requested-Service-Units | Used-Service-Units |
|---|---|---|
| INITIAL | If resolved value > 0 | Never |
| UPDATE | If resolved value > 0 | Always (zero is valid) |
| TERMINATE | Never | Always |
| EVENT | If resolved value > 0 | If resolved value > 0 |

The inner AVP inside `Requested-Service-Units` / `Used-Service-Units` /
`Granted-Service-Units` is selected by `scenario.unitType`:

| `unitType` | Inner AVP |
|---|---|
| OCTET | CC-Total-Octets |
| TIME | CC-Time |
| UNITS | CC-Service-Specific-Units |

### Per-Rating-Group state machine

After each CCA, the engine updates per-RG state from extracted values:

```
active      — granted units present, no FUI, no error result code
exhausted   — granted=0, or 4010/4011/4012 per-MSCC result code,
              or the round following a TERMINATE-FUI (once the final-
              unit round has been reported)
error       — any other non-2xxx per-MSCC result code
```

RGs in state `exhausted` or `error` are **automatically excluded** from
subsequent MSCC selections (fixed and random alike). Authors don't write
exclusion logic; the engine just does it.

RGs that return a non-TERMINATE Final-Unit-Indication (REDIRECT,
RESTRICT_ACCESS) remain `active` in the state machine — FUI affects UE
behaviour, not scheduling. The `RG<rg>_FUI_ACTION` variable captures the
code so assertions can react.

---

## 8. Authoring Rules (Invariants)

Four invariants constrain the authoring surface. Tooling enforces them; the
engine relies on them.

### 1. The AVP tree is structurally frozen at run time

Authoring-time editing is unrestricted — add, remove, rearrange freely. But
once an execution starts, the structure is immutable. During pause, users
can edit variable **values**, not AVP **shapes**.

This keeps the pause UI a flat form (value editors) rather than a tree
editor, and means the engine never has to reason about structural mutation
mid-run.

### 2. Variables are the only mutation surface

Every value in a CCR resolves through a variable. There are no literals at
leaf level in the stored model. "Hard-coded" is represented as
`generator: literal, refresh: once`.

The UI auto-promotes typed literals into variables so authoring stays fluent,
but the underlying model is uniform.

### 3. Two override paths, both explicit

During pause, users have exactly two ways to change a value, and they are
distinct in the UI:

- **Context override** — writes to the live context. Affects this send and
  every subsequent send. Permanent within the run.
- **Payload override** — writes only for the next send. Reverts afterward.

Both paths edit variables; they differ only in scope.

### 4. Scenarios are self-contained

A scenario owns its AVP tree, its MSCC catalogue, its variables, and its
step list. Users create scenarios by duplicating (from a system starter,
another user scenario, or blank). After duplication there is no live
dependency on the source.

The benefit is a single authoring surface with no cross-resource cascades:
editing a scenario cannot silently break unrelated scenarios, and there is
no catalogue rename or version-drift machinery to maintain. The cost is
that improvements to starter scenarios don't auto-propagate to duplicates;
that is a deliberate trade.

---

## 9. Interactive Debugging

The pause/mutate/resume loop is the product's flagship capability. It turns
the testbench from a batch runner into an interactive debugger for the OCS.

### Entering pause

An execution enters `paused` state via three triggers:

- **Pause step** — the scenario contains an explicit `pause` step; the
  engine pauses when it reaches that index.
- **Runtime breakpoint** — the user added a breakpoint via the debugger on
  any step index. Breakpoints are stored on the Execution, not the Scenario
  (so they don't pollute the scenario definition).
- **Pause action** — the user clicks Pause during a run. The engine pauses
  at the next safe point (between steps, or between consume-loop rounds).

### Actions available during pause

```
POST /executions/{id}/resume       — resume normal execution
POST /executions/{id}/step         — execute exactly one step, then pause
POST /executions/{id}/abort        — transition to aborted
POST /executions/{id}/context      — context override: PATCH variables map
POST /executions/{id}/payload      — payload override: one-shot values for
                                     the next send
POST /executions/{id}/breakpoints  — add/remove runtime breakpoints
```

`context` and `payload` overrides both accept `{[varName]: value}` maps. The
matrix in §5 defines which variables accept which kind.

### UI contract

At pause, the Execution Detail view presents:

- **Step list** on the left, with the current step highlighted.
- **AVP tree panel** centre-top showing the resolved CCR about to be sent
  (current variables substituted). Read-only.
- **Per-RG status strip** above the tree — one chip per RG in the catalogue,
  coloured by state, showing granted units, validity, FUI if any.
- **Variables panel** on the right with three sections (system / user /
  extracted). Each variable shows current value and an edit affordance
  gated by the override matrix. Two apply buttons per edit: "Update
  context" (permanent) and "Override next send" (one-shot).
- **Toolbar** with Resume / Step / Abort / Add breakpoint.

For random-mode MSCC selections, the "about to send" preview pre-rolls the
selection so the user sees the concrete plan and can override it to a fixed
list via the payload path, or lock it by editing the scenario via the
context path.

---

## 10. CCA Response Evaluation

Two levels of response handling: protocol behaviour (engine, non-negotiable)
and scenario-driven evaluation (author-defined).

### Level 1 — Protocol behaviour

The Diameter stack and the engine together handle protocol-mandated response
behaviour:

- **Per-MSCC result codes 4010/4011/4012** → per-RG `exhausted` state, RG
  excluded from future selections.
- **Final-Unit-Indication = TERMINATE** → RG marked `exhausted` after this
  round's usage is reported.
- **Session Result-Code 5xxx** → session terminates, execution transitions
  to `error` unless a handler overrides.
- **Validity-Time** → engine tracks the grant lifetime; consume loops honour
  it by re-authorising before expiry.

These are not exposed to the scenario. They are compliance, not testing
choices.

### Level 2 — Scenario-driven evaluation

Each step can declare:

- **Extractions** — implicit via the auto-provisioned per-RG variables.
  Authors writing their own extraction paths is a Tranche B feature (§21).
- **Guards** — pre-send predicates; if false, the step is skipped and
  (optionally) a handler is consulted.
- **Assertions** — post-receive predicates; on failure, the step is marked
  failed and (optionally) a handler is consulted.
- **Result handlers** — declarative reaction to specific outcomes:
  ```
  { when: "<expression>", action: "continue" | "stop" | "retry" }
  ```
  In MVP, default behaviour is "stop on any assertion failure or session
  error"; explicit handlers override.

### Expression language

Expressions are used in guards, assertions, `terminateWhen`, and handler
`when` clauses. The grammar is intentionally small:

```
expr       := comparison (('AND' | 'OR') comparison)*
comparison := value op value
value      := varName | literal | paren
op         := '==' | '!=' | '<' | '<=' | '>' | '>='
paren      := '(' expr ')'
literal    := number | string | 'null'
varName    := identifier           // resolved from live context
```

Examples:

```
RESULT_CODE == 2001
RG100_GRANTED > 0
RG100_GRANTED <= RG100_REQUESTED
SESSION_STATE == 'active' AND RG200_STATE != 'error'
```

Helper functions (`percentage(...)`, etc.) are a Tranche B addition.

---

## 11. AVP Dictionary

The testbench uses **go-diameter's built-in XML dictionary** for AVP metadata:

- **Base Protocol** (RFC 6733) — connection-level AVPs
- **Credit Control** (RFC 4006) — Rating-Group, MSCC, RSU/USU, result codes
- **3GPP Ro/Rf** (TS 32.299) — PS-Information, IMS-Information, SMS-Information

For vendor-specific or proprietary AVPs, additional XML dictionary files can
be loaded at runtime from the `custom_dictionary` store.

The engine uses dictionary lookups to:

- Resolve AVP names to codes, vendor IDs, and Diameter data types
- Validate scenario AVP trees at save time (reject unknown AVP names)
- Encode variable values into the correct Diameter data type when
  constructing CCRs

---

## 12. Resources

### Subscribers

| Field | Type | Required | Description |
|---|---|---|---|
| id | UUID | Yes | Primary key |
| msisdn | string | Yes | Mobile number — maps to Subscription-Id (END_USER_E164). Functional primary identifier. |
| iccid | string | Yes | SIM identifier |
| imei | string | No | Device identifier |
| tac | string | No | Type Allocation Code (from IMEI prefix) |

Every scenario requires a subscriber binding.

### Peers

A peer is an independent Diameter connection with its own:

- Origin-Host / Origin-Realm
- Remote endpoint (host, port)
- Connection state (CER/CEA exchange)
- Session space

Scenarios are bound to a peer. A peer can run multiple concurrent scenarios.
A scenario cannot span peers.

The MVP supports N independent concurrent peers for:

- Simulating different CTF identities (different Origin-Host values)
- Testing against multiple OCS/DRA endpoints simultaneously
- Scaled load testing via peer-level concurrency

### Configuration

Diameter peer endpoint configuration (host, port, realm, peer identity,
transport parameters) is **runtime-configurable** — users can add, modify,
or remove endpoints without restarting. Changes take effect on next
connection attempt.

Process-level configuration (Origin-Host, Origin-Realm of the testbench
itself) is also runtime-editable. These values back the `ORIGIN_HOST` /
`ORIGIN_REALM` system variables.

---

## 13. Transport

- **MVP:** TCP only
- Plaintext by default; TLS is a connection-level configuration option
- SCTP is out of scope (deprecated in practice)

---

## 14. Application Architecture

### API-first design

The application is a **Go HTTP server** exposing a REST API with SSE
streaming:

- **REST** — CRUD on resources, execution control, context/payload overrides
- **SSE** — server-to-client streaming of execution progress, peer status,
  per-step metrics

SSE is chosen over WebSocket because all real-time flow is server → client.
Client-to-server actions (start, stop, resume, override) are discrete REST
calls. SSE is simpler (standard HTTP, auto-reconnect, no protocol upgrade),
works through all proxies/load balancers, and keeps the application to a
single protocol layer.

### Frontend

**React/TypeScript SPA** built as static assets and embedded in the Go
binary via `go:embed`. The frontend consumes the same API as any other
client.

Dark mode / theme switching is included in the MVP. Storybook is used for
component prototyping and visual verification during development.

#### UX/UI specification

Visual and interaction rules — colour palette, typography, button
variants, form patterns, modal behaviour, toast persistence, accessibility
requirements — live in **[`docs/UX_DESIGN.md`](UX_DESIGN.md)**. That file is
the canonical reference for screen and component design and is consumed
by humans and AI agents when authoring or reviewing frontend code.

#### CRUD UI conventions

Applies to all list-resource pages (peers, subscribers, scenarios, …):

- **List view** uses a Mantine `Table` with a kebab (`⋯`) action menu per
  row.
- **Create and edit** open a right-hand **Drawer** with a sectioned form
  body and footer actions.
- **Delete** is not a row action. It lives inside the edit drawer footer
  so the resource's identity is visible at confirmation time, and routes
  through a small confirmation modal. Visual style of the destructive
  button is specified in `UX_DESIGN.md` §4.2.
- Field-level validation errors (RFC 7807 422 with `errors` map keyed by
  JSON Pointer) are routed onto form fields via `@mantine/form`'s
  `setErrors()`; non-field errors surface as toasts.

Scenario editing is an **exception** — it uses a full-page layout rather
than a drawer, given the complexity of AVP tree + MSCC catalogue +
variables + step list. Unsaved-changes guards prevent accidental
navigation away.

### Single binary

The Go binary serves both the API and the embedded UI. No external
dependencies, no separate frontend server.

### Deployment modes

| Mode | How |
|---|---|
| Local | Run binary, auto-opens browser to `localhost:<port>` |
| Docker | `docker run -p <port>:<port> ocs-testbench` |
| Kubernetes | Standard deployment + service |
| Headless / CI | Drive REST/SSE directly, no browser |

### Core separation

```
┌───────────────────────────────────────────────────┐
│  React/TS UI  (embedded via go:embed)             │
├───────────────────────────────────────────────────┤
│  REST API + SSE Streaming                         │
├───────────────────────────────────────────────────┤
│  Core library                                     │
│  ┌──────────┐ ┌───────────┐ ┌───────────────────┐│
│  │ Diameter │ │ Variable  │ │ Execution Engine   ││
│  │  Stack   │ │ Resolver  │ │                    ││
│  │          │ │ +         │ │ ┌─────────────────┐││
│  │ Conn Mgr │ │ CCR       │ │ │ ExecutionContext│││
│  │ Dict     │ │ Builder   │ │ │ Step Executor   │││
│  └──────────┘ └───────────┘ │ │ Orchestrator    │││
│                             │ └─────────────────┘││
│                             └────────────────────┘│
├───────────────────────────────────────────────────┤
│  Store (PostgreSQL + sqlc)                        │
└───────────────────────────────────────────────────┘
```

The core library has **no dependency on the HTTP layer**. This enables
headless use, CLI tooling, and alternative frontends.

### Sender interface

The `Sender` interface is the key abstraction between the execution engine
and the Diameter stack — a simple contract: takes a CCR, returns a CCA.

A `MeasuredSender` decorator wraps any `Sender` to capture per-request
metrics (round-trip time, send/receive timestamps, message sizes, result
code, transport errors). Test implementations return canned CCA responses
for deterministic testing without a live Diameter peer.

---

## 15. Persistence

**PostgreSQL** accessed via **sqlc** (type-safe Go generated from SQL
queries) with **pgx/v5** as the driver. Schema managed by **golang-migrate**.

Persisted entities:

- Peer configurations (JSONB body)
- Scenarios (JSONB body — avpTree, mscc, variables, steps)
- Subscribers (normalised columns)
- Custom AVP dictionaries (XML content)
- Executions (JSONB body — context, history, state)

JSONB bodies store complex/variable structures; normalised columns store
queryable fields. PostgreSQL-specific query features (JSONB containment,
array types) are avoided to keep future SQLite migration feasible.

Executions are durable across process restarts — a paused execution survives
process loss and is resumable. Live Diameter session state (open CER/CEA,
in-flight grants) may be lost on restart depending on the stack's behaviour;
resuming a paused execution after such a loss may require the user to
reinitialise the session with a fresh CCR-I.

---

## 16. Diameter Library

The application uses **`fiorix/go-diameter`** for the Diameter base protocol
(connection management, CER/CEA, DWR/DWA, message encoding/decoding). The
Gy credit-control application layer (CCR/CCA, AVP construction) is built on
top of it.

---

## 17. Monorepo Layout

```
ocs-testbench/
├── cmd/
│   └── ocs-testbench/          # Application entry point + config
├── internal/                    # Go backend
│   ├── appl/                   # App lifecycle (metrics, signals)
│   ├── baseconfig/             # Config loading, base structs
│   ├── logging/                # Structured logging (slog)
│   ├── store/                  # Database layer (sqlc)
│   ├── diameter/               # Diameter stack
│   ├── variables/              # Variable resolver + CCR builder
│   ├── engine/                 # Execution engine
│   └── api/                    # REST API + SSE handlers
├── db/
│   └── migrations/             # golang-migrate files
├── web/                        # React/TypeScript frontend
│   ├── src/
│   ├── .storybook/
│   └── dist/                   # Build output (go:embed target, gitignored)
├── docs/                       # Project documentation
├── api/
│   └── openapi.yaml            # API spec (source of truth)
├── go.mod
├── sqlc.yaml
└── .gitignore
```

---

## 18. Bootstrap and Infrastructure

Application bootstrap adapts proven patterns from the charging-domain
project:

- **`internal/baseconfig`** — YAML config loading, base config struct
- **`internal/logging`** — slog-based structured logging, Bootstrap/Configure
  lifecycle
- **`internal/appl`** — Prometheus metrics server, signal handling, graceful
  shutdown

The `cmd/ocs-testbench/main.go` entry point wires all components: config →
logging → store → Diameter stack → API router → embedded frontend → HTTP
server → signal handler → graceful shutdown.

---

## 19. Expression Evaluator

Expression evaluation (guards, assertions, `terminateWhen`, handler `when`)
uses **`github.com/eddiecarpenter/ruleevaluator`** — a general-purpose Go
expression evaluator.

Capabilities used in MVP:

- Dot-path variable resolution (`RG100_GRANTED`, `SESSION_STATE`)
- Comparisons (`==`, `!=`, `<`, `<=`, `>`, `>=`)
- Logical connectives (`AND`, `OR`, `NOT`)
- Parenthesisation

Deferred to Tranche B:

- Custom helper functions (`percentage(...)`, `avpPresent(...)`)
- Ternary expressions
- Arithmetic in expressions

The evaluator is fed the live execution context (variables map) as its
data context. Expressions resolve variable names directly against it.

---

## 20. Build Layers

The model is designed as a single cohesive whole; implementation proceeds
in layers. Release gates on the full stack being in place. Earlier layers
deliver internally-visible value on their own but are not published until
layer 12 completes.

| Layer | What lands | Depends on |
|---|---|---|
| 1 — Shell | Subscribers, Peers, Scenarios, Executions resources + list views + system starters seeded | — |
| 2 — Scenario editor (static) | AVP tree editor, MSCC catalogue panel, Variables panel (literal-only user vars) | 1 |
| 3 — Engine core | Request step resolution, CCR send / CCA receive, MSCC presence rules, StepRecord | 2 |
| 4 — Per-RG state | Auto-provisioned `RG<rg>_GRANTED`, `RG<rg>_STATE`, session-level `RESULT_CODE` / `SESSION_STATE`, auto-exclusion | 3 |
| 5 — Assertions | Expression parser + evaluator, comparison + AND/OR, post-receive check, default-stop behaviour | 4 |
| 6 — Execution detail UI | Read-only run view: per-step request/response, per-RG status strip | 3+4+5 |
| 7 — Interactive debugger | Pause step, runtime breakpoints, resume / step / abort, context + payload overrides | 6 |
| 8 — Consume step | Auto-generated UPDATE loop with fixed MSCC selection, `terminateWhen`, consumption formula | 4+7 |
| 9 — Richer variables | User generators beyond literal (uuid, incrementer, random-int, random-string, random-choice), full per-RG variables (VALIDITY, FUI_ACTION, RESULT_CODE) | 2+4 |
| 10 — Random MSCC selection | `mode: random` with count / range, per-round re-rolling in consume loops | 8+9 |
| 11 — Guards & result handlers | Pre-send guard expressions, handler action set (continue / stop / retry), handler wiring | 5+7 |
| 12 — Batched executions | Concurrency, repeats, subscriber pool at Run invocation, batch grouping in list view | 3 |

Layer 12 is the **release gate**. Everything in the designed model works
end-to-end.

### Cross-cutting concerns

- **Diameter stack** (outbound CER/CEA, CCR/CCA) is a pre-requisite for
  layer 3 and should be started in parallel with layer 1.
- **Executable durability** (surviving process restarts) is a design
  decision that should be made before layer 3 ships, not retrofitted at
  layer 7.
- **SSE plumbing** is wired once at layer 3 and extended at layers 4, 7, 8.

---

## 21. Future Work (Tranche B)

Designed around but not committed for the first release. Schema leaves room
for each (e.g. `ScenarioStep` is a discriminated union gaining new `kind`s
without schema break).

- **Branch step** — conditional step-to-step jumping based on an expression.
- **Inject action during pause** — one-shot ad-hoc CCR crafted from the
  current context, spliced into a paused run.
- **User-defined extractions** — author-declared variables with
  `source: extracted` and custom paths (the hook exists; no UX yet).
- **Expression helpers** — `percentage(GRANTED, 80)`, `avpPresent(...)`,
  arithmetic.
- **Multi-peer scenarios** — a scenario exercising multiple peers for
  failover testing.
- **Load-test profile** — ramp curves, target TPS, adaptive pacing.
- **MCP server frontend** — expose the testbench API as MCP tools for
  AI-driven scenario composition.
- **SCTP transport** — if required by specific OCS deployments.
- **Custom session-id formats** — beyond the RFC 6733 UUID-based default.

---

## Document Conventions

- `VarRef` refers to a variable-name string. In practice, a variable is
  referenced as `{{NAME}}` in the AVP tree and as a bare `NAME` in
  expressions and step configurations.
- **RG** abbreviates Rating-Group throughout. A per-RG variable is written
  as `RG<rg>_*` where `<rg>` is the catalogue key (e.g. `RG100_GRANTED`).
- Diameter AVP codes are given in parentheses the first time an AVP is
  introduced, e.g. Session-Id (263).

— end —
