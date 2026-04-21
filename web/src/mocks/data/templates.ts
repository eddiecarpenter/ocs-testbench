import type { TemplateSummary } from '../../api/resources/templates';

export const templateFixtures: TemplateSummary[] = [
  {
    id: 'tpl-initial-ccr',
    name: 'Initial-CCR',
    description: 'Canonical CCR-Initial for a data session',
    avpCount: 18,
    updatedAt: '2026-04-10T11:00:00Z',
  },
  {
    id: 'tpl-update-ccr',
    name: 'Update-CCR',
    description: 'CCR-Update with Used-Service-Unit reporting',
    avpCount: 22,
    updatedAt: '2026-04-10T11:05:00Z',
  },
  {
    id: 'tpl-terminate-ccr',
    name: 'Terminate-CCR',
    description: 'Session-terminate with final USU',
    avpCount: 20,
    updatedAt: '2026-04-10T11:10:00Z',
  },
  {
    id: 'tpl-voice-initial',
    name: 'Voice-Initial',
    description: 'Voice-call CCR-Initial (charging for duration)',
    avpCount: 24,
    updatedAt: '2026-04-12T09:30:00Z',
  },
  {
    id: 'tpl-voice-update',
    name: 'Voice-Update',
    description: 'Voice-call CCR-Update',
    avpCount: 22,
    updatedAt: '2026-04-12T09:32:00Z',
  },
  {
    id: 'tpl-sms-event',
    name: 'SMS-Event',
    description: 'Event-based charging for a single SMS',
    avpCount: 16,
    updatedAt: '2026-04-15T08:10:00Z',
  },
  {
    id: 'tpl-fui-compliance',
    name: 'FUI-Compliance',
    description: 'Final-Unit-Indication behaviour verification',
    avpCount: 26,
    updatedAt: '2026-04-16T14:00:00Z',
  },
  {
    id: 'tpl-validity-time',
    name: 'Validity-Time',
    description: 'Re-auth before Validity-Time expiry',
    avpCount: 21,
    updatedAt: '2026-04-18T16:00:00Z',
  },
];
