import { Center, Stack, Text, Title } from '@mantine/core';

interface PlaceholderPageProps {
  title: string;
}

export function PlaceholderPage({ title }: PlaceholderPageProps) {
  return (
    <Center py="xl" style={{ minHeight: 400 }}>
      <Stack align="center" gap="xs">
        <Title order={3}>{title}</Title>
        <Text c="dimmed" size="sm">
          Coming soon — screen is scoped in Figma and will be implemented next.
        </Text>
      </Stack>
    </Center>
  );
}
