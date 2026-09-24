import { useNavigate, useParams } from 'react-router';
import {
  Boxes,
  Cpu,
  MemoryStick,
  ShieldCheck,
  Users,
} from 'lucide-react';
import { PageHeader, SectionHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { EmptyState } from '@/components/common/states';
import {
  Card,
} from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  useAdminOverview,
  useProjects,
} from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatCpu, formatNumber } from '@/lib/format';
import { isEnterprise } from '@/lib/config';
import { useExtensions } from '@/lib/extensions';
import { ExtensionSlot } from '@/components/common/extension-slot';
import { useAuth } from '@/lib/auth';
import { PlatformHealthPanel } from '@/features/admin/platform-health';
import { SoftwareInventorySection } from '@/features/admin/software-inventory';
import { HardwareTiersSection } from '@/features/admin/hardware-tiers';
import { PeopleSection } from '@/features/admin/people';
import { SmtpSettingsSection } from '@/features/admin/smtp-settings';
import { AgentGovernanceSection } from '@/features/admin/agent-governance';
import { PlatformActivitySection } from '@/features/admin/platform-activity';
import { PlatformUsageSection } from '@/features/admin/platform-usage';
import { PlatformSettingsSection } from '@/features/admin/platform-settings';
import { ActivitySection } from '@/features/admin/activity-section';
import { AuditSection } from '@/features/admin/audit-section';
import { BackupsSection } from '@/features/admin/backups-section';
import { DataGovernanceSection } from '@/features/admin/data-governance-section';
import { NetworkSection } from '@/features/admin/network-section';
import { RbacSection } from '@/features/admin/rbac-section';
import { StorageSection } from '@/features/admin/storage-section';

const SECTIONS = [
  'overview',
  'identity',
  'activity',
  'data',
  'network',
  'rbac',
  'agents',
  'storage',
  'inventory',
  'backups',
  'audit',
  'settings',
] as const;

/* -- overview -------------------------------------------------------------- */

function OverviewSection() {
  const t = useT();
  const { locale } = useI18n();
  const overview = useAdminOverview();
  const projects = useProjects();

  return (
    <div className="space-y-4">
      <SectionHeader title={t('admin.overview')} description={t('admin.subtitle')} />
      <PlatformHealthPanel enabled />
      <StatGrid>
        <Stat
          icon={Users}
          label={t('home.users')}
          loading={overview.isLoading}
          value={formatNumber(overview.data?.counts.users, locale)}
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={overview.isLoading || projects.isLoading}
          value={formatNumber(overview.data?.counts.projects ?? projects.data?.length, locale)}
        />
        <Stat
          icon={Cpu}
          label={t('activity.cpuReserved')}
          loading={overview.isLoading}
          value={formatCpu(overview.data?.workloadMetrics.cpuRequestMillicores, locale)}
        />
        <Stat
          icon={MemoryStick}
          label={t('activity.memoryReserved')}
          loading={overview.isLoading}
          value={formatBytes(overview.data?.workloadMetrics.memoryRequestBytes, locale)}
        />
      </StatGrid>
    </div>
  );
}

/* -- identity -------------------------------------------------------------- */

function IdentitySection() {
  // Reprise en un seul ecran : les personnes, leurs organisations et leurs
  // equipes etaient trois sections ecrites a des moments differents, sans
  // que rien ne dessine le lien. Voir features/admin/people.tsx.
  return <PeopleSection />;
}








/* -- page ------------------------------------------------------------------ */

export function AdminPage() {
  const t = useT();
  const { locale } = useI18n();
  const navigate = useNavigate();
  const { section } = useParams<{ section?: string }>();
  const { isAdmin } = useAuth();

  // Enterprise modules add their own admin sections through the declared
  // extension point rather than by patching this file's markup.
  const extensions = useExtensions('admin.section');

  const known = new Set<string>([...SECTIONS, ...extensions.map((module) => module.id)]);
  const active: string = known.has(section ?? '') ? (section as string) : 'overview';

  if (!isAdmin) {
    return (
      <Card className="mx-auto max-w-lg">
        <EmptyState
          icon={ShieldCheck}
          title={t('common.accessDenied')}
          description={t('errors.noOrganizationHint')}
        />
      </Card>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeader title={t('admin.title')} description={t('admin.subtitle')} />

      <Tabs value={active} onValueChange={(value) => navigate(`/admin/${value}`)}>
        {/* The sections live in the sidebar, grouped: twelve of them on one
            horizontal bar ran off the side of the screen and put "Audit" next
            to "Network" as though they were the same kind of thing. What stays
            here is the overview and whatever an Enterprise module adds, which
            is a short list by construction. */}
        <TabsList>
          <TabsTrigger value="overview">{t('admin.overview')}</TabsTrigger>
          {extensions.map((module) => (
            <TabsTrigger key={module.id} value={module.id}>
              {module.title[locale]}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="overview">
          <OverviewSection />
        </TabsContent>
        <TabsContent value="identity">
          <IdentitySection />
        </TabsContent>
        <TabsContent value="activity">
          <ActivitySection />
        </TabsContent>
        <TabsContent value="network">
          <NetworkSection />
        </TabsContent>
        <TabsContent value="rbac">
          <RbacSection />
        </TabsContent>
        <TabsContent value="storage">
          <StorageSection />
        </TabsContent>
        <TabsContent value="audit">
          <AuditSection />
        </TabsContent>
        <TabsContent value="inventory">
          <SoftwareInventorySection />
        </TabsContent>
        <TabsContent value="usage">
          <PlatformActivitySection />
          <PlatformUsageSection />
        </TabsContent>

        <TabsContent value="settings">
          <div className="space-y-8">
            <PlatformSettingsSection />
            <SmtpSettingsSection />
            {/* Machine sizes sit with the platform settings rather than in a
                tab of their own: an administrator arrives here to say how the
                installation behaves, and the sizes it offers are part of that. */}
            <HardwareTiersSection />
          </div>
        </TabsContent>
        {isEnterprise() ? (
          <>
            <TabsContent value="data">
              <DataGovernanceSection />
            </TabsContent>
            <TabsContent value="backups">
              <BackupsSection />
            </TabsContent>
            <TabsContent value="agents">
              <AgentGovernanceSection />
            </TabsContent>
          </>
        ) : null}
        {extensions.map((module) => (
          <TabsContent key={module.id} value={module.id}>
            <ExtensionSlot module={module} />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}
