import { peerFixtures } from '../data/peers';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/peers$/)
  .withDelayInMs(250)
  .reply(() => [200, peerFixtures]);

mock.onGet(/\/peers\/[^/]+$/).reply((config) => {
  const m = /\/peers\/([^/]+)$/.exec(config.url ?? '');
  const id = m ? decodeURIComponent(m[1]) : '';
  const peer = peerFixtures.find((p) => p.id === id);
  if (!peer) {
    return [
      404,
      {
        type: 'about:blank',
        title: 'Peer not found',
        status: 404,
        detail: `No peer with id "${id}"`,
      },
    ];
  }
  return [200, peer];
});
