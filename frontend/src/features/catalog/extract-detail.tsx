import * as React from 'react';
import { AccessPanel, type AccessEntry } from '@/components/common/access-panel';
import { useMutation, useQuery } from '@tanstack/react-query';
import { ontologiesApi } from '@/lib/api/endpoints';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrapper,
} from '@/components/ui/table';
import { OwnerTransfer } from '@/components/common/owner';
import { useToast } from '@/components/ui/toast';
import { qk, useExtractAccess, useInvalidate } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber } from '@/lib/format';
import type { Extract } from '@/lib/api/types';

/**
 * Ce qu'un extrait contient, et si c'est encore la.
 *
 * La liste gelee est tout l'objet - c'est elle qui fait qu'un n se rejoue dans
 * deux ans - et aucun ecran ne la montrait. La route existait depuis le
 * premier jour et n'etait appelee nulle part : on lisait "4 019 fichiers" et
 * on croyait sur parole.
 */
export function ExtractDetail({ extract }: { extract: Extract }) {
  const t = useT();
  const { locale } = useI18n();
  const files = useQuery({
    queryKey: ['extracts', extract.id, 'members'],
    queryFn: () => ontologiesApi.extractMembers(extract.id),
  });

  const tout =
    extract.subjects.length === 0 &&
    extract.modalities.length === 0 &&
    extract.visits.length === 0;
  const disposition = (extract.layout ?? ['subject', 'visit', 'modality'])
    .map((level) => t(`ontologies.level_${level}` as 'ontologies.level_subject'))
    .join(' › ');

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{extract.name}</CardTitle>
          <CardDescription>{t('ontologies.extractDetailHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <Badge tone="outline">{disposition}</Badge>
          <span>
            {formatNumber(extract.objectCount, locale)} · {formatBytes(extract.totalBytes, locale)}
          </span>
          {tout ? (
            <Badge tone="brand">{t('ontologies.extractWholeBadge')}</Badge>
          ) : (
            <>
              {extract.modalities.length > 0 ? (
                <span>{extract.modalities.join(', ')}</span>
              ) : null}
              {extract.subjects.length > 0 ? (
                <span>{t('ontologies.extractSubjectCount', {
                  count: formatNumber(extract.subjects.length, locale),
                })}</span>
              ) : null}
            </>
          )}
        </div>

        <ExtractIntegrityLine extract={extract} />

        {files.data ? (
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">
              {t('ontologies.extractFilesShown', {
                shown: formatNumber(files.data.shown, locale),
                total: formatNumber(files.data.total, locale),
              })}
            </p>
            <TableWrapper className="max-h-96 overflow-auto rounded-md border border-border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('ontologies.object')}</TableHead>
                    <TableHead>{t('ontologies.patternSubject')}</TableHead>
                    <TableHead>{t('ontologies.patternVisit')}</TableHead>
                    <TableHead>{t('ontologies.patternModality')}</TableHead>
                    <TableHead className="text-right">{t('common.size')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {files.data.members.map((member) => (
                    <TableRow key={member.path}>
                      <TableCell className="max-w-[24rem] truncate font-mono text-xs">
                        {member.path}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{member.subjectId || '—'}</TableCell>
                      <TableCell className="font-mono text-xs">{member.visit || '—'}</TableCell>
                      <TableCell className="font-mono text-xs">{member.modality || '—'}</TableCell>
                      <TableCell className="text-right text-xs tabular-nums">
                        {formatBytes(member.sizeBytes, locale)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableWrapper>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

/* Geler protege le n, pas les octets.
 *
 *  Une liste gelee qui pointe vers des objets supprimes monte en liens casses,
 *  qu'un carnet decouvre un open() a la fois. La mesure se demande plutot
 *  qu'elle ne se fait a l'ouverture : elle coute des allers-retours vers le
 *  stockage, et la reponse est presque toujours "oui". */
function ExtractIntegrityLine({ extract }: { extract: Extract }) {
  const t = useT();
  const { locale } = useI18n();
  const [asked, setAsked] = React.useState(false);
  const integrity = useQuery({
    queryKey: ['extracts', extract.id, 'integrity'],
    queryFn: () => ontologiesApi.extractIntegrity(extract.id),
    enabled: asked,
  });

  if (!asked) {
    return (
      <div className="flex items-center gap-2">
        <Button variant="secondary" size="sm" onClick={() => setAsked(true)}>
          {t('ontologies.extractCheckFiles')}
        </Button>
        <span className="text-xs text-muted-foreground">{t('ontologies.extractCheckHint')}</span>
      </div>
    );
  }
  if (integrity.isLoading) {
    return <p className="text-xs text-muted-foreground">{t('ontologies.extractChecking')}</p>;
  }
  if (integrity.isError) {
    return (
      <p className="text-xs text-danger">
        {t('ontologies.extractCheckFailed')}
      </p>
    );
  }
  const data = integrity.data;
  if (!data) return null;

  return (
    <div className="space-y-1">
      <p className={data.complete ? 'text-xs text-muted-foreground' : 'text-xs text-warning-foreground'}>
        {data.complete
          ? t('ontologies.extractCheckAllPresent', {
              checked: formatNumber(data.checked, locale),
              total: formatNumber(data.total, locale),
            })
          : t('ontologies.extractCheckMissing', {
              missing: formatNumber(data.missing, locale),
              checked: formatNumber(data.checked, locale),
            })}
      </p>
      {(data.missingPaths ?? []).length > 0 ? (
        <ul className="space-y-0.5">
          {(data.missingPaths ?? []).map((item) => (
            <li key={item.path} className="font-mono text-xs text-muted-foreground">
              {item.path}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}


/**
 * Qui peut lire cet extrait, en plus de son proprietaire.
 *
 * Un extrait n'etait visible que par qui voyait l'ontologie : donner une
 * selection a une equipe partenaire voulait dire lui donner l'etude entiere,
 * ou lui transferer l'extrait et le perdre. C'est la mauvaise paire de choix
 * pour l'objet dont le but est d'etre plus petit que l'etude.
 *
 * Un seul role : un extrait est gele, il n'y a rien a y ecrire.
 */
export function ExtractPermissions({ extract }: { extract: Extract }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const access = useExtractAccess(extract.id);
  const grant = useMutation({
    mutationFn: (input: { subjectType: string; subjectId: string }) => ontologiesApi.grantExtract(extract.id, input.subjectType, input.subjectId),
    onSuccess: () => invalidate(qk.extractAccess(extract.id)),
    onError: (error) => toast.error(error, t('access.grant')),
  });
  const revoke = useMutation({
    mutationFn: (entry: AccessEntry) => ontologiesApi.revokeExtract(extract.id, entry.subjectType, entry.subjectId),
    onSuccess: () => invalidate(qk.extractAccess(extract.id)),
    onError: (error) => toast.error(error, t('access.revoke')),
  });
  return (
    <AccessPanel
      title={t('access.title')}
      hint={t('ontologies.extractPermissionsHint')}
      owner={{ type: extract.ownerType, id: extract.ownerId || extract.ownerUserId, name: extract.ownerName }}
      entries={access.data}
      isLoading={access.isLoading}
      isError={access.isError}
      error={access.error}
      onRetry={() => void access.refetch()}
      roles="readOnly"
      onGrant={(subjectType, subjectId) => grant.mutate({ subjectType, subjectId })}
      granting={grant.isPending}
      onRevoke={(entry) => revoke.mutate(entry)}
    />
  );
}


/**
 * A qui appartient cet extrait.
 *
 * Le transfert vivait dans le menu a trois points de la ligne, alors qu'un
 * dataset et une ontologie le posent en bas de leur detail, au-dessus de
 * leurs permissions. Trois objets du meme catalogue, trois endroits ou
 * chercher la meme chose : la propriete se range ou les deux autres la
 * rangent, avec les lectures juste en dessous, parce que c'est une seule
 * conversation - qui le detient, et qui peut le lire.
 */
export function ExtractOwnership({ extract }: { extract: Extract }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const transfer = useMutation({
    mutationFn: (input: { ownerType: string; ownerId: string }) =>
      ontologiesApi.setExtractOwner(extract.id, input),
    onSuccess: () => {
      invalidate(qk.extracts);
      invalidate(qk.extractAccess(extract.id));
      toast.success(t('projects.transferOwnership'));
    },
    onError: (error) => toast.error(error, t('projects.transferOwnership')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('projects.transferOwnership')}</CardTitle>
          <CardDescription>{t('ontologies.extractOwnershipHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent>
        <OwnerTransfer
          owner={extract}
          pending={transfer.isPending}
          onTransfer={(input) => transfer.mutate(input)}
          label={t('projects.transferOwnership')}
        />
      </CardContent>
    </Card>
  );
}
