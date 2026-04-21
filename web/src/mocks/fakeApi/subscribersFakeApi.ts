import type {
  Subscriber,
  SubscriberInput,
  TacEntry,
} from '../../api/resources/subscribers';
import { isValidImei } from '../../api/imei';
import { subscriberFixtures } from '../data/subscribers';
import tacCatalogDoc from '../data/tacCatalog.json';
import { mock } from '../MockAdapter';

/** Working copy so POST/PUT/DELETE survive across calls within a session. */
const subscribers: Subscriber[] = subscriberFixtures.map((s) => ({ ...s }));

// Narrow the JSON-import shape to TacEntry[] — the `$schema` metadata at
// the top of the file is intentional editorial scaffolding, not part of
// the API response.
const tacCatalog: TacEntry[] = (tacCatalogDoc as { entries: TacEntry[] })
  .entries;
const tacSet = new Set(tacCatalog.map((e) => e.tac));

type FieldErrors = Record<string, string[]>;

/**
 * Minimal field validator. Patterns match the OpenAPI spec; extra
 * cross-field rules (TAC catalogue lookup, IMEI/TAC consistency, Luhn
 * validity) are checked here so the fake API exercises the same 422
 * contract the real backend will.
 */
function validate(
  input: Partial<SubscriberInput> | undefined,
  existing: Subscriber[],
  excludeId?: string,
): FieldErrors | null {
  const errors: FieldErrors = {};
  const name = input?.name?.trim();
  const msisdn = input?.msisdn?.trim();
  const iccid = input?.iccid?.trim();
  const tac = input?.tac?.trim() || undefined;
  const imei = input?.imei?.trim() || undefined;

  if (!name) errors['/name'] = ['Name is required'];
  else if (name.length > 64) errors['/name'] = ['Name must be 64 chars or less'];

  if (!msisdn) errors['/msisdn'] = ['MSISDN is required'];
  else if (!/^[0-9]{8,15}$/.test(msisdn))
    errors['/msisdn'] = ['MSISDN must be 8–15 digits'];
  else if (
    existing.some((s) => s.id !== excludeId && s.msisdn === msisdn)
  )
    errors['/msisdn'] = [`MSISDN "${msisdn}" is already in use`];

  if (!iccid) errors['/iccid'] = ['ICCID is required'];
  else if (!/^[0-9]{19,20}$/.test(iccid))
    errors['/iccid'] = ['ICCID must be 19 or 20 digits'];
  else if (
    existing.some((s) => s.id !== excludeId && s.iccid === iccid)
  )
    errors['/iccid'] = [`ICCID "${iccid}" is already in use`];

  if (tac !== undefined) {
    if (!/^[0-9]{8}$/.test(tac))
      errors['/tac'] = ['TAC must be 8 digits'];
    else if (!tacSet.has(tac))
      errors['/tac'] = ['TAC is not in the catalogue'];
  }

  if (imei !== undefined) {
    if (!/^[0-9]{15}$/.test(imei))
      errors['/imei'] = ['IMEI must be 15 digits'];
    else if (!isValidImei(imei))
      errors['/imei'] = ['IMEI has an invalid Luhn check digit'];
    else if (tac && !imei.startsWith(tac))
      errors['/imei'] = [
        'IMEI prefix does not match the chosen manufacturer/model',
      ];
  }

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

function applyInput(base: Partial<Subscriber>, input: SubscriberInput): Subscriber {
  return {
    id: base.id!,
    name: input.name,
    msisdn: input.msisdn,
    iccid: input.iccid,
    tac: input.tac || undefined,
    imei: input.imei || undefined,
  };
}

// List
mock
  .onGet(/\/subscribers$/)
  .withDelayInMs(250)
  .reply(() => [200, subscribers]);

// Detail
mock.onGet(/\/subscribers\/[^/]+$/).reply((config) => {
  const m = /\/subscribers\/([^/]+)$/.exec(config.url ?? '');
  const id = m ? decodeURIComponent(m[1]) : '';
  const sub = subscribers.find((s) => s.id === id);
  if (!sub) {
    return [
      404,
      {
        type: 'about:blank',
        title: 'Subscriber not found',
        status: 404,
        detail: `No subscriber with id "${id}"`,
      },
    ];
  }
  return [200, sub];
});

// Create
mock
  .onPost(/\/subscribers$/)
  .withDelayInMs(300)
  .reply((config) => {
    const input = safeParse<SubscriberInput>(config.data);
    const errors = validate(input, subscribers);
    if (errors) return validationProblem(errors);

    const id = `sub-${String(subscribers.length + 1).padStart(2, '0')}`;
    const created = applyInput({ id }, input!);
    subscribers.push(created);
    return [201, created];
  });

// Update
mock
  .onPut(/\/subscribers\/[^/]+$/)
  .withDelayInMs(300)
  .reply((config) => {
    const m = /\/subscribers\/([^/]+)$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const idx = subscribers.findIndex((s) => s.id === id);
    if (idx === -1) {
      return [
        404,
        {
          type: 'about:blank',
          title: 'Subscriber not found',
          status: 404,
          detail: `No subscriber with id "${id}"`,
        },
      ];
    }

    const input = safeParse<SubscriberInput>(config.data);
    const errors = validate(input, subscribers, id);
    if (errors) return validationProblem(errors);

    const updated = applyInput(subscribers[idx], input!);
    subscribers[idx] = updated;
    return [200, updated];
  });

// Delete
mock
  .onDelete(/\/subscribers\/[^/]+$/)
  .withDelayInMs(200)
  .reply((config) => {
    const m = /\/subscribers\/([^/]+)$/.exec(config.url ?? '');
    const id = m ? decodeURIComponent(m[1]) : '';
    const idx = subscribers.findIndex((s) => s.id === id);
    if (idx === -1) {
      return [
        404,
        {
          type: 'about:blank',
          title: 'Subscriber not found',
          status: 404,
          detail: `No subscriber with id "${id}"`,
        },
      ];
    }
    subscribers.splice(idx, 1);
    return [204];
  });

// TAC catalogue — immutable; served directly from the bundled JSON.
mock
  .onGet(/\/tac-catalog$/)
  .withDelayInMs(100)
  .reply(() => [200, tacCatalog]);

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
