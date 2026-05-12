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
  Select,
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

import { provisionVariables } from '../provisionVariables';
import { useScenarioDraftStore } from '../scenarioDraftStore';
import {
  buildVariableOptions,
  type VariableOptionGroup,
} from '../selectors';
import type { Service, ServiceModel } from '../types';
import { deriveUnitType, matrix } from '../validators';

const SERVICE_MODELS: ServiceModel[] = ['root', 'single-mscc', 'multi-mscc'];

const HINT_BY_MODEL: Record<ServiceModel, string> = {
  'root': 'No MSI · RSU/USU on root (no MSCC)',
  'single-mscc': 'MSI=0 · one MSCC, identifiers + RSU + USU',
  'multi-mscc': 'MSI=1 · one MSCC per selected service',
};

interface ServiceEditorProps {
  service: Service;
  showIdentifiers: boolean;
  /** Show only the Service-Identifier field (root model: no Service-id or Rating-Group). */
  showServiceIdentifier?: boolean;
  variableOptions: VariableOptionGroup[];
  hasAnyVariable: boolean;
  onChange: (next: Service) => void;
  onRemove?: () => void;
}

function ServiceEditor({
  service,
  showIdentifiers,
  showServiceIdentifier,
  variableOptions,
  hasAnyVariable,
  onChange,
  onRemove,
}: ServiceEditorProps) {
  // Empty-state placeholder mirrors the `{{NAME}}` wrapping used for
  // selected values, so an unset variable slot is instantly
  // distinguishable from a free-text field.
  const variablePlaceholder = hasAnyVariable
    ? '{{ ... }}'
    : 'No variables available';

  return (
    <Stack gap="xs">
      {onRemove && (
        <Group justify="flex-end">
          <ActionIcon
            variant="subtle"
            color="red"
            onClick={onRemove}
            aria-label="Remove service"
          >
            <IconTrash size={14} />
          </ActionIcon>
        </Group>
      )}
      {showIdentifiers && (
        <Group grow>
          <TextInput
            label="Service id"
            value={service.id}
            onChange={(e) =>
              onChange({ ...service, id: e.currentTarget.value })
            }
          />
          <Select
            label="Rating-Group"
            placeholder={variablePlaceholder}
            data={variableOptions}
            value={service.ratingGroup ?? null}
            onChange={(v) =>
              onChange({ ...service, ratingGroup: v ?? '' })
            }
            clearable
            disabled={!hasAnyVariable}
          />
          <Select
            label="Service-Identifier"
            placeholder={variablePlaceholder}
            data={variableOptions}
            value={service.serviceIdentifier ?? null}
            onChange={(v) =>
              onChange({
                ...service,
                serviceIdentifier: v ?? '',
              })
            }
            clearable
            disabled={!hasAnyVariable}
          />
        </Group>
      )}
      {!showIdentifiers && showServiceIdentifier && (
        <TextInput
          label="Service-Identifier"
          placeholder="e.g. 1"
          value={service.serviceIdentifier ?? ''}
          onChange={(e) =>
            onChange({ ...service, serviceIdentifier: e.currentTarget.value })
          }
          style={{ maxWidth: 200 }}
        />
      )}
      <Group grow>
        <Select
          label="Requested service-units (RSU)"
          placeholder={variablePlaceholder}
          data={variableOptions}
          value={service.requestedUnits || null}
          onChange={(v) =>
            onChange({ ...service, requestedUnits: v ?? '' })
          }
          clearable
          disabled={!hasAnyVariable}
        />
        <Select
          label="Used service-units (USU)"
          placeholder={variablePlaceholder}
          data={variableOptions}
          value={service.usedUnits ?? null}
          onChange={(v) =>
            onChange({ ...service, usedUnits: v ?? '' })
          }
          clearable
          disabled={!hasAnyVariable}
        />
      </Group>
    </Stack>
  );
}

export function ServicesTab() {
  const draft = useScenarioDraftStore((s) => s.draft);
  const setServices = useScenarioDraftStore((s) => s.setServices);
  const setVariables = useScenarioDraftStore((s) => s.setVariables);
  const setServiceModel = useScenarioDraftStore((s) => s.setServiceModel);

  if (!draft) return null;

  const { serviceType, serviceModel, services } = draft;
  const unitType = deriveUnitType(serviceType);
  const { options: variableOptions, hasAny: hasAnyVariable } =
    buildVariableOptions(draft.variables, { services, serviceModel });

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
        // Seed two services using position-based variable names (RG1_, RG2_)
        // to match provisionVariables and the engine's CCA indexing.
        const seeded = [
          {
            id: '100',
            ratingGroup: 'RG1_RATING_GROUP',
            requestedUnits: 'RG1_UNITS_REQ',
            usedUnits: 'RG1_UNITS_USED',
          },
          {
            id: '200',
            ratingGroup: 'RG2_RATING_GROUP',
            requestedUnits: 'RG2_UNITS_REQ',
            usedUnits: 'RG2_UNITS_USED',
          },
        ];
        setServices(seeded);
        if (draft) {
          setVariables(provisionVariables({ ...draft, services: seeded }));
        }
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
              No MSCC · RSU/USU on the CCR root. Service-Identifier is optional.
            </Text>
            <ServiceEditor
              service={services[0]}
              showIdentifiers={false}
              showServiceIdentifier
              variableOptions={variableOptions}
              hasAnyVariable={hasAnyVariable}
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
              variableOptions={variableOptions}
              hasAnyVariable={hasAnyVariable}
              onChange={(next) => setServices([next])}
            />
          </Stack>
        </Card>
      )}

      {serviceModel === 'multi-mscc' && (
        <Card withBorder padding="md">
          <Stack gap="md">
            <Group justify="space-between">
              <Title order={5}>Charging services</Title>
              <Button
                variant="default"
                leftSection={<IconPlus size={14} />}
                onClick={() => {
                  // pos is 1-based position; id is the Diameter service identifier.
                  const pos = services.length + 1;
                  const id = String(pos * 100);
                  const newSvc: Service = {
                    id,
                    ratingGroup: `RG${pos}_RATING_GROUP`,
                    requestedUnits: `RG${pos}_UNITS_REQ`,
                    usedUnits: `RG${pos}_UNITS_USED`,
                  };
                  const nextServices = [...services, newSvc];
                  setServices(nextServices);
                  if (draft) {
                    setVariables(provisionVariables({ ...draft, services: nextServices }));
                  }
                }}
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
                    variableOptions={variableOptions}
                    hasAnyVariable={hasAnyVariable}
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
