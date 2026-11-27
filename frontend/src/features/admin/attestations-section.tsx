import { Download, GraduationCap } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { SectionHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useAdminAttestations } from '@/lib/api/queries';
import { platformApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDate } from '@/lib/format';
import { tonAttestationStatut } from '@/features/account/attestations';
import type { Attestation } from '@/lib/api/types';

/**
 * Qui a produit une attestation de formation.
 *
 * Une liste, pas une porte. Rien ici ne refuse quoi que ce soit a personne :
 * elle existe pour qu'on puisse voir qui est couvert **avant** de decider
 * d'exiger quoi que ce soit. L'ordre inverse - exiger d'abord - enferme tout
 * le monde dehors le premier jour.
 *
 * Le statut vient de la plateforme et pas d'un calcul refait ici : « sans date
 * de fin » n'est pas « valable », et deux ecrans qui en jugeraient chacun de
 * leur cote finiraient par ne plus dire la meme chose.
 */
export function AttestationsAdminSection() {
  const t = useT();
  const { locale } = useI18n();
  const attestations = useAdminAttestations();

  const columns: Column<Attestation>[] = [
    {
      id: 'person',
      header: t('rbac.subject'),
      sortValue: (item) => item.userName || item.userId,
      searchValue: (item) => `${item.userName ?? ''} ${item.userId} ${item.subject ?? ''}`,
      cell: (item) => <span className="font-medium">{item.userName || item.userId}</span>,
    },
    {
      id: 'subject',
      header: t('account.attestationSubject'),
      cell: (item) => <span>{item.subject || item.fileName}</span>,
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (item) => item.status,
      cell: (item) => {
        const status = tonAttestationStatut(item);
        return <Badge tone={status.tone}>{t(status.key as never)}</Badge>;
      },
    },
    {
      id: 'validUntil',
      header: t('account.attestationValidUntil'),
      sortValue: (item) => item.validUntil ?? null,
      cell: (item) => (
        <span className="text-xs text-muted-foreground">
          {item.validUntil ? formatDate(item.validUntil, locale) : '—'}
        </span>
      ),
    },
    {
      id: 'uploadedAt',
      header: t('common.createdAt'),
      sortValue: (item) => item.uploadedAt,
      cell: (item) => (
        <span className="text-xs text-muted-foreground">{formatDate(item.uploadedAt, locale)}</span>
      ),
    },
    {
      id: 'document',
      header: '',
      cell: (item) => (
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t('common.download')}
          onClick={() => void platformApi.downloadAttestation(item.id, item.fileName)}
        >
          <Download aria-hidden />
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('account.attestations')} description={t('admin.attestationsHint')} />
      <Card>
        <DataTable
          data={attestations.data}
          columns={columns}
          rowKey={(item) => item.id}
          isLoading={attestations.isLoading}
          isError={attestations.isError}
          error={attestations.error}
          onRetry={() => void attestations.refetch()}
          defaultSort={{ columnId: 'uploadedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={GraduationCap}
              title={t('admin.attestationsEmpty')}
              description={t('admin.attestationsEmptyHint')}
            />
          }
        />
      </Card>
    </div>
  );
}
