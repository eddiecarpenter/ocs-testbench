/**
 * Scenario Builder — full-screen modal overlay over the Scenarios list.
 *
 * Mounted by `ScenariosPage` whenever the route is `/scenarios/new` or
 * `/scenarios/:id`. The list page underneath stays mounted so closing
 * the modal returns the user to it without a page navigation, and
 * deep-linking / refresh on the editor URL still work.
 *
 * Owns the lifecycle of the editing draft:
 *   - Hydrate from the API (or seed a fresh draft for /new, optionally
 *     pre-filled from a source via `?dup=<id>`)
 *   - Save / Discard / Delete orchestration
 *   - Confirm modals matching the peers/subscribers Modal pattern
 *   - Modal close (X / Esc) honours the dirty guard
 *   - `DirtyGuard` covers browser refresh / close-tab via `beforeunload`
 *
 * Save & Run and Duplicate are intentionally NOT here — Run lives on
 * the list (and later on Executions); Duplicate is a list-only
 * operation that opens the editor pre-filled from the source.
 */
import {
  Alert,
  Button,
  Card,
  Group,
  Modal,
  Skeleton,
  Stack,
  Text,
} from '@mantine/core';
import { useHotkeys, useMediaQuery } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { notifyError } from '../../utils/notify';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router';

import { ApiError } from '../../api/errors';
import {
  useCreateScenario,
  useDeleteScenario,
  useScenario,
  useUpdateScenario,
} from '../../api/resources/scenarios';
import { useScenarioDraftStore } from './scenarioDraftStore';
import type { Scenario } from './types';
import { BuilderFooter } from './BuilderFooter';
import { BuilderHeader } from './BuilderHeader';
import { BuilderTabs } from './BuilderTabs';
import { DirtyGuard } from './DirtyGuard';
import { validateSteps } from './tabs/stepsValidation';
import { makeNewScenarioDraft, toScenarioInput } from './defaults';

/** Strip server-owned fields from a Scenario to use as a duplicate seed. */
function makeDuplicateDraft(source: Scenario): Scenario {
  return {
    ...structuredClone(source),
    id: '',
    name: `${source.name} (copy)`,
    origin: 'user',
    favourite: false,
    updatedAt: new Date().toISOString(),
  };
}

