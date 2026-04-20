import { createTheme, rem, type MantineColorsTuple } from '@mantine/core';

// Brand blue used across Figma designs — keyed to match the existing palette
const brand: MantineColorsTuple = [
  '#eaf1ff',
  '#cddeff',
  '#9bbaff',
  '#6495ff',
  '#3b77ff',
  '#2262eb',
  '#1a53d1',
  '#1445ad',
  '#0e3788',
  '#082a6b',
];

export const theme = createTheme({
  primaryColor: 'brand',
  colors: {
    brand,
  },
  defaultRadius: 'md',
  fontFamily:
    '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
  fontFamilyMonospace:
    'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
  headings: {
    fontWeight: '600',
  },
  components: {
    Card: {
      defaultProps: {
        withBorder: true,
        radius: 'md',
      },
    },
    Button: {
      defaultProps: {
        radius: 'md',
      },
    },
  },
});

export const dimensions = {
  navbarWidth: rem(240),
  headerHeight: rem(60),
};
