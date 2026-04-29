/**
 * Thin wrapper around Mantine's `notifications.show` that enforces
 * the project's notification UX:
 *
 *   - Errors (`notifyError`) are sticky — `autoClose: false` — so the
 *     user has time to read the message and dismiss it deliberately.
 *     A toast that vanishes before the user can read it is worse than
 *     no toast at all.
 *   - Success / warning / info toasts use Mantine's defaults.
 *
 * Use this helper instead of calling `notifications.show` directly
 * for failure paths. Success / info toasts can keep using
 * `notifications.show` — `notifySuccess` is provided as a thin alias
 * for symmetry but is optional.
 */
import { notifications, type NotificationData } from '@mantine/notifications';

type Optional<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;

export type NotifyArgs = Optional<NotificationData, 'color' | 'autoClose'>;

/** Sticky error toast — does not auto-close. */
export function notifyError(args: NotifyArgs): void {
  notifications.show({
    color: 'red',
    autoClose: false,
    withCloseButton: true,
    ...args,
  });
}

/** Auto-closing success toast (Mantine default behaviour). */
export function notifySuccess(args: NotifyArgs): void {
  notifications.show({
    color: 'green',
    ...args,
  });
}

/** Auto-closing warning toast. */
export function notifyWarning(args: NotifyArgs): void {
  notifications.show({
    color: 'yellow',
    ...args,
  });
}
