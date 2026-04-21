import { Group, Text } from '@mantine/core';

import type { PeerStatus } from '../../api/resources/peers';
import { STATUS_COLOR, STATUS_LABEL } from './peerStatus';

/** Small circular status indicator — reused across dashboard + peers list. */
export function StatusDot({ status }: { status: PeerStatus }) {
  return (
    <span
      aria-label={STATUS_LABEL[status]}
      style={{
        display: 'inline-block',
        flex: 'none',
        width: 8,
        height: 8,
        borderRadius: '50%',
        background: STATUS_COLOR[status],
      }}
    />
  );
}

/**
 * Coloured-dot + text-label composition that the Figma design uses in the
 * peers table and anywhere else peer status appears inline.
 */
export function PeerStatusLabel({ status }: { status: PeerStatus }) {
  return (
    <Group gap="xs" wrap="nowrap">
      <StatusDot status={status} />
      <Text size="sm">{STATUS_LABEL[status]}</Text>
    </Group>
  );
}
