import type { Subscriber } from '../../api/resources/subscribers';

const TOTAL = 142;

/** Generate a deterministic set of 142 subscribers. */
export const subscriberFixtures: Subscriber[] = Array.from(
  { length: TOTAL },
  (_, i): Subscriber => {
    const n = i + 1;
    const msisdn = `64210${String(100000 + n).padStart(6, '0')}`;
    const imsi = `530017${String(1000000 + n).padStart(8, '0')}`;
    return {
      id: `sub-${String(n).padStart(4, '0')}`,
      msisdn,
      imsi,
    };
  },
);
