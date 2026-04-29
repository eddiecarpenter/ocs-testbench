# UX_DESIGN.md — OCS Testbench

## Purpose

This document is the **canonical UX/UI specification** for the OCS Testbench
frontend. It is consumed by:

- **Humans** designing or reviewing screens, components, and flows.
- **AI agents** generating, modifying, or reviewing frontend code, where it
  serves as the authoritative reference for visual and interaction patterns.

Rules in this document use **MUST / SHOULD / MAY** semantics (RFC 2119):

- **MUST** — non-negotiable; deviation is a bug unless listed in §12 Exceptions.
- **SHOULD** — strong default; deviate only with a recorded reason.
- **MAY** — permitted variation; choose per context.

### Source-of-truth boundary

| Concern | Lives in |
|---|---|
| Visual / interaction rules | **This document** |
| Design tokens (color hex, spacing values) | `web/src/theme/theme.ts` |
| Pixel-perfect screen layouts | Figma — `docs/design/SCREENS.md` |
| Routing, data flow, system architecture | `docs/ARCHITECTURE.md` |
| API contract | `api/openapi.yaml` |

When this document references a token, it uses the **CSS custom property**
(`var(--mantine-color-brand-5)`) or **Mantine palette key** (`color="teal"`),
not the underlying hex. The theme file is the only place hex values live.

### How to use the rule IDs

Each rule has a stable identifier of the form `R-<AREA>-<TOPIC>-<NN>`.
Cite the ID in PR reviews, commit messages, and issue comments — e.g.
*"violates `R-BTN-DESTRUCTIVE-02`"*.

Areas: `FOUND` (foundations), `LAYOUT`, `FORM`, `BTN`, `TBL`, `MOD`,
`CARD`, `FEED`, `DATA`, `DARK`, `A11Y`.

---

## 1. Foundations

### 1.1 Color

#### `R-FOUND-COLOR-01` — Brand palette is the only chrome accent

The application's primary accent **MUST** resolve through the `brand`
palette (`var(--mantine-color-brand-*)`), not Mantine's default `blue`.
Selection backgrounds, active-state borders, focus outlines, and tinted
brand surfaces all use `brand`.

> **Don't** hard-code `var(--mantine-color-blue-light)` for selection rows.
> **Do** use `var(--mantine-color-brand-light)`.

#### `R-FOUND-COLOR-02` — Semantic palette assignment is fixed

| Semantic | Mantine palette | Use for |
|---|---|---|
| Success / completed | `teal` | Successful operations, healthy state, "saved", "deleted", "completed" execution |
| Warning / transient | `yellow` | Pending state, unsaved-changes badge, paused execution, "connecting"/"restarting" |
| Error / destructive | `red` | Failed operations, destructive CTAs, error toasts, error state |
| Stopped / aborted | `orange` | Aborted execution, intentional stop |
| Info / live | `blue` | Live / running execution badge only |
| Neutral / pending | `gray` | Inactive elements, dimmed text, default state |

The palette assignment **MUST** be applied consistently across all
contexts (buttons, badges, toasts, alerts). A given semantic does **NOT**
take a different palette in a different context.

> **Don't** colour a "saved" toast `green` here and `teal` there.
> **Do** standardise on `teal` for every success outcome.

#### `R-FOUND-COLOR-03` — Color is never the sole information channel

Status, severity, and category **MUST** be communicated by **color +
icon + text** together. A user with monochrome vision or a screen-reader
must receive the same information.

This satisfies WCAG 2.2 SC 1.4.1 (Use of Color). Examples:

- Status badges carry a label string (`Connected`, `Error`).
- Status indicators (peer dot) sit adjacent to their text label.
- Alerts carry an icon (`IconAlertTriangle`, `IconCheck`) plus body text.

#### `R-FOUND-COLOR-04` — WCAG AA contrast minimum

Body text **MUST** meet WCAG 2.2 AA contrast (4.5:1 for normal text,
3:1 for large text or non-text UI). The `brand` palette and Mantine's
semantic palettes meet this when used at the documented shade. Custom
inline colours **MUST** be verified.

