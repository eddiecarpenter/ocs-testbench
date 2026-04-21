import type {
  Peer,
  PeerInput,
  PeerTestResult,
} from '../../api/resources/peers';
import { peerFixtures } from '../data/peers';
import { mock } from '../MockAdapter';

/** Working copy so POST/PUT/DELETE survive across calls within a session. */
const peers: Peer[] = peerFixtures.map((p) => ({ ...p }));

type FieldErrors = Record<string, string[]>;

/** Minimal field validator — keeps the UI contract honest. */
function validate(input: Partial<PeerInput> | undefined): FieldErrors | null {
  const errors: FieldErrors = {};
  const name = input?.name?.trim();
  const host = input?.host?.trim();
  const port = input?.port;
  const originHost = input?.originHost?.trim();
  const originRealm = input?.originRealm?.trim();
  const transport = input?.transport;
  const watchdog = input?.watchdogIntervalSeconds;

  if (!name) errors['/name'] = ['Name is required'];
  else if (name.length > 64) errors['/name'] = ['Name must be 64 chars or less'];
  // Contrived-but-plausible: block "reserved" as a demo of a server-only rule
  // the client can't anticipate.
  else if (name.toLowerCase() === 'reserved')
    errors['/name'] = ['Name "reserved" is not allowed'];

  if (!host) errors['/host'] = ['Host is required'];

  if (typeof port !== 'number' || !Number.isInteger(port))
    errors['/port'] = ['Port is required'];
  else if (port < 1 || port > 65535)
    errors['/port'] = ['Port must be between 1 and 65535'];

  if (!originHost) errors['/originHost'] = ['Origin-Host is required'];
  if (!originRealm) errors['/originRealm'] = ['Origin-Realm is required'];

  if (transport !== 'TCP' && transport !== 'TLS')
    errors['/transport'] = ['Transport must be TCP or TLS'];

  if (typeof watchdog !== 'number' || !Number.isInteger(watchdog))
    errors['/watchdogIntervalSeconds'] = ['Watchdog interval is required'];
  else if (watchdog < 5 || watchdog > 3600)
    errors['/watchdogIntervalSeconds'] = [
      'Watchdog interval must be between 5 and 3600 seconds',
    ];

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

function applyInput(base: Partial<Peer>, input: PeerInput): Peer {
  return {
    id: base.id!,
    name: input.name,
    host: input.host,
    port: input.port,
    originHost: input.originHost,
    originRealm: input.originRealm,
    transport: input.transport,
    watchdogIntervalSeconds: input.watchdogIntervalSeconds,
    autoConnect: input.autoConnect,
    status: base.status ?? 'disconnected',
    statusDetail: base.statusDetail,
    lastChangeAt: new Date().toISOString(),
  };
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
    const created = applyInput({ id, status: 'disconnected' }, input!);
    peers.push(created);
    return [201, created];
  });

// Test — CER/CEA dry-run against an arbitrary candidate config.
// Intentionally stateless: does not look at `peers[]`, only the body.
mock
  .onPost(/\/peers\/test$/)
  .withDelayInMs(600)
  .reply((config): [number, PeerTestResult | Record<string, unknown>] => {
    const input = safeParse<PeerInput>(config.data);
    const errors = validate(input);
    if (errors) return validationProblem(errors);

    // Contrived demo rule: hosts in 10.0.99.x simulate a CER/CEA timeout
    // so the failure path is exercisable.
    if (input!.host.startsWith('10.0.99.')) {
      return [
        200,
        {
          ok: false,
          durationMs: 1_500,
          detail: `CER/CEA timeout (no response from ${input!.host}:${input!.port})`,
        },
      ];
    }

    return [
      200,
      {
        ok: true,
        durationMs: 42 + Math.floor(Math.random() * 30),
        detail: `Capability exchange OK (${input!.transport})`,
      },
    ];
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

    const updated = applyInput(peers[idx], input!);
    peers[idx] = updated;
    return [200, updated];
  });

// Connect — move peer to `connected` (with a brief `connecting` stop). The
// second SSE-style flip would normally come from the server's transport
// supervisor; the mock just returns the settled state for simplicity.
mock
  .onPost(/\/peers\/[^/]+\/connect$/)
  // Held long enough that the `connecting` transient is visibly on screen
  // — a few hundred ms flickers past too fast to read.
  .withDelayInMs(1500)
  .reply((config) => {
    const m = /\/peers\/([^/]+)\/connect$/.exec(config.url ?? '');
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
    // Contrived: hosts in 10.0.99.x cannot connect (same rule as the test
    // probe) so the failure path is exercisable.
    if (peer.host.startsWith('10.0.99.')) {
      peer.status = 'error';
      peer.statusDetail = 'CER/CEA timeout';
    } else {
      peer.status = 'connected';
      peer.statusDetail = undefined;
    }
    peer.lastChangeAt = new Date().toISOString();
    return [200, peer];
  });

// Disconnect — explicit disconnect. Supervision does not auto-reconnect.
mock
  .onPost(/\/peers\/[^/]+\/disconnect$/)
  .withDelayInMs(1200)
  .reply((config) => {
    const m = /\/peers\/([^/]+)\/disconnect$/.exec(config.url ?? '');
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
    peer.status = 'disconnected';
    peer.statusDetail = 'Administratively disconnected';
    peer.lastChangeAt = new Date().toISOString();
    return [200, peer];
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
