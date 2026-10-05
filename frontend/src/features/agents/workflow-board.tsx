import * as React from 'react';
import { Check, Loader2, Plus, RotateCw, Trash2, X } from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { SectionHeader } from '@/components/common/page-header';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import { platformApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, useWorkflowRuns, useWorkflows } from '@/lib/api/queries';
import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';
import { relativeTime, scheduleKey } from './agent-roster';
import { WorkflowDialog } from './workflow-dialog';
import type { Workflow, WorkflowRun, WorkflowRunStatus, WorkflowStep } from '@/lib/api/types';

/**
 * Les workflows, sous les agents.
 *
 * Meme forme que la page au-dessus : une rangee de tuiles qu'on parcourt d'un
 * regard, et le detail de celle qu'on a choisie en dessous. Le detail est en
 * deux parties, et l'ordre compte.
 *
 * D'abord la fiche de mission : ce pourquoi il est la, et ou il en est. Elle
 * est derivee du workflow et de son dernier run, jamais redigee - un resume
 * ecrit a la main se separe de la verite le jour ou quelqu'un modifie un pas
 * et oublie la phrase. Le pas courant est surligne sur la sequence, parce que
 * « il en est a la relecture » se lit plus vite sur la liste des pas que dans
 * un mot.
 *
 * Puis les runs : un fil, du plus recent au passe, avec l'etat de chaque pas.
 * Un run qui attend une personne porte ses deux boutons ici, dans le fil, a
 * l'endroit ou l'on vient de lire ce qu'il a trouve - pas dans une boite de
 * reception a cote.
 */
