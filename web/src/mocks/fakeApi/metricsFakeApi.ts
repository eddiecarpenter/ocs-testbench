import type { ResponseTimeSeries } from '../../api/resources/metrics';
import {
  buildResponseTimeSeries,
  parseIsoDurationMs,
} from '../data/metrics';
import { mock } from '../MockAdapter';

mock
  .onGet(/\/metrics\/response-time(\?|$)/)
  .withDelayInMs(300)
  .reply((config): [number, ResponseTimeSeries] => {
    const windowIso = (config.params?.window as string | undefined) ?? 'PT1H';
    const windowMs = parseIsoDurationMs(windowIso);
    return [200, buildResponseTimeSeries(windowIso, windowMs)];
  });
