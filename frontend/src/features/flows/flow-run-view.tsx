import * as React from 'react';
import { Link } from 'react-router';
import { AlertTriangle, ArrowRight, ExternalLink, Info } from 'lucide-react';
import { Badge, StatusBadge, describeStatus } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime, formatDuration } from '@/lib/format';
import { flowColumns } from './flow-layout';
import type { Flow, FlowRun, FlowStep, FlowStepRun } from '@/lib/api/types';

/**
 * Une execution, vue de deux facons qui ne disent pas la meme chose.
 *
 * Le graphe dit la structure : qui attend qui, et donc qui peut tourner avec
 * qui. Le deroule dit ce qui s'est reellement passe : deux barres qui
 * commencent au meme instant sont la preuve de l'embranchement, la ou le
 * graphe n'en montrait que la possibilite.
 *
 * Aucune fleche n'est dessinee entre les colonnes. Une fleche fausse est pire
 * qu'une fleche absente, et placer des fleches justes demande un moteur de
 * placement ; chaque etape porte donc la liste de ce qu'elle attend, qui est
 * la meme information, lisible et toujours exacte.
 */

function stepTone(status: string): 'neutral' | 'brand' | 'success' | 'warning' | 'danger' {
  return describeStatus(status).tone;
}

function StepCard({
  step,
  run,
  projectId,
}: {
  step: FlowStep;
  run: FlowStepRun | undefined;
  projectId: string;
}) {
  const t = useT();
  const { locale } = useI18n();
  const status = run?.status ?? 'pending';
  const waiting = step.dependsOn ?? [];

  return (
    <div
      className={`space-y-1.5 rounded-md border p-2.5 ${
        stepTone(status) === 'danger'
          ? 'border-danger-subtle bg-danger-subtle/30'
          : stepTone(status) === 'success'
            ? 'border-success-subtle bg-success-subtle/20'
            : 'border-border bg-surface'
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <p className="min-w-0 truncate text-sm font-medium" title={step.key}>
          {step.name || step.key}
        </p>
        <StatusBadge status={status} locale={locale} showRaw={false} />
      </div>

      {step.command?.length ? (
        <p className="truncate font-mono text-xs text-muted-foreground" title={step.command.join(' ')}>
          {step.command.join(' ')}
        </p>
      ) : null}

      {waiting.length > 0 ? (
        <p className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
          <ArrowRight className="size-3 shrink-0" aria-hidden />
          {t('flows.dependsOn')}
          {waiting.map((key) => (
            <code key={key} className="rounded bg-surface-muted px-1 font-mono">
              {key}
            </code>
          ))}
        </p>
      ) : null}

      {/* La raison, quand l'etat seul ne suffit pas : « ignoree » sans dire
        *  laquelle a casse laisse chercher dans douze cases. */}
      {run?.reason ? (
        <p className="text-xs leading-relaxed text-muted-foreground">{run.reason}</p>
      ) : status === 'skipped' ? (
        <p className="text-xs leading-relaxed text-muted-foreground">{t('flows.skippedBecause')}</p>
      ) : null}

      {run?.startedAt ? (
        <p className="text-xs tabular-nums text-muted-foreground">
          {formatDuration(run.startedAt, run.finishedAt ?? new Date(), locale)}
        </p>
      ) : null}

      {run?.jobId ? (
        <Link
          to={`/projects/${projectId}/jobs`}
          className="inline-flex items-center gap-1 text-xs text-brand hover:underline"
        >
          <ExternalLink className="size-3" aria-hidden />
          {t('flows.openJob')}
        </Link>
      ) : status === 'pending' ? (
        <p className="text-xs text-muted-foreground">{t('flows.noJobYet')}</p>
      ) : null}
    </div>
  );
}

/** Le deroule : une barre par etape, placee sur le temps reel du run.
 *
 *  C'est la vue qui prouve le parallelisme. Elle n'est dessinee que pour les
 *  etapes qui ont demarre - une etape en attente n'a pas de place sur un axe
 *  de temps, et lui en donner une inventerait un instant. */
function Timeline({
  steps,
  stepRuns,
  run,
}: {
  steps: FlowStep[];
  stepRuns: FlowStepRun[];
  run: FlowRun;
}) {
  const t = useT();
  const { locale } = useI18n();

  const byKey = new Map(stepRuns.map((item) => [item.stepKey, item]));
  const started = steps.filter((step) => byKey.get(step.key)?.startedAt);

  const origin = new Date(run.startedAt).getTime();
  const end = run.finishedAt ? new Date(run.finishedAt).getTime() : Date.now();
  // Jamais zero : une division par la duree arriverait sur un run qui vient de
  // demarrer, et toutes les barres partiraient a l'infini.
  const span = Math.max(1000, end - origin);

  if (started.length === 0) {
    return (
      <p className="p-4 text-xs text-muted-foreground">{t('flows.runsEmptyHint')}</p>
    );
  }

  return (
    <div className="space-y-1.5 p-4">
      {started.map((step) => {
        const stepRun = byKey.get(step.key);
        const from = new Date(stepRun?.startedAt ?? run.startedAt).getTime();
        const to = stepRun?.finishedAt ? new Date(stepRun.finishedAt).getTime() : end;
        const left = ((from - origin) / span) * 100;
        const width = Math.max(1.5, ((to - from) / span) * 100);
        const tone = stepTone(stepRun?.status ?? 'pending');
        return (
          <div key={step.key} className="grid items-center gap-2 sm:grid-cols-[10rem_1fr_5rem]">
            <p className="min-w-0 truncate text-xs font-medium" title={step.key}>
              {step.name || step.key}
            </p>
            <div className="relative h-5 rounded bg-surface-muted">
              <div
                className={`absolute inset-y-0.5 rounded ${
                  tone === 'danger' ? 'bg-danger' : tone === 'success' ? 'bg-success' : 'bg-brand'
                }`}
                style={{ left: `${left}%`, width: `${width}%` }}
                title={formatDateTime(stepRun?.startedAt ?? run.startedAt, locale)}
              />
            </div>
            <p className="text-xs tabular-nums text-muted-foreground">
              {formatDuration(stepRun?.startedAt ?? run.startedAt, stepRun?.finishedAt ?? end, locale)}
            </p>
          </div>
        );
      })}
    </div>
  );
}

export function FlowRunView({
  flow,
  run,
  steps,
}: {
  flow: Flow;
  run: FlowRun;
  steps: FlowStepRun[];
}) {
  const t = useT();
  const { locale } = useI18n();
  const columns = React.useMemo(() => flowColumns(flow.steps), [flow.steps]);
  const byKey = new Map(steps.map((item) => [item.stepKey, item]));
  const failed = steps.filter((item) => item.status === 'failed');
  const skipped = steps.filter((item) => item.status === 'skipped');
  /* Les etapes que cette execution a faites et que le graphe ne declare plus.
   *
   *  Un flow se modifie, ses executions passees ne se modifient pas. Rendre
   *  le graphe d'aujourd'hui pour une execution d'hier ferait disparaitre ces
   *  etapes de l'ecran sans un mot - et c'est precisement sur une execution
   *  qu'on vient relire qu'on a besoin de savoir qu'elle ne correspond plus a
   *  la definition courante. */
  const orphans = steps.filter((item) => !flow.steps.some((step) => step.key === item.stepKey));

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle className="flex flex-wrap items-center gap-2">
            {flow.name}
            <StatusBadge status={run.status} locale={locale} showRaw={false} />
            {run.resumeOf ? <Badge tone="brand">{t('flows.resumeOf')}</Badge> : null}
          </CardTitle>
          <CardDescription>
            {formatDateTime(run.startedAt, locale)} —{' '}
            {formatDuration(run.startedAt, run.finishedAt ?? new Date(), locale)} ·{' '}
            {t('flows.maxParallel', { count: String(flow.maxParallel) })}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>

      {/* Quand une etape a echoue et que d'autres ont ete ignorees, la lecture
        *  du graphe est ambigue : douze cases eteintes, une seule cassee. Dit
        *  en haut, avant le graphe. */}
      {failed.length > 0 ? (
        <CardContent className="border-b border-border">
          <div className="flex items-start gap-2 rounded-md border border-danger-subtle bg-danger-subtle/30 p-3">
            <AlertTriangle className="mt-0.5 size-4 shrink-0 text-danger-foreground" aria-hidden />
            <div className="space-y-1 text-xs leading-relaxed">
              <p className="font-medium">
                {failed.map((item) => item.stepKey).join(', ')}
              </p>
              {skipped.length > 0 ? (
                <p className="text-muted-foreground">
                  {t('flows.failureStopsDependents')} — {t('flows.failureStopsDependentsHint')}
                </p>
              ) : null}
            </div>
          </div>
        </CardContent>
      ) : null}

      {orphans.length > 0 ? (
        <CardContent className="border-b border-border">
          <div className="flex items-start gap-2 rounded-md border border-border bg-surface-muted p-3">
            <Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
            <div className="space-y-1 text-xs leading-relaxed">
              <p className="font-medium">{t('flows.definitionMoved')}</p>
              <p className="text-muted-foreground">{t('flows.definitionMovedHint')}</p>
              <ul className="space-y-0.5 pt-0.5">
                {orphans.map((item) => (
                  <li key={item.stepKey} className="flex items-center gap-1.5">
                    <code className="font-mono">{item.stepKey}</code>
                    <StatusBadge status={item.status} locale={locale} showRaw={false} />
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </CardContent>
      ) : null}

      <Tabs defaultValue="graph">
        <TabsList className="mx-4 mt-3">
          <TabsTrigger value="graph">{t('flows.graph')}</TabsTrigger>
          <TabsTrigger value="timeline">{t('flows.timeline')}</TabsTrigger>
        </TabsList>

        <TabsContent value="graph">
          {/* Le graphe defile horizontalement dans son propre conteneur : une
            *  chaine de huit etapes ne doit pas faire defiler la page. */}
          <div className="overflow-x-auto p-4">
            <div className="flex min-w-fit items-start gap-3">
              {columns.map((column, index) => (
                <div key={index} className="w-56 shrink-0 space-y-2">
                  <p className="text-xs font-medium text-muted-foreground">
                    {index + 1} / {columns.length}
                  </p>
                  {column.map((step) => (
                    <StepCard
                      key={step.key}
                      step={step}
                      run={byKey.get(step.key)}
                      projectId={flow.projectId}
                    />
                  ))}
                </div>
              ))}
            </div>
          </div>
          <CardContent className="flex items-start gap-2 border-t border-border pt-3">
            <Info className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" aria-hidden />
            <p className="text-xs leading-relaxed text-muted-foreground">
              <span className="font-medium text-foreground">{t('flows.stepIsAJob')}</span> —{' '}
              {t('flows.stepIsAJobHint')}
            </p>
          </CardContent>
        </TabsContent>

        <TabsContent value="timeline">
          <Timeline steps={flow.steps} stepRuns={steps} run={run} />
        </TabsContent>
      </Tabs>
    </Card>
  );
}
