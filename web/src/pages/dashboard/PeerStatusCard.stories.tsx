import type { Meta, StoryObj } from '@storybook/react-vite';

import { peerFixtures } from '../../mocks/data/peers';
import { PeerStatusCard } from './PeerStatusCard';

const meta = {
  title: 'Dashboard/PeerStatusCard',
  component: PeerStatusCard,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof PeerStatusCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { peers: peerFixtures },
};

export const SinglePeer: Story = {
  args: { peers: [peerFixtures[0]] },
};

export const Empty: Story = {
  args: { peers: [] },
};
