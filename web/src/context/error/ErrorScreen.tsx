import { Button, Center, Code, Stack, Text, Title } from '@mantine/core';
import { IconBug } from '@tabler/icons-react';

import { useError } from './useError';

export function ErrorScreen() {
  const { error, clear } = useError();

  return (
    <Center mih="100dvh" p="lg">
      <Stack align="center" gap="md" maw={560} ta="center">
        <IconBug size={56} stroke={1.4} color="var(--mantine-color-red-6)" />
        <Title order={2} fw={600}>
          Something went wrong
        </Title>
        <Text c="dimmed">
          {error?.message ??
            'An unexpected error occurred. Please try again, and if the problem continues, contact support.'}
        </Text>
        {error?.time && (
          <Text size="xs" c="dimmed">
            {new Date(error.time).toLocaleString()}
          </Text>
        )}
        {error?.stack && (
          <Code
            block
            style={{
              maxHeight: 200,
              overflow: 'auto',
              textAlign: 'left',
              width: '100%',
            }}
          >
            {error.stack}
          </Code>
        )}
        <Stack gap="xs" w="100%">
          <Button onClick={clear} variant="light">
            Try again
          </Button>
          <Button
            onClick={() => globalThis.location.reload()}
            variant="subtle"
            color="gray"
          >
            Reload the page
          </Button>
        </Stack>
      </Stack>
    </Center>
  );
}
