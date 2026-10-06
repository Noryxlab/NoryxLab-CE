import * as React from 'react';
import { useNavigate, useParams } from 'react-router';
import { PageHeader } from '@/components/common/page-header';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useT, type TranslationKey } from '@/lib/i18n';
import { DatasetCatalog } from '@/features/datasets/dataset-catalog';
import { DatasourceCatalog } from '@/features/catalog/datasource-catalog';
import { OntologyCatalog } from '@/features/catalog/ontology-catalog';
import { ExtractCatalog } from '@/features/catalog/extract-catalog';
import { RepositoryCatalog } from '@/features/catalog/repository-catalog';
import { EnvironmentCatalog } from '@/features/catalog/environment-catalog';

// Une seule liste : les onglets affiches et les segments d'URL acceptes.
//
// Il y en avait deux, et elles ont divergé. Cette liste valide le segment
// d'URL, et un onglet absent retombe sur `datasets` : « extracts » avait son
// onglet et son contenu mais pas sa section, donc le clic naviguait vers
// /catalog/extracts puis revenait aussitot aux jeux de donnees. Un onglet qui
// refuse de s'ouvrir sans rien dire, et rien dans le code ne signalait le
// manque - les deux listes etaient justes separement.
//
// Les onglets sont donc derives d'ici. Ajouter une section, c'est ajouter une
// ligne, et le compilateur reclame le contenu correspondant.
const SECTIONS = [
  { id: 'datasets', label: 'nav.datasets' },
  { id: 'datasources', label: 'nav.datasources' },
  { id: 'ontologies', label: 'nav.ontologies' },
  { id: 'extracts', label: 'ontologies.extracts' },
  { id: 'repositories', label: 'nav.repositories' },
  { id: 'environments', label: 'nav.environments' },
] as const satisfies readonly { id: string; label: TranslationKey }[];
type Section = (typeof SECTIONS)[number]['id'];
const SECTION_IDS: readonly string[] = SECTIONS.map((entry) => entry.id);

/**
 * Global catalogue.
 *
 * Datasets, data sources and ontologies are shared across projects, so they
 * live at the top level rather than inside one project — the Unity Catalog
 * shape. Projects attach what they need from here, which is what the
 * `/projects/:id/{datasets,datasources,ontologies}` endpoints already model.
 *
 * Secrets are deliberately *not* here. They are per person, not shared, and
 * listing them beside datasets suggested otherwise; they live on the account
 * page, and /catalog/secrets redirects there so existing links keep working.
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

  const active: Section = SECTION_IDS.includes(section ?? '') ? (section as Section) : 'datasets';
  const [selectedDatasetId, setSelectedDatasetId] = React.useState<string | null>(resourceId ?? null);

  // The selected dataset lives in the URL so the explorer state survives a
  // reload and can be shared as a link.
  React.useEffect(() => {
    setSelectedDatasetId(resourceId ?? null);
  }, [resourceId]);

  return (
    <div className="space-y-5">
      <PageHeader title={t('nav.catalog')} description={t('catalog.subtitle')} />

      <Tabs value={active} onValueChange={(value) => navigate(`/catalog/${value}`)}>
        <TabsList>
          {SECTIONS.map((entry) => (
            <TabsTrigger key={entry.id} value={entry.id}>
              {t(entry.label)}
            </TabsTrigger>
          ))}
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
        <TabsContent value="extracts">
          <ExtractCatalog />
        </TabsContent>
        <TabsContent value="repositories">
          <RepositoryCatalog />
        </TabsContent>
        <TabsContent value="environments">
          <EnvironmentCatalog />
        </TabsContent>
      </Tabs>
    </div>
  );
}
