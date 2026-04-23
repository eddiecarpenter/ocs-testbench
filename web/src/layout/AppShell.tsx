import {
  ActionIcon,
  AppShell as MantineAppShell,
  Badge,
  Box,
  Divider,
  Group,
  NavLink,
  Stack,
  TextInput,
  Title,
  UnstyledButton,
  useMantineColorScheme,
} from '@mantine/core';
import {
  IconDashboard,
  IconHelp,
  IconLayoutGrid,
  IconMoon,
  IconPlayerPlay,
  IconRouter,
  IconSearch,
  IconSettings,
  IconSun,
  IconUsers,
} from '@tabler/icons-react';
import { NavLink as RouterLink, Outlet, useLocation } from 'react-router';

type NavEntry = {
  label: string;
  to: string;
  icon: React.ComponentType<{ size?: number | string; stroke?: number }>;
};

const primaryNav: NavEntry[] = [
  { label: 'Dashboard', to: '/', icon: IconDashboard },
  { label: 'Peers', to: '/peers', icon: IconRouter },
  { label: 'Subscribers', to: '/subscribers', icon: IconUsers },
  { label: 'Scenarios', to: '/scenarios', icon: IconLayoutGrid },
  { label: 'Execution', to: '/execution', icon: IconPlayerPlay },
];

function ThemeToggle() {
  const { colorScheme, toggleColorScheme } = useMantineColorScheme();
  return (
    <ActionIcon
      variant="subtle"
      color="gray"
      size="lg"
      aria-label="Toggle colour scheme"
      onClick={() => toggleColorScheme()}
    >
      {colorScheme === 'dark' ? <IconSun size={18} /> : <IconMoon size={18} />}
    </ActionIcon>
  );
}

export function AppShell() {
  const { pathname } = useLocation();

  return (
    <MantineAppShell
      header={{ height: 60 }}
      navbar={{ width: 240, breakpoint: 'sm' }}
      padding="md"
    >
      <MantineAppShell.Header
        style={{
          borderBottom: '1px solid var(--mantine-color-gray-3)',
        }}
      >
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <UnstyledButton
            component={RouterLink}
            to="/"
            aria-label="Go to Dashboard"
          >
            <Group gap="sm" wrap="nowrap">
              <Box
                w={28}
                h={28}
                style={{
                  borderRadius: 6,
                  background: 'var(--mantine-color-brand-5)',
                }}
              />
              <Title order={4} fw={600} c="var(--mantine-color-text)">
                OCS Testbench
              </Title>
            </Group>
          </UnstyledButton>

          <Group gap="sm" wrap="nowrap">
            <TextInput
              placeholder="Search..."
              leftSection={<IconSearch size={14} />}
              rightSection={
                <Badge size="xs" variant="default" radius="sm">
                  ⌘K
                </Badge>
              }
              rightSectionWidth={42}
              w={260}
              size="sm"
              visibleFrom="sm"
            />
            <ThemeToggle />
            <ActionIcon variant="subtle" color="gray" size="lg" aria-label="Help">
              <IconHelp size={18} />
            </ActionIcon>
          </Group>
        </Group>
      </MantineAppShell.Header>

      <MantineAppShell.Navbar
        p="sm"
        bg="var(--mantine-color-body)"
        style={{
          borderRight: '1px solid var(--mantine-color-gray-3)',
        }}
      >
        <Stack gap={2} h="100%">
          {primaryNav.map((entry) => {
            const Icon = entry.icon;
            const isActive =
              entry.to === '/'
                ? pathname === '/'
                : pathname.startsWith(entry.to);
            return (
              <NavLink
                key={entry.to}
                component={RouterLink}
                to={entry.to}
                label={entry.label}
                leftSection={<Icon size={18} stroke={1.6} />}
                active={isActive}
                variant="light"
                color="brand"
                styles={{
                  root: {
                    borderRadius: 'var(--mantine-radius-sm)',
                    borderLeft: isActive
                      ? '3px solid var(--mantine-color-brand-6)'
                      : '3px solid transparent',
                    paddingLeft: 'calc(var(--mantine-spacing-sm) - 3px)',
                  },
                  label: {
                    fontWeight: isActive ? 600 : 500,
                  },
                }}
              />
            );
          })}

          <Box mt="auto">
            <Divider my="sm" />
            <NavLink
              component={RouterLink}
              to="/settings"
              label="Settings"
              leftSection={<IconSettings size={18} stroke={1.6} />}
              active={pathname.startsWith('/settings')}
              variant="light"
              color="brand"
              styles={{
                root: {
                  borderRadius: 'var(--mantine-radius-sm)',
                  borderLeft: pathname.startsWith('/settings')
                    ? '3px solid var(--mantine-color-brand-6)'
                    : '3px solid transparent',
                  paddingLeft: 'calc(var(--mantine-spacing-sm) - 3px)',
                },
                label: {
                  fontWeight: pathname.startsWith('/settings') ? 600 : 500,
                },
              }}
            />
          </Box>
        </Stack>
      </MantineAppShell.Navbar>

      <MantineAppShell.Main bg="var(--mantine-color-gray-0)">
        <Outlet />
      </MantineAppShell.Main>
    </MantineAppShell>
  );
}

