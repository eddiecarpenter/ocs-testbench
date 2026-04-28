/**
 * Builder tab strip + tab body shell.
 *
 * The active tab is driven by the `?tab=` query param so deep links and
 * browser refresh land on the right tab. Each tab body is implemented in
 * its own module — Steps (Task 4), Frame (Task 5), Services (Task 6),
 * Variables (Task 7). This shell only owns the strip + the URL contract.
 */
import { Card, Stack, Tabs } from '@mantine/core';
import { useCallback } from 'react';
import { useSearchParams } from 'react-router';

import { FrameTab } from './tabs/FrameTab';
import { ServicesTab } from './tabs/ServicesTab';
import { StepsTab } from './tabs/StepsTab';
import { VariablesTab } from './tabs/VariablesTab';

export type BuilderTabId = 'steps' | 'frame' | 'services' | 'variables';

const TAB_IDS: BuilderTabId[] = ['steps', 'frame', 'services', 'variables'];

function isTab(value: string | null): value is BuilderTabId {
  return value !== null && (TAB_IDS as string[]).includes(value);
}

export function BuilderTabs() {
  const [params, setParams] = useSearchParams();
  const requested = params.get('tab');
  const active: BuilderTabId = isTab(requested) ? requested : 'steps';

  const setActive = useCallback(
    (value: BuilderTabId) => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set('tab', value);
          return next;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  return (
    <Card withBorder padding="md">
      <Tabs
        value={active}
        onChange={(v) => v && setActive(v as BuilderTabId)}
        keepMounted={false}
      >
        <Tabs.List>
          <Tabs.Tab value="steps" data-testid="builder-tab-steps">
            Steps
          </Tabs.Tab>
          <Tabs.Tab value="frame" data-testid="builder-tab-frame">
            Frame
          </Tabs.Tab>
          <Tabs.Tab value="services" data-testid="builder-tab-services">
            Services
          </Tabs.Tab>
          <Tabs.Tab value="variables" data-testid="builder-tab-variables">
            Variables
          </Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="steps" pt="md">
          <Stack gap="md">
            <StepsTab />
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="frame" pt="md">
          <Stack gap="md">
            <FrameTab />
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="services" pt="md">
          <Stack gap="md">
            <ServicesTab />
          </Stack>
        </Tabs.Panel>
        <Tabs.Panel value="variables" pt="md">
          <Stack gap="md">
            <VariablesTab />
          </Stack>
        </Tabs.Panel>
      </Tabs>
    </Card>
  );
}
