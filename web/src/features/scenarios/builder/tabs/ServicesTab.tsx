/**
 * Builder — Services tab.
 *
 * Surfaces the segmented `serviceModel` control with disabled segments
 * driven by `matrix(unitType, model)`. Renders one of three editor
 * shapes depending on the active `serviceModel`:
 *
 *   root         — one implicit Root service; RSU/USU on root.
 *                  No Identifiers / MSCC sections.
 *   single-mscc  — one MSCC with Identifiers, RSU, USU.
 *   multi-mscc   — SERVICES list with Add service + per-service editor.
 *
 * Mutations write the full `services` array back through
 * `setServices` so the store-history middleware records each commit
 * as a single snapshot.
 */
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Group,
  SegmentedControl,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core';
import {
  IconAlertCircle,
  IconInfoCircle,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';

import { useScenarioDraftStore } from '../../store/scenarioDraftStore';
import type { Service, ServiceModel } from '../../store/types';
import { matrix } from '../../store/validators';

const SERVICE_MODELS: ServiceModel[] = ['root', 'single-mscc', 'multi-mscc'];

const HINT_BY_MODEL: Record<ServiceModel, string> = {
  'root': 'No MSI · RSU/USU on root (no MSCC)',
  'single-mscc': 'MSI=0 · one MSCC, identifiers + RSU + USU',
  'multi-mscc': 'MSI=1 · one MSCC per selected service',
};

interface ServiceEditorProps {
  service: Service;
  showIdentifiers: boolean;
  onChange: (next: Service) => void;
  onRemove?: () => void;
}

function ServiceEditor({
  service,
  showIdentifiers,
  onChange,
  onRemove,
}: ServiceEditorProps) {
  return (
    <Stack gap="xs">
      <Group justify="space-between">
        <Group gap="xs">
          <Title order={6}>Service id: {service.id || '—'}</Title>
        </Group>
        {onRemove && (
          <ActionIcon
            variant="subtle"
            color="red"
            onClick={onRemove}
            aria-label="Remove service"
          >
            <IconTrash size={14} />
          </ActionIcon>
        )}
      </Group>
      {showIdentifiers && (
        <Group grow>
          <TextInput
            label="Service id"
            value={service.id}
            onChange={(e) =>
              onChange({ ...service, id: e.currentTarget.value })
            }
          />
          <TextInput
            label="Rating-Group var"
            value={service.ratingGroup ?? ''}
            onChange={(e) =>
              onChange({ ...service, ratingGroup: e.currentTarget.value })
            }
          />
          <TextInput
            label="Service-Identifier var"
            value={service.serviceIdentifier ?? ''}
            onChange={(e) =>
              onChange({
                ...service,
                serviceIdentifier: e.currentTarget.value,
              })
            }
          />
        </Group>
      )}
      <Group grow>
        <TextInput
          label="Requested service-units var (RSU)"
          value={service.requestedUnits}
          onChange={(e) =>
            onChange({ ...service, requestedUnits: e.currentTarget.value })
          }
        />
        <TextInput
          label="Used service-units var (USU)"
          value={service.usedUnits ?? ''}
          onChange={(e) =>
            onChange({ ...service, usedUnits: e.currentTarget.value })
          }
        />
      </Group>
    </Stack>
  );
}

export function ServicesTab() {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setServices = useScenarioDraftStore((s) => s.setServices);
  const setServiceModel = useScenarioDraftStore((s) => s.setServiceModel);

  if (!draft) return null;

  const { unitType, serviceModel, services } = draft;

  function handleSegment(value: string) {
    const next = value as ServiceModel;
    setServiceModel(next);
    // Re-shape the services array to match the new model.
    if (next === 'root') {
      setServices([
        {
          id: 'root',
          requestedUnits: services[0]?.requestedUnits ?? 'RSU_TOTAL',
          usedUnits: services[0]?.usedUnits ?? 'USU_TOTAL',
        },
      ]);
    } else if (next === 'single-mscc') {
      const first = services[0];
      setServices([
        {
          id: first?.id && first.id !== 'root' ? first.id : '100',
          ratingGroup: first?.ratingGroup ?? 'RATING_GROUP',
          serviceIdentifier: first?.serviceIdentifier,
          requestedUnits: first?.requestedUnits ?? 'RSU_TOTAL',
          usedUnits: first?.usedUnits ?? 'USU_TOTAL',
        },
      ]);
    } else {
      // multi-mscc — keep existing if it already had >=1 entries with
      // identifiers; otherwise seed two entries.
      if (services.length >= 1 && services[0].id !== 'root') {
        setServices(services);
      } else {
        setServices([
          {
            id: '100',
            ratingGroup: 'RG100_RATING_GROUP',
            requestedUnits: 'RG100_RSU_TOTAL',
            usedUnits: 'RG100_USU_TOTAL',
          },
          {
            id: '200',
            ratingGroup: 'RG200_RATING_GROUP',
            requestedUnits: 'RG200_RSU_TOTAL',
            usedUnits: 'RG200_USU_TOTAL',
          },
        ]);
      }
    }
  }

  const segData = SERVICE_MODELS.map((m) => {
    const cell = matrix(unitType, m);
    return {
      value: m,
      label: m === 'root' ? 'Root' : m === 'single-mscc' ? 'Single MSCC' : 'Multi MSCC',
      disabled: !cell.allowed,
    };
  });

  const activeCell = matrix(unitType, serviceModel);

  return (
    <Stack gap="md">
      <Card withBorder padding="md">
        <Stack gap="xs">
          <Group justify="space-between">
            <Title order={5}>Service model</Title>
            <Text size="sm" c="dimmed">
              {HINT_BY_MODEL[serviceModel]}
            </Text>
          </Group>
          <SegmentedControl
            value={serviceModel}
            onChange={handleSegment}
            data={segData}
            fullWidth
            data-testid="services-segmented"
          />
          {!activeCell.allowed && (
            <Alert
              color="red"
              icon={<IconAlertCircle size={16} />}
              data-testid="services-matrix-error"
            >
              {activeCell.hint}
            </Alert>
          )}
          {SERVICE_MODELS.filter((m) => !matrix(unitType, m).allowed).map(
            (m) => (
              <Group gap={6} key={m}>
                <IconInfoCircle size={14} />
                <Text size="xs" c="dimmed">
                  {m}: {matrix(unitType, m).hint}
                </Text>
              </Group>
            ),
          )}
        </Stack>
      </Card>

      {serviceModel === 'root' && services[0] && (
        <Card withBorder padding="md">
          <Stack gap="xs">
            <Group gap="xs">
              <Title order={5}>Root service</Title>
              <Badge variant="outline">implicit</Badge>
            </Group>
            <Text size="xs" c="dimmed">
              No Identifiers · No MSCC · RSU/USU on the CCR root.
            </Text>
            <ServiceEditor
              service={services[0]}
              showIdentifiers={false}
              onChange={(next) => setServices([next])}
            />
          </Stack>
        </Card>
      )}

      {serviceModel === 'single-mscc' && services[0] && (
        <Card withBorder padding="md">
          <Stack gap="xs">
            <Title order={5}>MSCC</Title>
            <ServiceEditor
              service={services[0]}
              showIdentifiers
              onChange={(next) => setServices([next])}
            />
          </Stack>
        </Card>
      )}

      {serviceModel === 'multi-mscc' && (
        <Card withBorder padding="md">
          <Stack gap="md">
            <Group justify="space-between">
              <Title order={5}>Services</Title>
              <Button
                variant="default"
                leftSection={<IconPlus size={14} />}
                onClick={() =>
                  setServices([
                    ...services,
                    {
                      id: String((services.length + 1) * 100),
                      ratingGroup: `RG${(services.length + 1) * 100}_RATING_GROUP`,
                      requestedUnits: `RG${(services.length + 1) * 100}_RSU_TOTAL`,
                    },
                  ])
                }
                data-testid="services-add"
              >
                Add service
              </Button>
            </Group>
            {services.length === 0 ? (
              <Text c="dimmed">No services yet. Click Add service.</Text>
            ) : (
              services.map((svc, i) => (
                <Card key={i} withBorder padding="sm">
                  <ServiceEditor
                    service={svc}
                    showIdentifiers
                    onChange={(next) =>
                      setServices(services.map((s, j) => (j === i ? next : s)))
                    }
                    onRemove={
                      services.length > 1
                        ? () => setServices(services.filter((_, j) => j !== i))
                        : undefined
                    }
                  />
                </Card>
              ))
            )}
          </Stack>
        </Card>
      )}
    </Stack>
  );
}
