/**
 * Builder footer — sticky action bar at the bottom of the editor
 * modal. Delete sits on the left (visually separated to flag its
 * destructive intent); the completion actions are right-aligned:
 *
 *   [ Delete (red outline) ] ……… [ Discard ] [ Save ]
 *
 *   - Delete  = destructive (existing scenarios only); transparent
 *               background, red border + text
 *   - Discard = secondary, enabled when dirty (resets and closes)
 *   - Save    = primary, enabled when dirty
 *
 * Sticky positioning: the footer pins to the bottom of the modal's
 * scrolling body. With `position: sticky; bottom: 0` it stays
 * visible no matter how far down the form the user has scrolled.
 *
 * Save & Run, Duplicate, and Import / Export were intentionally
 * dropped from the editor — Run lives on the list (and later on
 * Executions); Duplicate is a list-only operation; Import / Export
 * was removed because Duplicate covers ~80% of the same need (the
 * cross-installation use case is rare and better solved at the
 * backend layer when one exists).
 */
import { Button, Group, Tooltip } from '@mantine/core';

interface BuilderFooterProps {
  isNew: boolean;
  isDirty: boolean;
  isSaving: boolean;
  hasName: boolean;
  hasPeer: boolean;
  hasSubscriber: boolean;
  onSave: () => void;
  onDiscard: () => void;
  onDelete: () => void;
}

export function BuilderFooter({
  isNew,
  isDirty,
  isSaving,
  hasName,
  hasPeer,
  hasSubscriber,
  onSave,
  onDiscard,
  onDelete,
}: BuilderFooterProps) {
  const missing = [
    !hasName && 'scenario name',
    !hasPeer && 'peer',
    !hasSubscriber && 'subscriber',
  ].filter(Boolean);
  const canSave = isDirty && missing.length === 0;

  return (
    <Group
      justify="space-between"
      gap="xs"
      wrap="nowrap"
      style={{
        background: 'var(--mantine-color-body)',
        borderTop: '1px solid var(--mantine-color-default-border)',
        marginInline: 'calc(var(--mantine-spacing-md) * -1)',
        marginBlockEnd: 'calc(var(--mantine-spacing-md) * -1)',
        paddingInline: 'var(--mantine-spacing-md)',
        paddingBlock: 'var(--mantine-spacing-sm)',
      }}
    >
      <Button
        color="red"
        variant="outline"
        disabled={isNew}
        onClick={onDelete}
        data-testid="builder-delete"
      >
        Delete
      </Button>
      <Group gap="xs" wrap="wrap" justify="flex-end">
        <Button
          variant="default"
          disabled={!isDirty}
          onClick={onDiscard}
          data-testid="builder-discard"
        >
          Discard
        </Button>
        <Tooltip
          label={missing.length > 0 ? `Required: ${missing.join(', ')}` : ''}
          disabled={missing.length === 0}
          position="top"
        >
          <Button
            onClick={onSave}
            loading={isSaving}
            disabled={!canSave}
            data-testid="builder-save"
          >
            Save
          </Button>
        </Tooltip>
      </Group>
    </Group>
  );
}
