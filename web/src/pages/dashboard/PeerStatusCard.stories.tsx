import type { Meta, StoryObj } from '@storybook/react-vite';
import { PeerStatusCard } from './PeerStatusCard';
import { mockPeers } from '../../mock/dashboard';

const meta = {
  title: 'Dashboard/PeerStatusCard',
  component: PeerStatusCard,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof PeerStatusCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { peers: mockPeers },
};

export const SinglePeer: Story = {
  args: { peers: [mockPeers[0]] },
};

export const Empty: Story = {
  args: { peers: [] },
};
