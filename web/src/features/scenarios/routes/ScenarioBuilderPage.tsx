/**
 * Builder route — `/scenarios/:id` and `/scenarios/new`.
 *
 * Owns the lifecycle of the editing draft: load on mount, save / save &
 * run / discard / duplicate orchestration, dirty guard, breadcrumbs.
 * Tab content lives in `BuilderTabs`; the header lives in
 * `BuilderHeader`. State is kept in `useScenarioDraftStore` so every
 * tab and the header observe the same draft.
 */
import {
  Alert,
  Anchor,
  Breadcrumbs,
  Card,
  Modal,
  Skeleton,
  Stack,
  Text,
} from '@mantine/core';
import { useHotkeys } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { IconAlertTriangle } from '@tabler/icons-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router';

import { ApiError } from '../../../api/errors';
import {
  useCreateScenario,
  useDuplicateScenario,
  useRunScenario,
  useScenario,
  useUpdateScenario,
} from '../api/scenarios';
import { useScenarioDraftStore } from '../store/scenarioDraftStore';
import type { Scenario } from '../store/types';
import { BuilderHeader } from '../builder/BuilderHeader';
import { BuilderTabs } from '../builder/BuilderTabs';
import { DirtyGuard } from '../builder/DirtyGuard';
import { makeNewScenarioDraft, toScenarioInput } from '../builder/defaults';
import { exportScenarioAsFile } from '../io/exportScenario';
import {
  type ImportError,
  parseAndValidateScenarioJson,
} from '../io/importScenario';