### 1.2 Typography

#### `R-FOUND-TYPE-01` — Heading scale

The heading scale defined in `theme.ts` is canonical:

| Tag | Mantine | Used for |
|---|---|---|
| `Title order={2}` | h2 / 22px | Primary page title |
| `Title order={3}` | h3 / 18px | Drawer/modal section title, top-level pane title in detail views |
| `Title order={4}` | h4 / 16px | Sub-card / unit-group title |
| `Title order={5}` | h5 / 14px | In-card sub-section title |

Page titles **MUST** be `order={2}`. `order={1}` is reserved and
currently unused. Skipping levels (e.g. `2 → 4`) is forbidden inside a
single visual region for accessibility (logical heading order).

#### `R-FOUND-TYPE-02` — Body and monospace

- Body text **SHOULD** use Mantine's default size (`sm` / 14px).
- Dense labels (table headers, section labels) **MAY** use `xs` / 12px.
- The `monospace` family **MUST** be applied to: identifiers
  (MSISDN, ICCID, IMEI, Origin-Host, endpoint), JSON / AVP payloads,
  expression strings, numeric duration / progress cells.

### 1.3 Spacing

#### `R-FOUND-SPACE-01` — Standard spacing tokens

All gaps, paddings, and margins **MUST** use Mantine spacing tokens
(`xs`, `sm`, `md`, `lg`, `xl`). Raw pixel values are forbidden except:

- The `gap={4}` between a page title and its dimmed subtitle (canonical
  tight pair — see `R-LAYOUT-PAGE-01`).
- Component-internal positioning where Mantine tokens cannot express
  the constraint (annotated with a comment explaining why).

#### `R-FOUND-SPACE-02` — Page-level density

A page's outermost container **MUST** be `<Stack gap="lg" p="md">` by
default. Tight content regions (sidebars, badge groups) **MAY** drop to
`gap="sm"` or `gap="xs"`.

### 1.4 Radius and borders

#### `R-FOUND-RADIUS-01` — Default radius `md`

The theme's `defaultRadius: 'md'` applies to Cards, Buttons, Inputs,
Modals. Smaller radius (`sm`) is permitted for in-list rows (sidebar
items, progress-pane rows). Custom radius values are forbidden.

#### `R-FOUND-BORDER-01` — Chrome border token

Subtle chrome borders (form footers, sidebar dividers, card section
separators) **MUST** use `var(--mantine-color-default-border)`. Inline
`light-dark()` literals duplicating the same intent are forbidden — the
theme already resolves the correct light/dark value.

### 1.5 Iconography

#### `R-FOUND-ICON-01` — Single icon library

Icons **MUST** be sourced exclusively from `@tabler/icons-react`. Mixing
icon libraries is forbidden.

#### `R-FOUND-ICON-02` — Icon sizing by context

| Context | Size | Stroke |
|---|---|---|
| Inside Button (`leftSection`) | `14` | default |
| Inside ActionIcon (table row, kebab) | `16` | default |
| Inside Add/Primary CTA Button | `16` | default |
| Inside `Alert` | `16` | default |
| Inline with body text | `14` | default |
| Nav / app-shell icons | `18` | `1.6` |
| Empty-state illustration | `48` – `56` | `1.4` |
| Status indicator (dot, badge prefix) | `14` | default |

Icons in buttons **MUST** sit in `leftSection`, never `rightSection`.

---

## 2. Layout & navigation

### 2.1 AppShell

#### `R-LAYOUT-SHELL-01` — Single canonical shell

All authenticated views **MUST** render inside the global `AppShell`
defined in `web/src/layout/AppShell.tsx`. Standalone full-window views
are forbidden except for: the global `ErrorScreen` fallback, and modal
overlays mounted on top of the shell.

### 2.2 Page header

#### `R-LAYOUT-PAGE-01` — Title + subtitle + actions

Every primary page **MUST** open with this header pattern:

```tsx
<Group justify="space-between" align="flex-start" wrap="nowrap">
  <Stack gap={4}>
    <Title order={2} fw={600}>{pageTitle}</Title>
    <Text c="dimmed" size="sm">{subtitle}</Text>
  </Stack>
  {primaryAction /* e.g. Add button */}
</Group>
```

The dimmed subtitle is **not optional**. Every page has one. If a page
genuinely has no descriptive subtitle, the page itself is mis-scoped.

> **Don't** ship a page with a bare `Title` and no subtitle.
> **Do** add a one-sentence dimmed subtitle that describes the page's purpose.

### 2.3 Loading states

#### `R-LAYOUT-LOAD-01` — Skeletons over spinners

Loading placeholders **MUST** use Mantine `Skeleton` shaped like the
content that will replace them. Indefinite spinners are forbidden for
content loading. Spinners are permitted only for in-button "pending"
states (Mantine `loading` prop).

### 2.4 Error states

#### `R-LAYOUT-ERR-01` — Three error tiers

| Error tier | Treatment |
|---|---|
| Failed query (data fetch) | Inline `<Alert color="red">` with **Retry** button — use the shared `QueryError` component |
| 404 / not-found for a specific resource | Dedicated panel (icon + title + supporting text + back-link) |
| Unhandled exception (ErrorBoundary) | Full-screen `ErrorScreen` |

Bare `<Alert>` without a retry affordance **MUST NOT** be used for query
failures — the user has nowhere to go.

### 2.5 Empty states

#### `R-LAYOUT-EMPTY-01` — Two distinct empty states

There are **two** empty states, with two distinct treatments:

- **First-run empty** (the resource has zero records ever): icon + headline
  + supporting text + primary CTA. The primary CTA opens the create flow.
- **Filtered empty** (the resource has records, but the current filter
  matches none): bare dimmed text inside the table's empty row, no CTA,
  encouraging the user to relax the filter.

> **Don't** show a generic "No items." message for a fresh install with no
> resources. **Do** show an empty-state illustration and a primary CTA.

---

## 3. Forms

### 3.1 Input layout

#### `R-FORM-LAYOUT-01` — Labels above inputs

Labels **MUST** be rendered above the input (Mantine default). Placeholder
text is **NOT** a label substitute — it disappears on input.

#### `R-FORM-LAYOUT-02` — Required vs. optional indication

When **most** fields in a form are required (≥60%), the form **MUST** mark
the **optional** fields explicitly with a `(optional)` suffix on the label,
and **MUST NOT** show the red asterisk on required fields. This reduces
visual noise (NN/g guidance).

When **most** fields are optional (<60% required), the form **MUST** mark
the required fields with the asterisk via the Mantine `required` prop.

A form **MUST NOT** mix the two conventions internally.

### 3.2 Section headings

#### `R-FORM-SECTION-01` — `SectionLabel` component

Form sections **MUST** be separated by a `<SectionLabel>` component
(uppercase, weight 600, dimmed, letter-spacing 0.5). The component is
shared, not re-implemented per form.

### 3.3 Validation

#### `R-FORM-VALID-01` — Validation timing

Client-side validation **MUST** fire on **blur** (and on submit), not on
every keystroke. Configure Mantine forms with
`validateInputOnBlur: true`. After the user's first submit attempt that
fails validation, fields **MAY** revalidate on change for that session
(so the user sees errors clear as they fix them).

> **Don't** use `validateInputOnChange: true` — error messages flashing
> while the user is mid-typing is hostile.

#### `R-FORM-VALID-02` — Server validation surfacing

422 responses (RFC 7807 with field-keyed error map) **MUST** be applied
to specific fields via `form.setErrors(ApiError.fieldErrors())`. A
top-of-form `Alert` listing field errors is **forbidden** when the form
has the offending fields visible — errors must attach to their fields.

For multi-tab forms (Scenario Builder), the offending tab(s) **MUST**
display a non-color indicator (badge dot or icon) so the user can find
the error without scrolling.

