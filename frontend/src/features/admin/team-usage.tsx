import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Download } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { SectionHeader } from '@/components/common/page-header';
import { Skeleton } from '@/components/ui/skeleton';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { adminApi } from '@/lib/api/endpoints';
import { useT } from '@/lib/i18n';

/**
 * Ce que chaque equipe a consomme.
 *
 * La consommation est echantillonnee par projet et jamais par personne : le
 * chiffre d'une equipe est donc une attribution, pas une mesure, et la seule
 * facon honnete de le publier est de le dire.
 *
 * Un projet atteint par deux equipes a ete paye une fois et travaille par les
 * deux. Le couper en deux inventerait une precision que personne n'a mesuree,
 * et les moitiés seraient fausses des qu'une equipe fait tout le travail. Ce
 * tableau credite donc chacune en entier, et publie le recouvrement : qui
 * compare deux equipes a une comparaison juste, qui additionne est prevenu.
 *
 * Le nombre de membres est a cote des heures parce que le meme total reparti
 * sur trente personnes ne se lit pas comme celui de deux.
 */

const JOURS = 30;

function fenetre() {
  const to = new Date();
  const from = new Date(to.getTime() - JOURS * 24 * 3600 * 1000);
  return { from: from.toISOString(), to: to.toISOString() };
}

export function TeamUsageSection() {
  const t = useT();
  const { from, to } = React.useMemo(fenetre, []);
  const usage = useQuery({
    queryKey: ['admin', 'team-usage', from, to],
    queryFn: () => adminApi.teamUsage(from, to),
  });

  const report = usage.data;
  const rows = report?.rows ?? [];
  const nombre = (value: number) =>
    value.toLocaleString(undefined, { maximumFractionDigits: 1 });

  type Row = (typeof rows)[number];
  const columns: Column<Row>[] = [
    {
      id: 'team',
      header: t('admin.teams'),
      sortValue: (row) => row.teamName.toLowerCase(),
      cell: (row) => (
        <>
          {row.teamName}
          {/* Le recouvrement est vrai de certaines lignes et pas d'autres : il
              est donc porte par la ligne. */}
          {row.sharedProjects > 0 ? (
            <span className="ml-2 text-xs text-muted-foreground">
              {t('admin.teamUsageShared', { count: row.sharedProjects })}
            </span>
          ) : null}
        </>
      ),
    },
    { id: 'members', header: t('admin.teamUsageMembers'), align: 'right', sortValue: (row) => row.members, cell: (row) => <span className="tabular-nums">{row.members}</span> },
    { id: 'projects', header: t('admin.teamUsageProjects'), align: 'right', sortValue: (row) => row.projects, cell: (row) => <span className="tabular-nums">{row.projects}</span> },
    { id: 'vcpuHours', header: t('admin.teamUsageVcpuHours'), align: 'right', sortValue: (row) => row.vcpuHours, cell: (row) => <span className="tabular-nums">{nombre(row.vcpuHours)}</span> },
    { id: 'peak', header: t('admin.teamUsagePeak'), align: 'right', sortValue: (row) => row.peakVcpu, cell: (row) => <span className="tabular-nums">{nombre(row.peakVcpu)}</span> },
  ];

  return (
    <section className="space-y-3">
      <SectionHeader
        title={t('admin.teamUsage')}
        description={t('admin.teamUsageHint')}
        actions={
          <Button variant="secondary" onClick={() => adminApi.downloadTeamUsage(from, to)}>
            <Download className="size-4" />
            {t('common.export')}
          </Button>
        }
      />

      {usage.isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : !report ? (
        <p className="text-sm text-muted-foreground">{t('admin.teamUsageEmpty')}</p>
      ) : (
        <>
          <Card>
            <CardContent className="grid gap-3 p-3 sm:grid-cols-3">
              <Chiffre
                label={t('admin.teamUsagePlatform')}
                value={nombre(report.platformVcpuHours)}
                hint={t('admin.teamUsagePlatformHint')}
              />
              <Chiffre
                label={t('admin.teamUsageAttributed')}
                value={nombre(report.attributedVcpuHours)}
                /* Le depassement n'est pas une anomalie : c'est le
                   recouvrement, et il doit se lire sans calcul. */
                hint={
                  report.attributedVcpuHours > report.platformVcpuHours
                    ? t('admin.teamUsageOverlap', {
                        excess: nombre(report.attributedVcpuHours - report.platformVcpuHours),
                      })
                    : t('admin.teamUsageAttributedHint')
                }
              />
              <Chiffre
                label={t('admin.teamUsageUnattributed')}
                value={nombre(report.unattributedVcpuHours)}
                hint={t('admin.teamUsageUnattributedHint')}
              />
            </CardContent>
          </Card>
          <DataTable
              data={rows}
              columns={columns}
              rowKey={(row) => row.teamId}
              defaultSort={{ columnId: 'vcpuHours', direction: 'desc' }}
              emptyState={<EmptyState title={t('admin.teamUsageNoTeams')} />}
            />
        </>
      )}
    </section>
  );
}

function Chiffre({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <div>
      <div className="text-xl font-semibold tabular-nums">{value}</div>
      <div className="text-sm">{label}</div>
      <div className="text-xs text-muted-foreground">{hint}</div>
    </div>
  );
}
