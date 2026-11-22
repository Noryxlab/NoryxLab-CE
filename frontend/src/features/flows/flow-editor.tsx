import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import { flowsApi } from '@/lib/api/endpoints';
import { qk, useEnvironments, useHardwareTiers, useInvalidate } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { presentTier } from '@/lib/presenters';
import { flowColumns } from './flow-layout';
import type { Flow, FlowStep } from '@/lib/api/types';

/**
 * Declarer un graphe.
 *
 * Les dependances se choisissent parmi les etapes deja ecrites, et seulement
 * celles-la : c'est ce qui rend un cycle impossible a saisir plutot que
 * refuse a l'enregistrement. Le serveur valide quand meme - il est le seul a
 * faire foi - mais personne n'a a decouvrir la regle par un refus.
 */

/** Une cle de la forme que le serveur accepte, derivee du libelle : on tape un
 *  nom lisible et la cle suit, parce que devoir inventer deux identifiants
 *  pour une etape est la premiere chose qu'on fait de travers. */
function keyFrom(label: string): string {
  return label
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40);
}

function splitCommand(line: string): string[] {
  const matches = line.trim().match(/(?:[^\s'"]+|'[^']*'|"[^"]*")+/g);
  return (matches ?? []).map((part) => part.replace(/^['"]|['"]$/g, ''));
}

interface Draft extends FlowStep {
  /** La ligne telle qu'elle est tapee. Gardee a part de `command`, sinon un
   *  espace en cours de frappe reconstruirait le tableau a chaque caractere. */
  commandLine: string;
}

export function FlowEditorSheet({
  projectId,
  flow,
  open,
  onOpenChange,
}: {
  projectId: string;
  flow: Flow | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const environments = useEnvironments();
  const tiers = useHardwareTiers();

  const usable = (environments.data ?? []).filter((environment) => environment.destinationImage);
  const defaultImage = usable[0]?.destinationImage ?? '';

  const [name, setName] = React.useState('');
  const [maxParallel, setMaxParallel] = React.useState('3');
  const [steps, setSteps] = React.useState<Draft[]>([]);

  React.useEffect(() => {
    if (!open) return;
    setName(flow?.name ?? '');
    setMaxParallel(String(flow?.maxParallel ?? 3));
    setSteps(
      (flow?.steps ?? []).map((step) => ({ ...step, commandLine: (step.command ?? []).join(' ') })),
    );
  }, [open, flow]);

  const addStep = () =>
    setSteps((current) => [
      ...current,
      {
        key: '',
        name: '',
        image: defaultImage,
        commandLine: '',
        dependsOn: current.length > 0 ? [current[current.length - 1]?.key ?? ''].filter(Boolean) : [],
      },
    ]);

  const patch = (index: number, change: Partial<Draft>) =>
    setSteps((current) => current.map((step, position) => (position === index ? { ...step, ...change } : step)));

  const removeStep = (index: number) =>
    setSteps((current) => {
      const removed = current[index]?.key;
      return current
        .filter((_, position) => position !== index)
        // Une dependance vers l'etape retiree disparait avec elle : la laisser
        // produirait un graphe que le serveur refuse, pour une raison que
        // personne n'a choisie.
        .map((step) => ({
          ...step,
          dependsOn: (step.dependsOn ?? []).filter((key) => key !== removed),
        }));
    });

  const toggleDependency = (index: number, key: string) =>
    setSteps((current) =>
      current.map((step, position) => {
        if (position !== index) return step;
        const held = step.dependsOn ?? [];
        return {
          ...step,
          dependsOn: held.includes(key) ? held.filter((item) => item !== key) : [...held, key],
        };
      }),
    );

  const prepared: FlowStep[] = steps.map((step) => ({
    key: step.key || keyFrom(step.name ?? ''),
    name: step.name ?? '',
    image: step.image,
    command: splitCommand(step.commandLine),
    ...(step.hardwareTier ? { hardwareTier: step.hardwareTier } : {}),
    dependsOn: step.dependsOn ?? [],
  }));

  const keys = prepared.map((step) => step.key);
  const duplicated = keys.some((key, index) => key !== '' && keys.indexOf(key) !== index);
  const valid =
    name.trim() !== '' &&
    prepared.length > 0 &&
    prepared.every((step) => step.key !== '' && step.image !== '' && (step.command ?? []).length > 0) &&
    !duplicated;

  const save = useMutation({
    mutationFn: () => {
      const payload = {
        name: name.trim(),
        steps: prepared,
        maxParallel: Number(maxParallel) || 3,
      };
      return flow
        ? flowsApi.update(projectId, flow.id, payload)
        : flowsApi.create(projectId, payload);
    },
    onSuccess: () => {
      invalidate(qk.flows(projectId));
      onOpenChange(false);
      toast.success(t('flows.saved'), t('flows.title'));
    },
    onError: (error) => toast.error(error, t('flows.createTitle')),
  });

  // L'apercu des colonnes pendant la saisie : c'est ce qui montre qu'on est en
  // train d'ecrire un graphe et non une liste.
  const columns = flowColumns(prepared.filter((step) => step.key !== ''));

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent size="lg" aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (valid) save.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>{t('flows.createTitle')}</SheetTitle>
            <SheetDescription>{t('flows.createHint')}</SheetDescription>
          </SheetHeader>

          <SheetBody>
            <Field label={t('flows.nameLabel')} required>
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t('flows.namePlaceholder')}
                maxLength={80}
              />
            </Field>

            <Field label={t('flows.maxParallel', { count: maxParallel })} description={t('flows.maxParallelHint')}>
              <Input
                type="number"
                min={1}
                max={20}
                value={maxParallel}
                onChange={(event) => setMaxParallel(event.target.value)}
                className="tabular-nums"
              />
            </Field>

            <div className="space-y-3">
              {steps.map((step, index) => (
                <div key={index} className="space-y-2.5 rounded-md border border-border p-3">
                  <div className="flex items-start justify-between gap-2">
                    <p className="text-xs font-medium text-muted-foreground">
                      {t('flows.steps')} {index + 1}
                    </p>
                    <Button type="button" variant="ghost" size="sm" onClick={() => removeStep(index)}>
                      <Trash2 aria-hidden />
                      {t('flows.removeStep')}
                    </Button>
                  </div>

                  <Field label={t('flows.stepNameLabel')} required>
                    <Input
                      value={step.name ?? ''}
                      onChange={(event) =>
                        patch(index, {
                          name: event.target.value,
                          // La cle suit le libelle tant qu'on ne l'a pas
                          // touchee a la main.
                          key: step.key && step.key !== keyFrom(step.name ?? '') ? step.key : keyFrom(event.target.value),
                        })
                      }
                      placeholder="maillage"
                    />
                  </Field>

                  <Field label={t('flows.stepKeyLabel')} description={t('flows.stepKeyHint')} required>
                    <Input
                      value={step.key}
                      onChange={(event) => patch(index, { key: keyFrom(event.target.value) })}
                      className="font-mono text-xs"
                    />
                  </Field>

                  <Field label={t('workspaces.environmentLabel')} required>
                    <Select
                      value={step.image}
                      onValueChange={(value) => patch(index, { image: value })}
                      options={usable.map((item) => ({
                        value: item.destinationImage ?? '',
                        label: item.name,
                      }))}
                    />
                  </Field>

                  <Field label={t('workspaces.tierLabel')}>
                    <Select
                      value={step.hardwareTier ?? ''}
                      onValueChange={(value) => patch(index, { hardwareTier: value })}
                      options={(tiers.data ?? []).map((tier) => {
                        const presented = presentTier(tier, locale);
                        return { value: tier.id, label: presented.name, hint: presented.specs };
                      })}
                    />
                  </Field>

                  <Field label={t('jobs.commandLabel')} description={t('jobs.commandHint')} required>
                    <Input
                      value={step.commandLine}
                      onChange={(event) => patch(index, { commandLine: event.target.value })}
                      placeholder={t('jobs.commandPlaceholder')}
                      className="font-mono text-xs"
                    />
                  </Field>

                  {/* Les dependances : seules les etapes ecrites avant celle-ci
                    *  sont proposees, ce qui rend un cycle impossible a saisir. */}
                  {index > 0 ? (
                    <Field label={t('flows.dependsOn')}>
                      <div className="flex flex-wrap gap-1.5">
                        {steps
                          .slice(0, index)
                          .filter((candidate) => candidate.key !== '')
                          .map((candidate) => {
                            const held = (step.dependsOn ?? []).includes(candidate.key);
                            return (
                              <button
                                key={candidate.key}
                                type="button"
                                aria-pressed={held}
                                onClick={() => toggleDependency(index, candidate.key)}
                                className={`rounded-full border px-2 py-0.5 font-mono text-xs transition-colors ${
                                  held
                                    ? 'border-brand bg-brand-subtle text-brand-subtle-foreground'
                                    : 'border-border text-muted-foreground hover:border-border-strong'
                                }`}
                              >
                                {candidate.key}
                              </button>
                            );
                          })}
                      </div>
                      {(step.dependsOn ?? []).length === 0 ? (
                        <p className="pt-1 text-xs text-muted-foreground">{t('flows.dependsOnNothing')}</p>
                      ) : null}
                    </Field>
                  ) : (
                    <p className="text-xs text-muted-foreground">{t('flows.dependsOnNothing')}</p>
                  )}
                </div>
              ))}

              <Button type="button" variant="secondary" onClick={addStep} disabled={usable.length === 0}>
                <Plus aria-hidden />
                {t('flows.addStep')}
              </Button>
            </div>

            {columns.length > 1 ? (
              <div className="rounded-md border border-border bg-surface-muted p-3">
                <p className="text-xs font-medium">{t('flows.graph')}</p>
                <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
                  {columns.map((column, index) => (
                    <React.Fragment key={index}>
                      {index > 0 ? <span className="text-muted-foreground">→</span> : null}
                      <span className="rounded bg-surface px-1.5 py-0.5 font-mono">
                        {column.map((step) => step.key).join(' | ')}
                      </span>
                    </React.Fragment>
                  ))}
                </div>
              </div>
            ) : null}

            {duplicated ? (
              <p className="text-xs text-danger-foreground">{t('flows.stepKeyHint')}</p>
            ) : null}
          </SheetBody>

          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" loading={save.isPending} disabled={!valid}>
              {t('common.save')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
