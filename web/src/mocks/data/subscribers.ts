import type { Subscriber } from '../../api/resources/subscribers';
import { buildImei } from '../../api/imei';

/**
 * Five subscribers matching the Figma "Subscribers / Light" screen.
 * Names / MSISDN / ICCID come from the design; IMEIs are composed
 * from the TAC picked for each subscriber so they Luhn-validate.
 *
 * Two subscribers (Charlie Pool, Eve Testbench) are deliberately left
 * without a device binding so the empty-device "—" rendering is
 * exercised on the list screen.
 */
// Values must match entries in src/mocks/data/tacCatalog.json.
const TAC_APPLE_IPHONE_7 = '35656108';
const TAC_APPLE_IPHONE_13 = '35104463';
const TAC_GOOGLE_PIXEL_4 = '35293110';

// Fixed serials (rather than random) so fixtures are deterministic
// across reloads — handy for screenshot tests and debugging.
const imeiFor = (tac: string, serial: string): string =>
  buildImei(tac, serial)!;

export const subscriberFixtures: Subscriber[] = [
  {
    id: 'sub-01',
    name: 'Alice Test',
    msisdn: '27821234567',
    iccid: '89270100001234567890',
    tac: TAC_APPLE_IPHONE_13,
    imei: imeiFor(TAC_APPLE_IPHONE_13, '678943'),
  },
  {
    id: 'sub-02',
    name: 'Bob Demo',
    msisdn: '27837654321',
    iccid: '89270100009876543210',
    tac: TAC_APPLE_IPHONE_7,
    imei: imeiFor(TAC_APPLE_IPHONE_7, '678124'),
  },
  {
    id: 'sub-03',
    name: 'Charlie Pool',
    msisdn: '27831122334',
    iccid: '89270100005544332211',
    // No device bound — list shows "—".
  },
  {
    id: 'sub-04',
    name: 'Diana Load',
    msisdn: '27839988776',
    iccid: '89270100003344556677',
    tac: TAC_GOOGLE_PIXEL_4,
    imei: imeiFor(TAC_GOOGLE_PIXEL_4, '301234'),
  },
  {
    id: 'sub-05',
    name: 'Eve Testbench',
    msisdn: '27835566778',
    iccid: '89270100007788990011',
  },
];
