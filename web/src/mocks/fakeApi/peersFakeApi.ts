import type { Peer, PeerInput } from '../../api/resources/peers';
import { peerFixtures } from '../data/peers';
import { mock } from '../MockAdapter';

/** Working copy so POST/PUT/DELETE survive across calls within a session. */
const peers: Peer[] = peerFixtures.map((p) => ({ ...p }));

type FieldErrors = Record<string, string[]>;

/** Minimal field validator — keeps the UI contract honest. */
function validate(input: Partial<PeerInput> | undefined): FieldErrors | null {
  const errors: FieldErrors = {};
  const name = input?.name?.trim();
  const endpoint = input?.endpoint?.trim();
  const originHost = input?.originHost?.trim();

  if (!name) errors['/name'] = ['Name is required'];
  else if (name.length > 64) errors['/name'] = ['Name must be 64 chars or less'];
  // Contrived-but-plausible: block "reserved" as a demo of a server-only rule
  // the client can't anticipate.
  else if (name.toLowerCase() === 'reserved')
    errors['/name'] = ['Name "reserved" is not allowed'];

  if (!endpoint) errors['/endpoint'] = ['Endpoint is required'];
  else if (!/^[^:]+:[0-9]+$/.test(endpoint))
    errors['/endpoint'] = ['Endpoint must be host:port'];

  if (!originHost) errors['/originHost'] = ['Origin host is required'];

  return Object.keys(errors).length > 0 ? errors : null;
}

function validationProblem(
  errors: FieldErrors,
): [number, Record<string, unknown>] {
  return [
    422,
    {
      type: 'about:blank',
      title: 'Validation failed',
      status: 422,
      detail: 'One or more fields are invalid',
      errors,
    },
  ];
}

// List
mock
  .onGet(/\/peers$/)
  .withDelayInMs(250)
  .reply(() => [200, peers]);

// Detail
mock.onGet(/\/peers\/[^/]+$/).reply((config) => {
  const m = /\/peers\/([^/]+)$/.exec(config.url ?? '');
  const id = m ? decodeURIComponent(m[1]) : '';
  const peer = peers.find((p) => p.id === id);
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

// Create
mock
  .onPost(/\/peers$/)
  .withDelayInMs(300)
  .reply((config) => {
    const input = safeParse<PeerInput>(config.data);
    const errors = validate(input);
    if (errors) return validationProblem(errors);

    // Name uniqueness — demonstrates a field-level business rule.
    if (peers.some((p) => p.name === input!.name)) {
      return validationProblem({
        '/name': [`Name "${input!.name}" is already in use`],
      });
    }

    const id = `peer-${String(peers.length + 1).padStart(2, '0')}`;
    const created: Peer = {
      id,
      name: input!.name,
      endpoint: input!.endpoint,
      originHost: input!.originHost,
      status: 'disconnected',
      lastChangeAt: new Date().toISOString(),
    };
    peers.push(created);
    return [201, created];
  });

// Update
mock
  .onPut(/\/peers\/[^/]+$/)
  .withDelayInMs(300)
  .reply((config) => {
    const m = /\/peers\/([^/]+)$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const idx = peers.findIndex((p) => p.id === id);
    if (idx === -1) {
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

    const input = safeParse<PeerInput>(config.data);
    const errors = validate(input);
    if (errors) return validationProblem(errors);

    if (peers.some((p) => p.id !== id && p.name === input!.name)) {
      return validationProblem({
        '/name': [`Name "${input!.name}" is already in use`],
      });
    }

    const updated: Peer = {
      ...peers[idx],
      name: input!.name,
      endpoint: input!.endpoint,
      originHost: input!.originHost,
      lastChangeAt: new Date().toISOString(),
    };
    peers[idx] = updated;
    return [200, updated];
  });

// Delete
mock
  .onDelete(/\/peers\/[^/]+$/)
  .withDelayInMs(200)
  .reply((config) => {
    const m = /\/peers\/([^/]+)$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const idx = peers.findIndex((p) => p.id === id);
    if (idx === -1) {
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
    peers.splice(idx, 1);
    return [204];
  });

function safeParse<T>(data: unknown): T | undefined {
  if (typeof data === 'string') {
    try {
      return JSON.parse(data) as T;
    } catch {
      return undefined;
    }
  }
  return data as T | undefined;
}
