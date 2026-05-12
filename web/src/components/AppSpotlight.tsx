import { Spotlight } from '@mantine/spotlight';
import {
  IconDashboard,
  IconLayoutGrid,
  IconPlayerPlay,
  IconRouter,
  IconSettings,
  IconUsers,
} from '@tabler/icons-react';
import { useNavigate } from 'react-router';

import '@mantine/spotlight/styles.css';

const NAV_ACTIONS = [
  { id: 'dashboard',   label: 'Dashboard',   description: 'Overview and KPIs',              icon: IconDashboard,   to: '/' },
  { id: 'peers',       label: 'Peers',        description: 'Manage Diameter peer connections', icon: IconRouter,      to: '/peers' },
  { id: 'subscribers', label: 'Subscribers',  description: 'Manage subscriber profiles',       icon: IconUsers,       to: '/subscribers' },
  { id: 'scenarios',   label: 'Scenarios',    description: 'Author and manage test scenarios', icon: IconLayoutGrid,  to: '/scenarios' },
  { id: 'executions',  label: 'Executions',   description: 'View and debug test executions',   icon: IconPlayerPlay,  to: '/executions' },
  { id: 'settings',    label: 'Settings',     description: 'Configure testbench preferences',  icon: IconSettings,    to: '/settings' },
];

/**
 * AppSpotlight — ⌘K command palette for page navigation.
 *
 * Render this once inside the router (App.tsx) so `useNavigate` works.
 * Trigger it from anywhere with `spotlight.open()`.
 */
export function AppSpotlight() {
  const navigate = useNavigate();

  const actions = NAV_ACTIONS.map(({ id, label, description, icon: Icon, to }) => ({
    id,
    label,
    description,
    leftSection: <Icon size={18} stroke={1.5} />,
    onClick: () => navigate(to),
  }));

  return (
    <Spotlight
      actions={actions}
      shortcut={['mod+K']}
      nothingFound="No matching pages"
      searchProps={{ placeholder: 'Go to…' }}
      highlightQuery
    />
  );
}

