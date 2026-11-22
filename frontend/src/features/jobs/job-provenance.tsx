import * as React from 'react';
import { AlertTriangle, Check, FileClock, GitCommitHorizontal, Lock } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/common/states';
import { CopyButton } from '@/components/common/copy-button';
import { useT } from '@/lib/i18n';
import type { Job } from '@/lib/api/types';

/**
 * Ce qu'un run a lu, et quel code il a execute.
 *
 * L'ecran existe pour une raison precise : le bouton « rejouer » a cote ne
 * vaut que ce que vaut cette trace. Sans elle, un rejeu relancerait la
 * definition d'aujourd'hui sous le nom d'hier - l'image que l'etiquette
 * designe maintenant, les datasets que le projet attache maintenant - et
 * appellerait ca une reproduction. Alors la trace est montree avant le
 * bouton, et son absence est dite plutot que laissee en blanc.
 */

/** L'empreinte, raccourcie comme une forge la raccourcit, avec le tout a copier. */
function Digest({ value }: { value: string }) {
  const algorithm = value.includes(':') ? value.split(':')[0] : '';
  const hex = value.includes(':') ? (value.split(':')[1] ?? '') : value;
  return (
    <span className="inline-flex items-center gap-1">
      <code className="font-mono text-xs">
        {algorithm ? `${algorithm}:` : ''}
        {hex.slice(0, 12)}
      </code>
      <CopyButton value={value} size="sm" variant="ghost" />
    </span>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1 border-b border-border py-2.5 last:border-0 sm:grid-cols-[11rem_1fr] sm:gap-4">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-sm">{children}</dd>
    </div>
  );
}

export function JobProvenance({ job }: { job: Job }) {
  const t = useT();

  const datasets = job.datasets ?? [];
  const repositories = job.repositories ?? [];
  const extracts = job.extracts ?? [];
  // Une trace absente n'est pas une trace vide. Un run d'avant
  // l'enregistrement n'a ni empreinte, ni datasets, ni depots - et un run
  // legitime peut n'avoir monte aucune donnee. Le seul signe qui distingue
  // les deux est l'empreinte d'image, que tout lancement enregistre depuis.
  const recorded = Boolean(job.imageDigest) || datasets.length > 0 || repositories.length > 0;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('jobs.provenance')}</CardTitle>
          <CardDescription>{t('jobs.provenanceHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>

      {recorded ? (
        <CardContent className="pt-1">
          <dl>
            <Row label={t('jobs.imageDigest')}>
              <p className="truncate font-mono text-xs">{job.image}</p>
              {job.imageDigest ? (
                <Digest value={job.imageDigest} />
              ) : (
                <p className="flex items-start gap-1.5 pt-1 text-xs text-warning-foreground">
                  <AlertTriangle className="mt-px size-3.5 shrink-0" aria-hidden />
                  {t('jobs.imageDigestMissing')}
                </p>
              )}
            </Row>

            {datasets.length > 0 || extracts.length > 0 ? (
              <Row label={t('jobs.datasetsMounted')}>
                <ul className="space-y-1">
                  {datasets.map((dataset) => (
                    <li key={dataset.id} className="flex flex-wrap items-center gap-1.5">
                      <span className="font-medium">{dataset.name}</span>
                      {dataset.readOnly ? (
                        <Badge tone="outline">
                          <Lock className="size-3" aria-hidden />
                          {t('jobs.datasetReadOnly')}
                        </Badge>
                      ) : null}
                      {dataset.prefix ? (
                        <code className="font-mono text-xs text-muted-foreground">{dataset.prefix}</code>
                      ) : null}
                    </li>
                  ))}
                  {extracts.map((extract) => (
                    <li key={extract} className="flex items-center gap-1.5">
                      <Badge tone="brand">{t('search.kind_extract')}</Badge>
                      <code className="font-mono text-xs">{extract}</code>
                    </li>
                  ))}
                </ul>
              </Row>
            ) : null}

            {repositories.length > 0 ? (
              <Row label={t('jobs.repositoriesAtCommit')}>
                <ul className="space-y-1.5">
                  {repositories.map((repository) => (
                    <li key={repository.name} className="min-w-0">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="font-medium">{repository.name}</span>
                        {repository.ref ? (
                          <Badge tone="outline">{repository.ref}</Badge>
                        ) : null}
                        {repository.commit ? (
                          <span className="inline-flex items-center gap-1 text-muted-foreground">
                            <GitCommitHorizontal className="size-3.5" aria-hidden />
                            <code className="font-mono text-xs">{repository.commit.slice(0, 8)}</code>
                            <CopyButton value={repository.commit} size="sm" variant="ghost" />
                          </span>
                        ) : (
                          <Badge tone="warning">{t('jobs.commitUnknown')}</Badge>
                        )}
                      </div>
                      {repository.commit ? null : (
                        <p className="pt-0.5 text-xs text-muted-foreground">{t('jobs.commitUnknownHint')}</p>
                      )}
                    </li>
                  ))}
                </ul>
              </Row>
            ) : null}

            {job.rerunOf ? (
              <Row label={t('jobs.rerunOf')}>
                <code className="font-mono text-xs">{job.rerunOf}</code>
              </Row>
            ) : null}
          </dl>

          {/* Les ecarts, et la difference entre « aucun » et « sans objet ».
            *  Un rejeu fidele l'affiche comme un resultat ; un run qui n'est
            *  pas un rejeu n'a rien a comparer et ne doit pas se voir
            *  decerner une fidelite qui ne veut rien dire. */}
          {job.rerunOf ? (
            (job.divergences ?? []).length > 0 ? (
              <div className="mt-3 rounded-md border border-warning-subtle bg-warning-subtle/40 p-3">
                <p className="flex items-center gap-1.5 text-xs font-medium text-warning-foreground">
                  <AlertTriangle className="size-3.5" aria-hidden />
                  {t('jobs.divergences')}
                </p>
                <ul className="mt-1.5 space-y-1 text-xs leading-relaxed text-foreground">
                  {(job.divergences ?? []).map((divergence) => (
                    <li key={divergence} className="flex gap-1.5">
                      <span aria-hidden>·</span>
                      <span>{divergence}</span>
                    </li>
                  ))}
                </ul>
                <p className="mt-2 text-xs text-muted-foreground">{t('jobs.divergencesHint')}</p>
              </div>
            ) : (
              <div className="mt-3 rounded-md border border-border bg-surface-muted p-3">
                <p className="flex items-center gap-1.5 text-xs font-medium text-success-foreground">
                  <Check className="size-3.5" aria-hidden />
                  {t('jobs.divergencesNone')}
                </p>
                <p className="mt-0.5 text-xs text-muted-foreground">{t('jobs.divergencesNoneHint')}</p>
              </div>
            )
          ) : null}
        </CardContent>
      ) : (
        <EmptyState
          compact
          icon={FileClock}
          title={t('jobs.provenanceEmpty')}
          description={t('jobs.provenanceEmptyHint')}
        />
      )}
    </Card>
  );
}