export function WorkflowsSection({ projectId }: { projectId: string | undefined }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const workflows = useWorkflows();
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [writing, setWriting] = React.useState(false);
  const [deleting, setDeleting] = React.useState<Workflow | null>(null);

  const items = (workflows.data ?? []).filter((item) => item.projectId === projectId);
  const selected = items.find((item) => item.id === selectedId) ?? items[0] ?? null;
  const runs = useWorkflowRuns(selected?.id);

  const runNow = useMutation({
    mutationFn: (id: string) => platformApi.runWorkflow(id),
    onSuccess: (run) => {
      invalidate(qk.workflows);
      if (selected) invalidate(qk.workflowRuns(selected.id));
      if (run.error) toast.error(run.error, t('workflows.title'));
      else toast.success(t('workflows.started'), t('workflows.title'));
    },
    onError: (error) => toast.error(error, t('workflows.title')),
  });

  const decide = useMutation({
    mutationFn: ({ runId, approve }: { runId: string; approve: boolean }) =>
      approve ? platformApi.approveWorkflowRun(runId) : platformApi.rejectWorkflowRun(runId),
    onSuccess: (run, { approve }) => {
      invalidate(qk.workflowRuns(run.workflowId));
      invalidate(qk.workflows);
      toast.success(approve ? t('workflows.approved') : t('workflows.rejected'), t('workflows.title'));
    },
    onError: (error) => toast.error(error, t('workflows.title')),
  });

  const remove = useMutation({
    mutationFn: (id: string) => platformApi.deleteWorkflow(id),
    onSuccess: (_result, id) => {
      invalidate(qk.workflows);
      if (selectedId === id) setSelectedId(null);
      toast.success(t('workflows.deleted', { name: deleting?.name ?? '' }), t('workflows.title'));
      setDeleting(null);
    },
    onError: (error) => toast.error(error, t('workflows.title')),
  });

  return (
    <section className="space-y-4">
      <SectionHeader
        title={t('workflows.title')}
        description={t('workflows.intro')}
        actions={
          items.length > 0 ? (
            <Button variant="secondary" size="sm" onClick={() => setWriting(true)}>
              {t('workflows.create')}
            </Button>
          ) : null
        }
      />

      {workflows.isLoading ? (
        <Skeleton className="h-10 w-full" />
      ) : items.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border px-6 py-8 text-center">
          <p className="mx-auto max-w-lg text-sm leading-relaxed text-muted-foreground">
            {t('workflows.emptyLead')}
          </p>
          <Button className="mt-5" variant="secondary" onClick={() => setWriting(true)}>
            {t('workflows.createFirst')}
          </Button>
        </div>
      ) : (
        <>
          <div className="flex flex-wrap gap-3">
            {items.map((item) => (
              <WorkflowTile
                key={item.id}
                workflow={item}
                selected={item.id === selected?.id}
                onSelect={() => setSelectedId(item.id)}
              />
            ))}
            <button
              type="button"
              onClick={() => setWriting(true)}
              className={cn(
                'flex w-44 flex-col items-center justify-center gap-2 rounded-xl border border-dashed',
                'border-border-strong/70 px-4 py-5 text-sm text-muted-foreground transition',
                'hover:border-brand hover:text-brand focus-visible:outline-2 focus-visible:outline-offset-2',
                'focus-visible:outline-[var(--noryx-ring)]',
              )}
            >
              <Plus className="size-5" aria-hidden />
              {t('workflows.create')}
            </button>
          </div>

          {selected ? (
            <>
              <MissionCard
                workflow={selected}
                latest={runs.data?.[0]}
                running={runNow.isPending}
                onRun={() => runNow.mutate(selected.id)}
                onDelete={() => setDeleting(selected)}
              />
              <RunTimeline
                workflow={selected}
                runs={runs.data ?? []}
                loading={runs.isLoading}
                deciding={decide.isPending}
                onDecide={(runId, approve) => decide.mutate({ runId, approve })}
              />
            </>
          ) : null}
        </>
      )}

      <WorkflowDialog open={writing} onOpenChange={setWriting} projectId={projectId} />
      <Dialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('workflows.deleteTitle', { name: deleting?.name ?? '' })}</DialogTitle>
            <DialogDescription>{t('workflows.deleteBody')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleting(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="danger"
              onClick={() => deleting && remove.mutate(deleting.id)}
              disabled={remove.isPending}
            >
              {t('workflows.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

function WorkflowTile({
  workflow,
  selected,
  onSelect,
}: {
  workflow: Workflow;
  selected: boolean;
  onSelect: () => void;
}) {
  const t = useT();
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        'flex w-44 flex-col items-start gap-1.5 rounded-xl border bg-surface px-4 py-4 text-left transition',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--noryx-ring)]',
        selected ? 'border-brand shadow-[0_0_0_1px_var(--noryx-brand)]' : 'border-border hover:border-border-strong',
        !workflow.enabled && 'opacity-65',
      )}
    >
      <span className="max-w-full truncate text-sm font-medium">{workflow.name}</span>
      {/* La sequence en miniature : autant de points que de pas, les
          approbations creusees. On voit la forme avant de lire. */}
      <span className="flex items-center gap-1" aria-hidden>
        {workflow.steps.map((step) => (
          <span
            key={step.index}
            className={cn(
              'size-2 rounded-full',
              step.kind === 'approval' ? 'border border-border-strong' : 'bg-border-strong',
            )}
          />
        ))}
      </span>
      <span className="text-[11px] text-placeholder">
        {t(scheduleKey(workflow.schedule))} · {relativeTime(workflow.lastRunAt, t)}
      </span>
    </button>
  );
}

/**
 * La fiche de mission.
 *
 * Trois questions, dans l'ordre ou on les pose en arrivant : pourquoi il est
 * la, ou il en est, et quand il a travaille. Tout est derive : le pas courant
 * vient du dernier run, le prochain passage des horaires. Rien ici n'est une
 * phrase qu'il faudrait tenir a jour.
 */
