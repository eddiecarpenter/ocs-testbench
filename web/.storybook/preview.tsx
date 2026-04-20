import { MantineProvider, useMantineColorScheme } from '@mantine/core';
import type { Decorator, Preview } from '@storybook/react-vite';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { theme } from '../src/theme/theme';

import '@mantine/core/styles.css';
import '@mantine/charts/styles.css';
import '@mantine/notifications/styles.css';

// eslint-disable-next-line react-refresh/only-export-components
function ColorSchemeSync({ scheme }: { scheme: 'light' | 'dark' }) {
  const { setColorScheme } = useMantineColorScheme();
  useEffect(() => {
    setColorScheme(scheme);
  }, [scheme, setColorScheme]);
  return null;
}

const withMantine: Decorator = (Story, context) => {
  const scheme = (context.globals.theme as 'light' | 'dark') ?? 'light';
  return (
    <MantineProvider theme={theme} defaultColorScheme={scheme}>
      <ColorSchemeSync scheme={scheme} />
      <MemoryRouter>
        <div
          style={{
            minHeight: '100vh',
            background: 'var(--mantine-color-body)',
            padding: 16,
          }}
        >
          <Story />
        </div>
      </MemoryRouter>
    </MantineProvider>
  );
};

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    a11y: { test: 'todo' },
    layout: 'fullscreen',
  },
  globalTypes: {
    theme: {
      description: 'Colour scheme',
      defaultValue: 'light',
      toolbar: {
        title: 'Theme',
        icon: 'circlehollow',
        items: [
          { value: 'light', title: 'Light', icon: 'sun' },
          { value: 'dark', title: 'Dark', icon: 'moon' },
        ],
        dynamicTitle: true,
      },
    },
  },
  decorators: [withMantine],
};

export default preview;
