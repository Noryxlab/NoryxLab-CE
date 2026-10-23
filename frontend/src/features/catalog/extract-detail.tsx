import * as React from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Trash2 } from 'lucide-react';
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
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useToast } from '@/components/ui/toast';
import { qk, useAdminUsers, useAllTeams, useExtractAccess, useInvalidate, useOrganizations } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber } from '@/lib/format';
import type { Extract, ExtractAccess } from '@/lib/api/types';

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
  const users = useAdminUsers();
  const organizations = useOrganizations();
  const teams = useAllTeams();

  const [subjectType, setSubjectType] = React.useState<'user' | 'organization' | 'team'>('user');
  const [subjectId, setSubjectId] = React.useState('');

  const grant = useMutation({
    mutationFn: () => ontologiesApi.grantExtract(extract.id, subjectType, subjectId),
    onSuccess: () => {
      invalidate(qk.extractAccess(extract.id));
      setSubjectId('');
    },
    onError: (error) => toast.error(error, t('ontologies.extractGrant')),
  });

  const revoke = useMutation({
    mutationFn: (entry: ExtractAccess) =>
      ontologiesApi.revokeExtract(extract.id, entry.subjectType, entry.subjectId),
    onSuccess: () => invalidate(qk.extractAccess(extract.id)),
    onError: (error) => toast.error(error, t('ontologies.extractRevoke')),
  });

  const subjectOptions =
    subjectType === 'organization'
      ? (organizations.data ?? []).map((organization) => ({
          value: organization.alias ?? organization.id,
          label: organization.name,
        }))
      : subjectType === 'team'
        ? teams.data.map((team) => ({
            value: team.id,
            label: team.name,
            hint: team.organizationName || undefined,
          }))
        : (users.data ?? []).map((user) => ({
            value: user.username ?? user.id,
            label: user.username ?? user.id,
            hint: user.email ?? undefined,
          }));

  const entries = access.data ?? [];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.extractPermissions')}</CardTitle>
          <CardDescription>{t('ontologies.extractPermissionsHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <Field label={t('common.type')}>
            <Select
              value={subjectType}
              onValueChange={(value) => {
                setSubjectType(
                  value === 'organization' ? 'organization' : value === 'team' ? 'team' : 'user',
                );
                setSubjectId('');
              }}
              options={[
                { value: 'user', label: t('common.user') },
                { value: 'team', label: t('common.team') },
                { value: 'organization', label: t('common.organization') },
              ]}
            />
          </Field>
          <Field label={t('rbac.subject')}>
            <Select
              value={subjectId}
              onValueChange={setSubjectId}
              options={subjectOptions}
              placeholder={t('common.search')}
            />
          </Field>
          <Button
            variant="primary"
            disabled={!subjectId}
            loading={grant.isPending}
            onClick={() => grant.mutate()}
          >
            {t('ontologies.extractGrant')}
          </Button>
        </div>

        <TableWrapper>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('rbac.subject')}</TableHead>
                <TableHead>{t('common.type')}</TableHead>
                <TableHead>{t('common.role')}</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map((entry) => (
                <TableRow key={`${entry.subjectType}:${entry.subjectId}`}>
                  <TableCell>
                    <span className="font-medium">{entry.subjectName || entry.subjectId}</span>
                    {entry.subjectName && entry.subjectName !== entry.subjectId ? (
                      <div className="truncate font-mono text-xs text-muted-foreground">
                        {entry.subjectId}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    <Badge tone="outline">
                      {entry.subjectType === 'organization'
                        ? t('common.organization')
                        : entry.subjectType === 'team'
                          ? t('common.team')
                          : t('common.user')}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge tone={entry.role === 'owner' ? 'brand' : 'outline'}>
                      {entry.role === 'owner' ? t('common.owner') : t('common.reader')}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    {/* Le proprietaire ne se retire pas : il se transfere, et
                        c'est un autre geste. */}
                    {entry.role === 'owner' ? null : (
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={t('ontologies.extractRevoke')}
                        onClick={() => revoke.mutate(entry)}
                      >
                        <Trash2 aria-hidden />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrapper>
      </CardContent>
    </Card>
  );
}
