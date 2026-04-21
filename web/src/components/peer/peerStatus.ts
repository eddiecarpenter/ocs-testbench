import type { PeerStatus } from '../../api/resources/peers';

export const STATUS_COLOR: Record<PeerStatus, string> = {
  connected: 'var(--mantine-color-teal-6)',
  disconnected: 'var(--mantine-color-gray-5)',
  error: 'var(--mantine-color-red-6)',
  connecting: 'var(--mantine-color-yellow-6)',
  disconnecting: 'var(--mantine-color-yellow-6)',
  restarting: 'var(--mantine-color-yellow-6)',
};

export const STATUS_LABEL: Record<PeerStatus, string> = {
  connected: 'Connected',
  disconnected: 'Disconnected',
  error: 'Error',
  connecting: 'Connecting…',
  disconnecting: 'Disconnecting…',
  restarting: 'Restarting…',
};
