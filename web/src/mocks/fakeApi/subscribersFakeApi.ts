import type { SubscriberPage } from '../../api/resources/subscribers';
import { subscriberFixtures } from '../data/subscribers';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/subscribers(\?|$)/)
  .withDelayInMs(250)
  .reply((config): [number, SubscriberPage] => {
    const limit = Math.min(500, Math.max(1, Number(config.params?.limit ?? 50)));
    const offset = Math.max(0, Number(config.params?.offset ?? 0));
    const items = subscriberFixtures.slice(offset, offset + limit);
    return [
      200,
      {
        items,
        page: { total: subscriberFixtures.length, limit, offset },
      },
    ];
  });
