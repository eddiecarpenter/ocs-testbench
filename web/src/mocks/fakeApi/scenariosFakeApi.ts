import type { ScenarioSummary } from '../../api/resources/scenarios';
import { scenarioFixtures } from '../data/scenarios';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/scenarios$/)
  .withDelayInMs(200)
  .reply((): [number, ScenarioSummary[]] => [200, scenarioFixtures]);