export function ScenarioBuilderPage() {
  const { id: routeId } = useParams<{ id?: string }>();
  const isNew = !routeId;
  const navigate = useNavigate();

  const draft = useScenarioDraftStore((s) => s.draft);
  const dirty = useScenarioDraftStore((s) => s.dirty);
  const load = useScenarioDraftStore((s) => s.load);
  const reset = useScenarioDraftStore((s) => s.reset);
  const markSaved = useScenarioDraftStore((s) => s.markSaved);
  const undo = useScenarioDraftStore((s) => s.undo);
  const redo = useScenarioDraftStore((s) => s.redo);

  // Shell-scope hotkeys for undo / redo. `useHotkeys` listens at the
  // window level — the third element `["INPUT", "TEXTAREA"]` would
  // skip the listener inside text inputs, but we want undo/redo to
  // work everywhere, including within form controls. Empty list of
  // tag exclusions keeps the listener active everywhere.
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

  // Track which scenario we last hydrated so swap routes refetch.
  // Uses a ref because hydration is a side effect that must not
  // re-render the component each time it bumps.
  const hydratedFor = useRef<string>('');

  useEffect(() => {
    if (isNew) {
      if (hydratedFor.current !== '__new__') {
        load(makeNewScenarioDraft());
        hydratedFor.current = '__new__';
      }
      return;
    }
    if (detail.data && hydratedFor.current !== detail.data.id) {
      load(detail.data);
      hydratedFor.current = detail.data.id;
    }
  }, [isNew, detail.data, load]);

  // Reset the draft when the route is fully unmounted.
  useEffect(() => () => reset(), [reset]);

  // ---------------------------------------------------------------------------
  // Mutations
  // ---------------------------------------------------------------------------
  const create = useCreateScenario();
  const update = useUpdateScenario(routeId ?? '');
  const duplicate = useDuplicateScenario();
  const run = useRunScenario();

  const isSaving = create.isPending || update.isPending;

  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});

  // ---------------------------------------------------------------------------
  // Action handlers
  // ---------------------------------------------------------------------------

  /** Persist the current draft. Returns the saved Scenario on success. */
  async function persist(): Promise<Scenario | null> {
    if (!draft) return null;
    const input = toScenarioInput(draft);
    setFieldErrors({});
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
      // Move the URL onto the now-server-known id when we just created.
      if (isNew && saved.id) {
        navigate(`/scenarios/${encodeURIComponent(saved.id)}`, {
          replace: true,
        });
        hydratedFor.current = saved.id;
      }
      return saved;
    } catch (err) {
      // Surface field errors when the API returned an RFC-7807 problem
      // with a populated `errors` map; fall back to a toast otherwise.
      const apiErr = err as ApiError;
      if (apiErr instanceof ApiError && apiErr.errors) {
        setFieldErrors(apiErr.errors);
        notifications.show({
          color: 'red',
          title: 'Save failed',
          message: 'Validation errors — see fields below.',
        });
      } else {
        notifications.show({
          color: 'red',
          title: 'Save failed',
          message: (err as Error).message,
        });
      }
      return null;
    }
  }

  async function handleSave() {
    await persist();
  }

  async function handleSaveAndRun() {
    const saved = await persist();
    if (!saved) return;
    try {
      const res = await run.mutateAsync(saved.id);
      const id = res.items[0]?.id ?? '(no id)';
      notifications.show({
        color: 'green',
        title: 'Run intent fired',
        message: `Execution ${id} started for ${saved.name}`,
      });
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Run failed',
        message: (err as Error).message,
      });
    }
  }

  const [discardOpen, setDiscardOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [importError, setImportError] = useState<ImportError | null>(null);

  function handleExport() {
    if (!draft) return;
    exportScenarioAsFile(draft);
  }

  function handleImportClick() {
    setImportError(null);
    fileInputRef.current?.click();
  }

  async function handleImportFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ''; // allow re-uploading the same file
    if (!file) return;
    try {
      const text = await file.text();
      const result = parseAndValidateScenarioJson(text);
      if (!result.ok) {
        setImportError(result.error);
        notifications.show({
          color: 'red',
          title: 'Import failed',
          message:
            result.error.kind === 'parse'
              ? result.error.message
              : `${result.error.errors.length} schema error(s) — see Builder.`,
        });
        return;
      }
      // Strip server-owned fields and create a fresh scenario.
      const { id: _omitId, origin: _omitOrigin, stepCount: _omitStepCount, updatedAt: _omitUpdatedAt, ...rest } = result.value;
      void _omitId; void _omitOrigin; void _omitStepCount; void _omitUpdatedAt;
      const created = await create.mutateAsync({
        name: rest.name,
        description: rest.description,
        unitType: rest.unitType,
        sessionMode: rest.sessionMode,
        serviceModel: rest.serviceModel,
        favourite: rest.favourite ?? false,
        subscriberId: rest.subscriberId ?? '',
        peerId: rest.peerId ?? '',
        avpTree: rest.avpTree,
        services: rest.services,
        variables: rest.variables,
        steps: rest.steps,
      });
      notifications.show({
        color: 'green',
        title: 'Scenario imported',
        message: `Opened "${created.name}".`,
      });
      navigate(`/scenarios/${encodeURIComponent(created.id)}`);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Import failed',
        message: (err as Error).message,
      });
    }
  }

  function handleDiscardRequest() {
    if (!dirty) return;
    setDiscardOpen(true);
  }

  function handleDiscardConfirm() {
    setDiscardOpen(false);
    if (isNew) {
      load(makeNewScenarioDraft());
    } else if (detail.data) {
      load(detail.data);
    }
    setFieldErrors({});
  }

  async function handleDuplicate() {
    if (!routeId) return;
    try {
      const dup = await duplicate.mutateAsync({ id: routeId });
      notifications.show({
        color: 'green',
        title: 'Scenario duplicated',
        message: `Opened "${dup.name}" in the Builder.`,
      });
      navigate(`/scenarios/${encodeURIComponent(dup.id)}`);
    } catch (err) {
      notifications.show({
        color: 'red',
        title: 'Duplicate failed',
        message: (err as Error).message,
      });
    }
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const breadcrumbs = useMemo(
    () => [
      <Anchor key="scenarios" onClick={() => navigate('/scenarios')}>
        Scenarios
      </Anchor>,
      <Text key="current" c="dimmed">
        {isNew
          ? 'New scenario'
          : draft?.name ?? detail.data?.name ?? 'Loading…'}
      </Text>,
    ],
    [navigate, isNew, draft, detail.data],
  );

  if (!isNew && detail.isLoading && !draft) {
    return (
      <Stack gap="md">
        <Skeleton height={32} />
        <Skeleton height={140} />
        <Skeleton height={300} />
      </Stack>
    );
  }

  if (!isNew && detail.isError) {
    return (
      <Alert
        icon={<IconAlertTriangle size={16} />}
        color="red"
        title="Failed to load scenario"
      >
        {(detail.error as ApiError | Error).message}
      </Alert>
    );
  }

  if (!draft) return null;

  return (
    <Stack gap="md">
      <DirtyGuard dirty={dirty} />

      <Breadcrumbs>{breadcrumbs}</Breadcrumbs>

      <Card withBorder padding="md">
        <BuilderHeader
          isNew={isNew}
          isDirty={dirty}
          isSaving={isSaving}
          onSave={handleSave}
          onSaveAndRun={handleSaveAndRun}
          onDiscard={handleDiscardRequest}
          onDuplicate={handleDuplicate}
          onExport={handleExport}
          onImport={handleImportClick}
        />
        <input
          ref={fileInputRef}
          type="file"
          accept="application/json"
          style={{ display: 'none' }}
          onChange={handleImportFile}
          data-testid="builder-import-input"
        />
      </Card>

      {importError && (
        <Alert
          icon={<IconAlertTriangle size={16} />}
          color="red"
          title="Import failed"
          withCloseButton
          onClose={() => setImportError(null)}
          data-testid="builder-import-error"
        >
          {importError.kind === 'parse' ? (
            <Text size="sm">{importError.message}</Text>
          ) : (
            <Stack gap={4}>
              {importError.errors.map((e, i) => (
                <Text key={i} size="sm">
                  <strong>{e.path}</strong>: {e.message}
                </Text>
              ))}
            </Stack>
          )}
        </Alert>
      )}

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

      <BuilderTabs />

      <Modal
        opened={discardOpen}
        onClose={() => setDiscardOpen(false)}
        title="Discard changes?"
        centered
      >
        <Text size="sm" mb="md">
          Reverts to the last saved version. Unsaved edits will be lost.
        </Text>
        <Stack gap={0} align="flex-end">
          <Stack gap={8}>
            <Anchor
              component="button"
              type="button"
              c="red"
              onClick={handleDiscardConfirm}
              data-testid="builder-discard-confirm"
            >
              Yes, discard
            </Anchor>
            <Anchor
              component="button"
              type="button"
              onClick={() => setDiscardOpen(false)}
            >
              Keep editing
            </Anchor>
          </Stack>
        </Stack>
      </Modal>
    </Stack>
  );
}
