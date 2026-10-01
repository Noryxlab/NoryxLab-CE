import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
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