export function ScenarioBuilderPage() {
  const { id: routeId } = useParams<{ id?: string }>();
  const isNew = !routeId;
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const dupFromId = isNew ? searchParams.get('dup') : null;
  // Where to land when the modal closes. Callers (e.g. the Executions
  // page's "Edit scenario" button) can pass `state: { returnTo: '/…' }`
  // to bring the user back to where they came from instead of the
  // default Scenarios list.
  const returnTo =
    (location.state as { returnTo?: string } | null)?.returnTo ?? '/scenarios';

  // Responsive sizing: full-screen on small viewports, 95% on
  // medium, 80% on wide. Avoids the cramped feel that prompted
  // this work.
  const isMobile = useMediaQuery('(max-width: 768px)');
  const isMediumOrSmaller = useMediaQuery('(max-width: 1280px)');
  const modalSize = isMobile ? '100%' : isMediumOrSmaller ? '95%' : '80%';
  const isFullViewport = isMobile;

  const draft = useScenarioDraftStore((s) => s.draft);
  const dirty = useScenarioDraftStore((s) => s.dirty);
  const load = useScenarioDraftStore((s) => s.load);
  const markSaved = useScenarioDraftStore((s) => s.markSaved);
  const undo = useScenarioDraftStore((s) => s.undo);
  const redo = useScenarioDraftStore((s) => s.redo);

  // Shell-scope hotkeys for undo / redo. Empty tag exclusions keep the
  // listener active inside form controls too.
  useHotkeys(
    [
      ['mod+z', () => undo()],
      ['mod+shift+z', () => redo()],
      ['mod+y', () => redo()], // Windows convention
    ],
    [],
  );

  // ---------------------------------------------------------------------------
  // Load: hydrate the draft from the API (or seed a fresh one for /new)
  // ---------------------------------------------------------------------------
  const detail = useScenario(routeId);
  const dupSource = useScenario(dupFromId ?? undefined);

  const hydratedFor = useRef<string>('');

  useEffect(() => {
    if (isNew) {
      const wantKey = dupFromId ? `__dup__${dupFromId}` : '__new__';
      if (hydratedFor.current === wantKey) return;
      if (dupFromId) {
        // Wait for the source to load before hydrating from it.
        if (!dupSource.data) return;
        load(makeDuplicateDraft(dupSource.data));
      } else {
        load(makeNewScenarioDraft());
      }
      hydratedFor.current = wantKey;
      return;
    }
    if (detail.data && hydratedFor.current !== detail.data.id) {
      load(detail.data);
      hydratedFor.current = detail.data.id;
    }
  }, [isNew, dupFromId, dupSource.data, detail.data, load]);

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------
  const create = useCreateScenario();
  const update = useUpdateScenario(routeId ?? '');
  const remove = useDeleteScenario();

  const isSaving = create.isPending || update.isPending;
  const isDeleting = remove.isPending;

  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});
  const [discardOpen, setDiscardOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  // Toggled when a programmatic close is in flight (Discard / Delete)
  // so the Modal's onClose doesn't re-fire the dirty-guard prompt.
  const closingProgrammatically = useRef(false);

  // ---------------------------------------------------------------------------
  // Save
  // ---------------------------------------------------------------------------
  async function handleSave() {
    if (!draft) return;
    const input = toScenarioInput(draft);
    setFieldErrors({});
    // UX-layer step validation runs first — surface a single rolled-up
    // toast for things like an unbounded UPDATE repeat policy before
    // we hit the schema's `if/then/else` / `anyOf` constraints at the
    // mock handler. The schema is still the contractual backstop.
    const stepProblems = validateSteps(draft.steps, draft.sessionMode);
    if (stepProblems.length > 0) {
      notifyError({
        title: 'Step validation failed',
        message: stepProblems.join('\n'),
      });
      return;
    }
    try {
      const saved = isNew
        ? await create.mutateAsync(input)
        : await update.mutateAsync(input);
      markSaved(saved);
      notifications.show({
        color: 'green',
        title: 'Scenario saved',
        message: saved.name,
      });
      // Save closes the editor and returns to the list — matches the
      // peers / subscribers Drawer convention. The dirty flag has
      // already been cleared by `markSaved`, so the close handler
      // won't fire the discard prompt; we still set the
      // programmatic-close flag for clarity.
      closingProgrammatically.current = true;
      navigate(returnTo);
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr instanceof ApiError && apiErr.errors) {
        setFieldErrors(apiErr.errors);
        notifyError({
          title: 'Save failed',
          message: 'Validation errors — see fields below.',
        });
      } else {
        notifyError({
          title: 'Save failed',
          message: (err as Error).message,
        });
      }
    }
  }

  // ---------------------------------------------------------------------------
  // Discard / Delete / Close
  // ---------------------------------------------------------------------------
  function handleDiscardRequest() {
    if (!dirty) {
      // Nothing to discard — close the editor.
      closingProgrammatically.current = true;
      navigate(returnTo);
      return;
    }
    setDiscardOpen(true);
  }

  function handleDiscardConfirm() {
    setDiscardOpen(false);
    closingProgrammatically.current = true;
    navigate(returnTo);
  }

  function handleDeleteRequest() {
    if (isNew) return;
    setDeleteOpen(true);
  }

  async function handleDeleteConfirm() {
    if (!routeId) return;
    try {
      await remove.mutateAsync(routeId);
      notifications.show({
        color: 'teal',
        title: 'Scenario deleted',
        message: draft?.name ?? routeId,
      });
      setDeleteOpen(false);
      closingProgrammatically.current = true;
      navigate(returnTo);
    } catch (err) {
      notifyError({
        title: 'Could not delete scenario',
        message: err instanceof Error ? err.message : 'Unexpected error',
      });
    }
  }

  /** Modal X / Esc / outside — honours the dirty guard. */
  function handleCloseRequest() {
    if (closingProgrammatically.current) {
      closingProgrammatically.current = false;
      return;
    }
    if (!dirty) {
      navigate(returnTo);
      return;
    }
    setDiscardOpen(true);
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------
  const modalTitle = useMemo(() => {
    if (isNew) return 'New scenario';
    return draft?.name || detail.data?.name || 'Loading scenario…';
  }, [isNew, draft?.name, detail.data?.name]);

  const isLoadingDetail = !isNew && detail.isLoading && !draft;
  const isLoadingDup = isNew && Boolean(dupFromId) && dupSource.isLoading && !draft;
  const detailError = !isNew && detail.isError;
  const dupError = isNew && Boolean(dupFromId) && dupSource.isError;

  return (
    <Modal
      opened
      onClose={handleCloseRequest}
      title={modalTitle}
      size={modalSize}
      centered={!isFullViewport}
      fullScreen={isFullViewport}
      overlayProps={{ backgroundOpacity: 0.55, blur: 2 }}
      closeOnClickOutside={false}
      transitionProps={{ transition: 'fade', duration: 150 }}
      // Fixed height so the modal frame stays put as the user
      // switches between tabs. Make the modal CONTENT a flex
      // column so the BODY can claim whatever height remains
      // after the title bar (`flex: 1`); avoids the off-by-N-px
      // mismatch between `92vh` and the actually-rendered modal
      // height. The body itself doesn't scroll — only the tab
      // content area inside it does (see the wrapping <div> with
      // `flex: 1; overflow-y: auto`).
      styles={{
        content: isFullViewport
          ? undefined
          : {
              height: '92vh',
              display: 'flex',
              flexDirection: 'column',
            },
        body: isFullViewport
          ? undefined
          : {
              flex: 1,
              minHeight: 0,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
            },
      }}
    >
      <DirtyGuard dirty={dirty} />

      {(isLoadingDetail || isLoadingDup) && (
        <Stack gap="md">
          <Skeleton height={32} />
          <Skeleton height={140} />
          <Skeleton height={300} />
        </Stack>
      )}

      {detailError && (
        <Alert
          icon={<IconAlertTriangle size={16} />}
          color="red"
          title="Failed to load scenario"
        >
          {(detail.error as ApiError | Error).message}
        </Alert>
      )}

      {dupError && (
        <Alert
          icon={<IconAlertTriangle size={16} />}
          color="red"
          title="Failed to load source scenario for duplication"
        >
          {(dupSource.error as ApiError | Error).message}
        </Alert>
      )}

      {draft && (
        <Stack gap="md" style={{ flex: 1, minHeight: 0 }}>
          <Card withBorder padding="md">
            <BuilderHeader isNew={isNew} isDirty={dirty} />
          </Card>

          {Object.keys(fieldErrors).length > 0 && (
            <Alert
              icon={<IconAlertTriangle size={16} />}
              color="red"
              title="Some fields are invalid"
              data-testid="builder-validation"
            >
              <Stack gap={4}>
                {Object.entries(fieldErrors).map(([path, msgs]) => (
                  <Text key={path} size="sm">
                    <strong>{path}</strong>: {msgs.join('; ')}
                  </Text>
                ))}
              </Stack>
            </Alert>
          )}

          {/*
           * Tab content gets the only scrollable region in the
           * modal. `flex: 1` + `min-height: 0` is the magic combo
           * that lets a flex child actually scroll when its content
           * overflows (without it, flex children expand to content
           * and push the footer off the bottom).
           */}
          <div style={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>
            <BuilderTabs />
          </div>

          <BuilderFooter
            isNew={isNew}
            isDirty={dirty}
            isSaving={isSaving}
            onSave={handleSave}
            onDiscard={handleDiscardRequest}
            onDelete={handleDeleteRequest}
          />
        </Stack>
      )}

      <Modal
        opened={discardOpen}
        onClose={() => setDiscardOpen(false)}
        title="Discard unsaved changes?"
        centered
        closeOnClickOutside
        closeOnEscape
      >
        <Stack gap="md">
          <Text size="sm">
            You have unsaved changes in this scenario. Leaving will discard
            them.
          </Text>
          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setDiscardOpen(false)}>
              Keep editing
            </Button>
            <Button
              color="red"
              onClick={handleDiscardConfirm}
              data-testid="builder-discard-confirm"
            >
              Discard and close
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal
        opened={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title="Delete scenario"
        centered
        closeOnClickOutside={!isDeleting}
        closeOnEscape={!isDeleting}
      >
        <Stack gap="md">
          <Text size="sm">
            Are you sure you want to delete{' '}
            <strong>{draft?.name ?? ''}</strong>? This cannot be undone.
          </Text>
          <Group justify="flex-end">
            <Button
              variant="subtle"
              onClick={() => setDeleteOpen(false)}
              disabled={isDeleting}
            >
              Cancel
            </Button>
            <Button
              color="red"
              loading={isDeleting}
              onClick={handleDeleteConfirm}
              data-testid="builder-delete-confirm"
            >
              Delete
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Modal>
  );
}