#### `R-FORM-VALID-03` — Submit is never silently disabled

The primary submit button **MUST NOT** be disabled because the form is
invalid. Allow submission, then surface field-level errors and **focus
the first invalid field**. (Disabled buttons confuse users — they don't
know why they can't submit.)

The submit button **MAY** be disabled while a submission is in flight
(use Mantine `loading` prop instead).

### 3.4 Footer button arrangement

#### `R-FORM-FOOTER-01` — Drawer / page-form footer

Drawer and page-form footers **MUST** use:

```tsx
<Group justify="space-between">
  {/* left: destructive (edit-mode only) */}
  {isEditing && <DeleteButton />}
  {/* right: secondary actions, then primary */}
  <Group justify="flex-end">
    <CancelButton />
    {/* optional secondary, e.g. Test */}
    <PrimaryButton />
  </Group>
</Group>
```

In create mode, the left slot becomes the Cancel button and the right
slot loses Cancel.

#### `R-FORM-FOOTER-02` — Modal footer

Modal footers **MUST** be `<Group justify="flex-end">` with:
*Cancel* (left of the primary), then *Primary action* (right). Order
is non-negotiable — Cancel never sits to the right of the primary.

### 3.5 Dirty-state guards

#### `R-FORM-DIRTY-01` — Universal dirty-guard

Any form (drawer, page, or modal) that holds unsaved edits **MUST**
prompt the user before discarding those edits via:

- The browser's `beforeunload` (handled by `DirtyGuard`).
- Modal X / Esc close.
- Drawer X / outside-click close.
- Router navigation away.

The discard-confirm modal uses copy *"You have unsaved changes — leaving
will discard them."* with buttons **Keep editing** (`variant="subtle"`)
and **Discard and close** (`color="red"`).

### 3.6 Debounced inputs

#### `R-FORM-DEBOUNCE-01` — Debounce only when expensive

Inputs **MUST** commit synchronously by default (Mantine `getInputProps`).
Debounce (`DebouncedTextInput` / `DebouncedTextarea`) is reserved for
fields whose every-keystroke change would be expensive (validation
re-runs, large re-renders, network calls).

---

## 4. Buttons

### 4.1 Variant intent matrix

#### `R-BTN-VARIANT-01` — Variant maps to intent

| Variant | Intent | Use for |
|---|---|---|
| (no variant) — filled brand | **Primary CTA** | The single highest-emphasis action on a screen / region: Save, Create, Add, Run, the destructive confirmation inside a confirm modal (with `color="red"`) |
| `variant="default"` | **Secondary** | Co-equal alternatives next to the primary: Test, Discover, Discard, View scenario |
| `variant="subtle"` | **Tertiary / Cancel** | Cancel buttons in modals, low-emphasis row actions, dismiss links |
| `variant="light"` | **Tinted highlight** | NavLink active state, status badges, retry-inline buttons |
| `variant="outline"` | **Destructive opener** | The Delete button on a drawer/page form footer that opens a confirm modal (see §4.2) |

A screen / region **MUST** have **exactly one** primary CTA. Two
filled-brand buttons next to each other is a bug.

### 4.2 Destructive buttons

This is the most error-prone area in the codebase. The rule is:
**emphasis matches consequence**.

#### `R-BTN-DESTRUCTIVE-01` — "Open confirm" uses outline-red

The Delete button on a drawer / page-form footer that **opens a
confirmation modal MUST** use:

```tsx
<Button color="red" variant="outline" leftSection={<IconTrash size={14}/>}>
  Delete
</Button>
```

Visually: transparent background, red border, red text. Right-aligned
when it's the sole footer action; left-aligned in the
`<Group justify="space-between">` form-footer pattern (see
`R-FORM-FOOTER-01`).

> **Don't** use `variant="subtle" color="red"` — it under-states the
> consequence and is too easily mistaken for a Cancel link.

#### `R-BTN-DESTRUCTIVE-02` — "Commit destruction" uses filled-red

The primary CTA **inside** a destructive confirmation modal **MUST** use:

```tsx
<Button color="red">Delete</Button>
```

Filled red, full emphasis. This is the action that actually destroys
data; it earns the maximum visual weight.

#### `R-BTN-DESTRUCTIVE-03` — Confirmation copy

Destructive confirmations **MUST** include the literal phrase
*"This cannot be undone."* in the body, unless the operation is genuinely
recoverable, in which case the copy describes the recovery path.

### 4.3 Icon buttons

#### `R-BTN-ICON-01` — `ActionIcon` conventions

| Context | Variant | Color |
|---|---|---|
| Table row kebab | `subtle` | `gray` |
| In-list secondary action | `subtle` | match action semantic |
| Toolbar pair (undo/redo) | `default` | (no color) |
| Read-only indicator inside input | `transparent` | (no color) |

Icon-only buttons **MUST** carry an `aria-label` describing the action
(see `R-A11Y-LABEL-01`).

### 4.4 Sizing

#### `R-BTN-SIZE-01` — Default sizes

- Standard buttons **MUST** use Mantine `sm` (default).
- Compact contexts (filter chips, retry-inline, inline pane controls)
  **MAY** use `xs`.
- Right-section input adornments (Generate / Regenerate) **MAY** use
  `compact-xs`.
- Touch targets **MUST** be ≥ 36×36 px (this is a desktop tool;
  mobile-only views would require 44×44).

---

## 5. Tables and lists

### 5.1 Table chrome

#### `R-TBL-CHROME-01` — Mantine `Table` defaults

All resource tables **MUST** use Mantine `Table` with `highlightOnHover`,
without `striped`. Sortable columns **MUST** wrap the header label in
an `UnstyledButton` carrying a sort indicator.

#### `R-TBL-CHROME-02` — Header style

Table headers **MUST** use the uppercase-dimmed-xs treatment:

```tsx
<Table.Th tt="uppercase" fz="xs" c="dimmed">Status</Table.Th>
```

This applies whether the column is sortable or not. For sortable
columns, the `UnstyledButton` wrapping the label inherits the same style.

### 5.2 Row interaction

#### `R-TBL-ROW-01` — No click-to-edit

Rows **MUST NOT** be globally clickable. Click-to-edit is forbidden
because rows often contain interactive sub-elements (badges, links,
status). All editing flows go through the row's kebab menu.

#### `R-TBL-ROW-02` — Action column

Tables **MUST** end with a single right-aligned action column:

```tsx
<Table.Th aria-label="Actions" w={48} />
```

Each row's cell contains a single `<ActionIcon variant="subtle"
color="gray"><IconDots size={16}/></ActionIcon>` triggering a Mantine
`Menu` with `position="bottom-end"`. Action items live **inside** the
menu, not alongside it. Two row-level affordances side-by-side is
forbidden.

### 5.3 Status badges

#### `R-TBL-STATUS-01` — Single badge variant per state

Badges representing the **same** state **MUST** use the same `variant`
across the entire app. The execution-state badge, for example, is
`variant="light"` everywhere — including detail views, top bars, and
sidebars.

---

## 6. Modals

### 6.1 Structure

#### `R-MOD-STRUCT-01` — Standard modal

Modals **MUST** use Mantine `Modal` with `centered` and a meaningful
`title`. Close on Esc is enabled by default; close on outside-click
follows §6.2.

Modal body uses `<Stack gap="md">`. Footer is the last child of the
body, **not** a separate Mantine slot, and **MUST** be
`<Group justify="flex-end">` with Cancel + primary CTA.

### 6.2 Confirmation modals

#### `R-MOD-CONFIRM-01` — Outside-click does not dismiss

Destructive confirmation modals **MUST** set
`closeOnClickOutside={false}`. The user must explicitly choose Cancel
or the destructive action. Outside-click is too easily triggered
accidentally for irreversible operations.

Esc-to-close **MAY** remain enabled; it requires deliberate keyboard
intent.

### 6.3 Sizing

| Modal kind | Size |
|---|---|
| Confirmation | default |
| Form modal (StartRunModal) | `md` |
| Content viewer (raw AVP) | `xl` |
| Full-surface editor (Scenario Builder) | `'80%' / '95%' / '100%'` responsive |

---

## 7. Cards and panels

### 7.1 Card defaults

#### `R-CARD-DEFAULT-01` — Theme-driven defaults

Cards inherit `withBorder` and `radius: 'md'` from the theme. Padding
follows context:

| Context | Padding |
|---|---|
| Dashboard / settings section | `lg` |
| Table-wrapper card | `md` |
| Edge-to-edge table card (table fills card) | `0` |
| Builder shell / debugger pane | `md` |

Custom shadow **MAY** be applied to dashboard-level cards (`shadow="xs"`).
Other cards remain shadow-less.

### 7.2 Card header

#### `R-CARD-HEADER-01` — In-stack title

Cards **MUST** open with an in-stack `<Title order={5}>` (or `order={4}`
for unit-group cards) plus optional dimmed micro-copy to its right.
Mantine `Card.Section` is **NOT** used for card headers in this app.

---

## 8. Feedback

### 8.1 Toasts

#### `R-FEED-TOAST-01` — Must-see vs. safe-to-miss

Toasts split into **two** persistence categories, decided by a single
question: *"If the user closes their laptop now, will they regret not
having seen this toast?"*

| Category | Configuration | Examples |
|---|---|---|
| **Must-see** | `autoClose: false`, `withCloseButton: true` (sticky until acknowledged) | Failed user actions (save, delete, validation, probe); async outcomes the user has a stake in (a long-running execution finishes/fails); background-job results |
| **Safe-to-miss** | Mantine default auto-close | Successful confirmations of just-completed actions; ambient system-state changes (peer up/down); transient informational notices |

The decision axis is **importance**, not severity. A successful
background job (positive outcome) is **must-see**. A peer flapping
between connected/disconnected (red severity) is **safe-to-miss**
because the row badge is the source of truth.

#### `R-FEED-TOAST-02` — Color follows §1.1

Toast color **MUST** apply the §`R-FOUND-COLOR-02` palette assignment.
A success toast is `teal`. An error toast is `red`. A warning is
`yellow`. No drift between resources or contexts.

#### `R-FEED-TOAST-03` — Helper API

Toasts **MUST** be raised via wrapper helpers (`utils/notify.ts`),
never via direct `notifications.show(…)` at the call site. The wrappers
encode the must-see-vs-safe-to-miss configuration so call sites only
choose semantic intent.

The wrappers **SHOULD** expose at minimum:

- `notifyMustSee({ color, title, message })` — sticky.
- `notifyTransient({ color, title, message })` — auto-close.

`notifyError` and `notifySuccess` are convenience aliases that resolve
to the appropriate wrapper.

#### `R-FEED-TOAST-04` — Position

Toasts **MUST** appear in the top-right (Mantine default). The position
is global; per-call overrides are forbidden.

### 8.2 Inline banners

#### `R-FEED-BANNER-01` — When to use `Alert`

Inline `<Alert>` is for messages **anchored to a specific page region**
that the user is currently looking at. Examples: query-failure retry
banner at the top of a page, validation-summary inside a multi-tab form,
non-success state header in the debugger.

`Alert` **MUST NOT** be used as a substitute for a toast — if the
information needs to reach the user regardless of which page they're
on, it is a toast (see §8.1).

`Alert` icon size **MUST** be `16`.

### 8.3 Full-screen error

#### `R-FEED-ERROR-01` — `ErrorScreen` usage

`ErrorScreen` is the **ErrorBoundary fallback only**. Application code
**MUST NOT** route to it for recoverable errors — those use Alerts or
toasts.

### 8.4 Status indicators

#### `R-FEED-STATUS-01` — Single status-color contract

Status colours **MUST** be expressed as Mantine palette keys
(`'teal' | 'red' | …`), not as `var(--mantine-color-*)` literals. The
keys flow through `Badge`, `Indicator`, or a small status-dot helper —
never re-derived per consumer.

This applies equally to peer status, execution state, and any future
state-driven UI.

---

## 9. Data display

### 9.1 Date and time

#### `R-DATA-TIME-01` — Relative time everywhere

User-facing timestamps **MUST** use `relativeTime(iso)` from
`web/src/utils/relativeTime.ts` (`Ns / Nm / Nh / Nd / Nw ago`).

Absolute timestamps **MAY** be exposed as a `Tooltip` over the
relative-time string. Bare `Date.toLocaleString()` is forbidden in user
surfaces.

### 9.2 Numeric formatting

#### `R-DATA-NUM-01` — Domain-appropriate formatters

- Run duration (mm:ss): `formatDuration(secs)` — `runTableHelpers.ts`.
- Elapsed (Nh Nm Ns): `formatElapsed(secs)` — `DebuggerTopBar.tsx`.
- Progress (`done / total`): rendered in monospace.

All numeric cells in tables **MUST** be `ff="monospace"` for column
alignment.

### 9.3 Code and JSON

#### `R-DATA-CODE-01` — Code rendering

- Single-token / inline values use `<Code>`.
- Multi-line / JSON / stack-trace use `<Code block>` inside a
  `<ScrollArea.Autosize mah={…}>` and `whiteSpace: 'pre'` to preserve
  formatting.

---

## 10. Dark mode

### 10.1 Token usage

#### `R-DARK-TOKEN-01` — Use theme-aware tokens

Color values **MUST** use either:

- A Mantine palette key (`color="brand"`) resolved by Mantine.
- A theme-aware CSS custom property (`var(--mantine-color-default-border)`,
  `var(--mantine-color-brand-light)`).

Inline `light-dark(<light>, <dark>)` literals are forbidden — the theme
already resolves both. If a token doesn't exist for the use case, add
one to the theme rather than inlining.

### 10.2 Brand palette

#### `R-DARK-BRAND-01` — Different shade per scheme

The theme's `primaryShade: { light: 5, dark: 4 }` rule **MUST** be
honoured. Hardcoding `--mantine-color-brand-5` ignores the dark-mode
override; use the palette key (`color="brand"`) so Mantine selects the
correct shade automatically.

---

## 11. Accessibility (a11y)

### 11.1 Keyboard navigation

#### `R-A11Y-KEY-01` — Tab order matches visual order

Tab order **MUST** match the document's visual reading order. Custom
`tabIndex` values are forbidden except `tabIndex={-1}` for elements
intentionally removed from the tab sequence (decorative, programmatic
focus targets).

#### `R-A11Y-KEY-02` — Esc closes the topmost dismissible surface

Esc **MUST** close, in order of precedence: an open `Menu`, an open
`Modal` / `Drawer`, an active `Tooltip`. Mantine handles this for
standard components; custom interactive surfaces **MUST** wire it
explicitly.

### 11.2 Focus management

#### `R-A11Y-FOCUS-01` — Visible focus rings

Focus rings **MUST** be visible. Mantine's default focus ring is
acceptable; suppressing it via `outline: none` is forbidden. Custom
focus styles **MUST** meet WCAG 2.2 SC 2.4.7 (Focus Visible) and 2.4.13
(Focus Appearance).

#### `R-A11Y-FOCUS-02` — Focus return on close

Closing a `Modal`, `Drawer`, or `Menu` **MUST** return focus to the
element that opened it. Mantine handles this for declared-trigger
patterns; custom open/close flows **MUST** track the trigger and call
`triggerRef.current?.focus()` on close.

#### `R-A11Y-FOCUS-03` — Focus first invalid field on submit failure

When a form submit fails validation, focus **MUST** move to the first
invalid field. The user should not have to scroll to find the error.

### 11.3 Labelling

#### `R-A11Y-LABEL-01` — Icon-only buttons carry an `aria-label`

Every `ActionIcon` and `Button` rendered without visible text **MUST**
carry an `aria-label` describing the action ("Delete peer",
"Open menu", not just "Delete" or "Menu"). Tooltip text alone is not
sufficient — screen-reader users may not surface tooltips.

#### `R-A11Y-LABEL-02` — Form inputs are labelled

Every input **MUST** have a Mantine `label` (which becomes a
`<label htmlFor>`) or, when a label cannot be visible, an
`aria-label`. Placeholder-as-label is forbidden.

### 11.4 Motion

#### `R-A11Y-MOTION-01` — Respect `prefers-reduced-motion`

All non-essential animations (hover translateY, transitions on
`transform` / `top` / `left`, expand/collapse) **MUST** be guarded by
`@media (prefers-reduced-motion: reduce)` in CSS modules, or by
checking `window.matchMedia('(prefers-reduced-motion: reduce)')` in
JS-driven animations.

Functional motion (loading skeletons, route transitions essential to
the interaction) **MAY** continue but **SHOULD** be slower or simpler
under reduced-motion preference.

### 11.5 Color and contrast

#### `R-A11Y-CONTRAST-01` — WCAG AA

See `R-FOUND-COLOR-04`.

#### `R-A11Y-CONTRAST-02` — Color is never the sole channel

See `R-FOUND-COLOR-03`.

---

## 12. Exceptions

This section lists **intentional, sanctioned deviations** from the
rules above. Each entry follows the schema:

- **Rule deviated from** — the rule ID.
- **Where** — the file(s) / component(s).
- **Deviation** — what is done differently.
- **Reason** — why the deviation is correct in this context.
- **Generalisation** — how to recognise other valid candidates for
  the same exception.

### 12.1 Peer-status toasts auto-close even at `error` severity

- **Rule deviated from**: `R-FEED-TOAST-01` (must-see vs. safe-to-miss).
- **Where**: `web/src/api/resources/usePeerStatusToasts.ts`.
- **Deviation**: Toasts raised when a peer transitions to `error` (or
  any settled state) use Mantine's default auto-close, **not** the
  sticky must-see configuration that the rule would suggest.
- **Reason**: Peer-status changes are **ambient system-state notifications**,
  not consequences of a user action. The peers list is the source of
  truth — the badge in the row already shows the current state. Making
  these toasts sticky would cause toast pile-up during a peer flap or
  restart cycle and would force the user to dismiss notifications about
  state they can already see in the list.
- **Generalisation**: Any toast that fires from an SSE / cache
  subscription representing **ambient state**, where the corresponding
  page already displays the current value, **MAY** auto-close
  regardless of severity. The decision rule remains the §8.1 question
  — for ambient state with a visible source-of-truth, the answer is
  always *"safe to miss"*.

---

## Appendix A — Helper APIs

| Helper | Location | Purpose |
|---|---|---|
| `notify.ts` | `web/src/utils/notify.ts` | Toast wrappers — see `R-FEED-TOAST-03` |
| `relativeTime` | `web/src/utils/relativeTime.ts` | Relative-time formatter — see `R-DATA-TIME-01` |
| `peerStatus.ts` | `web/src/components/peer/peerStatus.ts` | Peer status palette + label map |
| `runTableHelpers.ts` | `web/src/pages/executions/runTableHelpers.ts` | Execution state palette + label map + duration formatter |
| `SectionLabel` | (extract to `web/src/components/SectionLabel.tsx`) | Form section heading — see `R-FORM-SECTION-01` |
| `QueryError` | (extract to `web/src/components/QueryError.tsx`) | Query-failure inline alert — see `R-LAYOUT-ERR-01` |

## Appendix B — References

- `web/src/theme/theme.ts` — design tokens (the only place hex values
  live).
- `docs/ARCHITECTURE.md` §14 Frontend — system-level architecture.
- `docs/design/SCREENS.md` — Figma references and pixel specs per screen.
- WCAG 2.2 — https://www.w3.org/TR/WCAG22/
- Nielsen Norman Group — form-design and toast guidance.
- Mantine v7 — https://mantine.dev/