function MissionCard({
  workflow,
  latest,
  running,
  onRun,
  onDelete,
}: {
  workflow: Workflow;
  latest: WorkflowRun | undefined;
  running: boolean;
  onRun: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  const current = latest && !isTerminal(latest.status) ? currentStep(latest) : null;
  const next =
    workflow.schedule === 'hourly'
      ? t('workflows.nextHourly')
      : workflow.schedule === 'daily'
        ? t('workflows.nextDaily')
        : t('workflows.nextManual');

  return (
    <div className="rounded-xl border border-border bg-surface p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-3">
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-placeholder">
              {t('workflows.missionTitle')}
            </p>
            <h3 className="text-sm font-semibold">{workflow.name}</h3>
          </div>
          <ol className="space-y-1.5">
            {workflow.steps.map((step) => {
              const stepRun = latest?.steps.find((item) => item.index === step.index);
              const isCurrent = current !== null && current === step.index;
              return (
                <li
                  key={step.index}
                  className={cn(
                    'flex items-start gap-3 rounded-lg px-2 py-1.5',
                    isCurrent && 'bg-brand-subtle',
                  )}
                >
                  <span className="w-5 shrink-0 text-xs text-placeholder tabular-nums">
                    {step.index + 1}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline gap-2">
                      <span className="text-sm font-medium">{step.name || stepKindLabel(step, t)}</span>
                      <Badge tone={step.kind === 'approval' ? 'outline' : 'neutral'}>
                        {stepKindLabel(step, t)}
                      </Badge>
                      {stepRun && latest ? <StatusBadge status={stepRun.status} /> : null}
                    </div>
                    {step.kind === 'agent' && step.instruction ? (
                      <p className="mt-0.5 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                        {step.instruction}
                      </p>
                    ) : null}
                    {step.kind === 'approval' ? (
                      <p className="mt-0.5 text-xs text-muted-foreground">
                        {t('workflows.waitingFor', { name: step.approverUserId ?? '' })}
                      </p>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ol>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-2">
          <Button variant="secondary" size="sm" onClick={onRun} disabled={running}>
            {running ? (
              <Loader2 className="size-4 animate-spin" aria-hidden />
            ) : (
              <RotateCw className="size-4" aria-hidden />
            )}
            {t('workflows.runNow')}
          </Button>
          <Button variant="ghost" size="sm" onClick={onDelete} title={t('workflows.delete')}>
            <Trash2 className="size-4" aria-hidden />
            <span className="sr-only">{t('workflows.delete')}</span>
          </Button>
        </div>
      </div>

      <dl className="mt-4 grid gap-3 border-t border-border pt-3 text-xs sm:grid-cols-3">
        <div>
          <dt className="text-placeholder">{t('workflows.whereItIs')}</dt>
          <dd className="mt-0.5 text-foreground">
            {current !== null && latest
              ? t('workflows.atStep', {
                  index: current + 1,
                  name: workflow.steps[current]?.name ?? '',
                })
              : t('workflows.idle')}
          </dd>
        </div>
        <div>
          <dt className="text-placeholder">{t('workflows.lastRun')}</dt>
          <dd className="mt-0.5 flex items-center gap-2 text-foreground">
            {relativeTime(latest?.startedAt, t)}
            {latest ? <StatusBadge status={latest.status} /> : null}
          </dd>
        </div>
        <div>
          <dt className="text-placeholder">{t('workflows.nextRun')}</dt>
          <dd className="mt-0.5 text-foreground">{workflow.enabled ? next : t('agents.paused')}</dd>
        </div>
      </dl>
    </div>
  );
}

/**
 * Les runs, du plus recent au passe.
 *
 * Chaque run deroule ses pas. Un pas agent montre ce qu'il a produit ; un pas
 * d'approbation qui attend montre les deux boutons, ici, parce que la decision
 * se prend en lisant ce que le pas d'avant a trouve.
 */
function RunTimeline({
  workflow,
  runs,
  loading,
  deciding,
  onDecide,
}: {
  workflow: Workflow;
  runs: WorkflowRun[];
  loading: boolean;
  deciding: boolean;
  onDecide: (runId: string, approve: boolean) => void;
}) {
  const t = useT();
  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">{t('workflows.runsTitle')}</h3>
      {loading && runs.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('workflows.loadingRuns')}</p>
      ) : runs.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
          {t('workflows.neverRan')}
        </p>
      ) : (
        <ol className="relative space-y-0 border-l border-border pl-5">
          {runs.map((run) => (
            <li key={run.id} className="relative py-3">
              <span
                aria-hidden
                className={cn(
                  'absolute -left-[1.4rem] top-[1.15rem] size-2 rounded-full ring-2 ring-[var(--noryx-background)]',
                  dotFor(run.status),
                )}
              />
              <div className="flex flex-wrap items-baseline gap-2">
                <span className="text-xs text-placeholder tabular-nums">{relativeTime(run.startedAt, t)}</span>
                <StatusBadge status={run.status} />
              </div>
              {run.error ? (
                <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{run.error}</p>
              ) : null}
              <ol className="mt-2 space-y-1.5">
                {run.steps.map((stepRun) => {
                  const step = workflow.steps[stepRun.index];
                  const waiting = run.status === 'waiting_approval' && stepRun.status === 'waiting_approval';
                  return (
                    <li key={stepRun.index} className="rounded-lg bg-surface-muted px-3 py-2">
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="text-xs text-placeholder tabular-nums">{stepRun.index + 1}</span>
                        <span className="text-sm font-medium">
                          {step?.name || (step ? stepKindLabel(step, t) : '')}
                        </span>
                        <StatusBadge status={stepRun.status} />
                        {stepRun.attempts > 1 ? (
                          <span className="text-xs text-placeholder">
                            {t('workflows.attempts', { count: stepRun.attempts })}
                          </span>
                        ) : null}
                      </div>
                      {stepRun.output ? (
                        <p className="mt-1 text-sm leading-relaxed whitespace-pre-line">{stepRun.output}</p>
                      ) : null}
                      {stepRun.error && stepRun.status !== 'succeeded' ? (
                        <p className="mt-1 text-xs text-muted-foreground">{stepRun.error}</p>
                      ) : null}
                      {stepRun.actions.length > 0 ? (
                        <div className="mt-1.5 flex flex-wrap gap-1.5">
                          {stepRun.actions.map((action, index) => (
                            <Badge key={`${action}-${index}`} tone="success">
                              {t('agents.actionRestartApp')}
                            </Badge>
                          ))}
                        </div>
                      ) : null}
                      {waiting ? (
                        <div className="mt-2 flex flex-wrap gap-2">
                          <Button size="sm" onClick={() => onDecide(run.id, true)} disabled={deciding}>
                            <Check className="size-4" aria-hidden />
                            {t('workflows.approve')}
                          </Button>
                          <Button
                            size="sm"
                            variant="danger-outline"
                            onClick={() => onDecide(run.id, false)}
                            disabled={deciding}
                          >
                            <X className="size-4" aria-hidden />
                            {t('workflows.reject')}
                          </Button>
                        </div>
                      ) : null}
                    </li>
                  );
                })}
              </ol>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: WorkflowRunStatus }) {
  const t = useT();
  const [tone, key] = (
    {
      pending: ['neutral', 'workflows.statusPending'],
      running: ['brand', 'workflows.statusRunning'],
      waiting_approval: ['warning', 'workflows.statusWaiting'],
      succeeded: ['success', 'workflows.statusSucceeded'],
      failed: ['danger', 'workflows.statusFailed'],
      cancelled: ['outline', 'workflows.statusCancelled'],
    } as const
  )[status];
  return <Badge tone={tone}>{t(key)}</Badge>;
}

function dotFor(status: WorkflowRunStatus): string {
  switch (status) {
    case 'failed':
      return 'bg-[var(--noryx-danger)]';
    case 'succeeded':
    case 'running':
      return 'bg-brand';
    default:
      return 'bg-border-strong';
  }
}

function isTerminal(status: WorkflowRunStatus): boolean {
  return status === 'succeeded' || status === 'failed' || status === 'cancelled';
}

/** Le premier pas non fini, derive du run comme le fait le serveur. */
function currentStep(run: WorkflowRun): number | null {
  const step = run.steps.find((item) => item.status !== 'succeeded');
  return step ? step.index : null;
}

type Translate = ReturnType<typeof useT>;

function stepKindLabel(step: WorkflowStep, t: Translate): string {
  return step.kind === 'approval' ? t('workflows.stepApproval') : t('workflows.stepAgent');
}
