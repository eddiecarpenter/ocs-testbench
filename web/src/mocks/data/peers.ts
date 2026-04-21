import type { Peer } from '../../api/resources/peers';

/** 5 peers, 3 connected, 1 disconnected, 1 error — matches dashboard KPIs. */
export const peerFixtures: Peer[] = [
  {
    id: 'peer-01',
    name: 'peer-01',
    endpoint: '10.0.1.5:3868',
    originHost: 'ctf-01.test.local',
    status: 'connected',
    lastChangeAt: '2026-04-21T09:12:04Z',
  },
  {
    id: 'peer-02',
    name: 'peer-02',
    endpoint: '10.0.1.6:3868',
    originHost: 'ctf-02.test.local',
    status: 'connected',
    lastChangeAt: '2026-04-21T08:45:00Z',
  },
  {
    id: 'peer-03',
    name: 'peer-03',
    endpoint: '10.0.2.5:3868',
    originHost: 'ctf-03.test.local',
    status: 'error',
    statusDetail: 'CER/CEA timeout',
    lastChangeAt: '2026-04-21T09:03:22Z',
  },
  {
    id: 'peer-04',
    name: 'peer-04',
    endpoint: '10.0.2.6:3868',
    originHost: 'ctf-04.test.local',
    status: 'connected',
    lastChangeAt: '2026-04-21T09:00:00Z',
  },
  {
    id: 'peer-05',
    name: 'peer-05',
    endpoint: '10.0.3.7:3868',
    originHost: 'ctf-05.test.local',
    status: 'disconnected',
    statusDetail: 'Administratively disabled',
    lastChangeAt: '2026-04-20T17:10:00Z',
  },
];
