import { scenarioFixtures } from '../data/scenarios';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/scenarios$/)
  .withDelayInMs(200)
  .reply(() => [200, scenarioFixtures]);
