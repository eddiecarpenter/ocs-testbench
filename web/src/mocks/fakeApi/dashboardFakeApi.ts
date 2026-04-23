import type { DashboardKpis } from '../../api/resources/dashboard';
import { executionFixtures } from '../data/executions';
import { peerFixtures } from '../data/peers';
import { scenarioFixtures } from '../data/scenarios';
import { subscriberFixtures } from '../data/subscribers';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/dashboard\/kpis$/)
  .withDelayInMs(200)
  .reply((): [number, DashboardKpis] => {
    const connected = peerFixtures.filter((p) => p.status === 'connected').length;
    const activeRuns = executionFixtures.filter(
      (e) => e.state === 'running' || e.state === 'paused',
    ).length;

    return [
      200,
      {
        peers: { connected, total: peerFixtures.length },
        subscribers: subscriberFixtures.length,
        scenarios: scenarioFixtures.length,
        activeRuns,
      },
    ];
  });
