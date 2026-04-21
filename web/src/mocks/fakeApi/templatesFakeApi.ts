import { templateFixtures } from '../data/templates';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/templates$/)
  .withDelayInMs(200)
  .reply(() => [200, templateFixtures]);
