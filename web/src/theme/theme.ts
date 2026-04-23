import { createTheme, rem, type MantineColorsTuple } from '@mantine/core';

/**
 * Brand blue — derived from the v2 Figma palette. Shade 5 is the resting
 * accent (logo, primary buttons), shade 6 is the active-state/selected
 * accent (nav border, focus outline).
 */
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
  primaryShade: { light: 5, dark: 4 },
  colors: {
    brand,
  },
  defaultRadius: 'md',
  fontFamily:
    '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
  fontFamilyMonospace:
    'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
  fontSizes: {
    xs: rem(12),
    sm: rem(13),
    md: rem(14),
    lg: rem(16),
    xl: rem(20),
  },
  lineHeights: {
    xs: '1.4',
    sm: '1.45',
    md: '1.5',
    lg: '1.55',
    xl: '1.6',
  },
  headings: {
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
    fontWeight: '600',
    sizes: {
      h1: { fontSize: rem(28), lineHeight: '1.3' },
      h2: { fontSize: rem(22), lineHeight: '1.35' },
      h3: { fontSize: rem(18), lineHeight: '1.4' },
      h4: { fontSize: rem(16), lineHeight: '1.45' },
      h5: { fontSize: rem(14), lineHeight: '1.5' },
    },
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
    Input: {
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
