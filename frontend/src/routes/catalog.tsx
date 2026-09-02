import * as React from 'react';
import { useNavigate, useParams } from 'react-router';
import { PageHeader } from '@/components/common/page-header';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useT } from '@/lib/i18n';
import { DatasetCatalog } from '@/features/datasets/dataset-catalog';
import { DatasourceCatalog } from '@/features/catalog/datasource-catalog';
import { OntologyCatalog } from '@/features/catalog/ontology-catalog';
import { RepositoryCatalog } from '@/features/catalog/repository-catalog';
import { SecretCatalog } from '@/features/catalog/secret-catalog';
import { EnvironmentCatalog } from '@/features/catalog/environment-catalog';

const SECTIONS = [
  'datasets',
  'datasources',
  'ontologies',
  'repositories',
  'environments',
  'secrets',
] as const;
type Section = (typeof SECTIONS)[number];

/**
 * Global catalogue.
 *
 * Datasets, data sources and ontologies are shared across projects, so they
 * live at the top level rather than inside one project — the Unity Catalog
 * shape. Projects attach what they need from here, which is what the
 * `/projects/:id/{datasets,datasources,ontologies}` endpoints already model.
 *
 * Environments belong here for the same reason, and the code always said so:
 * `/api/v1/environments` is a platform endpoint with an optional project
 * filter. Listing them inside a project made a shared asset look owned by one,
 * and hid every environment a user could actually launch.
 */
export function CatalogPage() {
  const t = useT();
  const navigate = useNavigate();
  const { section, resourceId } = useParams<{ section?: string; resourceId?: string }>();

  const active: Section = SECTIONS.includes(section as Section) ? (section as Section) : 'datasets';
  const [selectedDatasetId, setSelectedDatasetId] = React.useState<string | null>(resourceId ?? null);

  // The selected dataset lives in the URL so the explorer state survives a
  // reload and can be shared as a link.
  React.useEffect(() => {
    setSelectedDatasetId(resourceId ?? null);
  }, [resourceId]);

  return (
    <div className="space-y-5">
      <PageHeader title={t('nav.catalog')} description={t('datasets.subtitle')} />

      <Tabs value={active} onValueChange={(value) => navigate(`/catalog/${value}`)}>
        <TabsList>
          <TabsTrigger value="datasets">{t('nav.datasets')}</TabsTrigger>
          <TabsTrigger value="datasources">{t('nav.datasources')}</TabsTrigger>
          <TabsTrigger value="ontologies">{t('nav.ontologies')}</TabsTrigger>
          <TabsTrigger value="repositories">{t('nav.repositories')}</TabsTrigger>
          <TabsTrigger value="environments">{t('nav.environments')}</TabsTrigger>
          <TabsTrigger value="secrets">{t('nav.secrets')}</TabsTrigger>
        </TabsList>

        <TabsContent value="datasets">
          <DatasetCatalog
            selectedId={selectedDatasetId}
            onSelect={(datasetId) => {
              setSelectedDatasetId(datasetId);
              navigate(datasetId ? `/catalog/datasets/${datasetId}` : '/catalog/datasets', {
                replace: true,
              });
            }}
          />
        </TabsContent>
        <TabsContent value="datasources">
          <DatasourceCatalog />
        </TabsContent>
        <TabsContent value="ontologies">
          <OntologyCatalog />
        </TabsContent>
        <TabsContent value="repositories">
          <RepositoryCatalog />
        </TabsContent>
        <TabsContent value="environments">
          <EnvironmentCatalog />
        </TabsContent>
        <TabsContent value="secrets">
          <SecretCatalog />
        </TabsContent>
      </Tabs>
    </div>
  );
}
