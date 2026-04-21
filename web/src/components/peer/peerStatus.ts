import type { PeerStatus } from '../../api/resources/peers';

export const STATUS_COLOR: Record<PeerStatus, string> = {
  // Stopped = admin down, no supervision. A deeper gray than `disconnected`
  // (which is supervised-but-currently-down) — the distinction matters for
  // the operator: `stopped` peers will not self-heal, `disconnected` peers
  // are already retrying.
  stopped: 'var(--mantine-color-gray-7)',
  connected: 'var(--mantine-color-teal-6)',
  disconnected: 'var(--mantine-color-gray-5)',
  error: 'var(--mantine-color-red-6)',
  connecting: 'var(--mantine-color-yellow-6)',
  disconnecting: 'var(--mantine-color-yellow-6)',
  restarting: 'var(--mantine-color-yellow-6)',
};

export const STATUS_LABEL: Record<PeerStatus, string> = {
  stopped: 'Stopped',
  connected: 'Connected',
  disconnected: 'Disconnected',
  error: 'Error',
  connecting: 'Connecting…',
  disconnecting: 'Disconnecting…',
  restarting: 'Restarting…',
};
