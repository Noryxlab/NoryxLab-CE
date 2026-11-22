import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Infinity as InfinityIcon, Info, Wallet } from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
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
import { adminApi } from '@/lib/api/endpoints';
import { qk, useBudgets, useInvalidate } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatNumber } from '@/lib/format';
import type { BudgetState } from '@/lib/api/types';

/**
 * Les budgets, et ce qu'ils ne font pas.
 *
 * Deux phrases sont affichees en haut plutot qu'enfouies dans une aide : un
 * budget est en vCPU-heures et pas en euros, et un depassement n'arrete rien.
 * Les deux seraient lues de travers autrement - un chiffre sans unite se lit
 * comme une monnaie, et une barre rouge se lit comme une coupure. Un
 * administrateur qui croirait qu'un projet a ete suspendu irait chercher une
 * panne qui n'existe pas.
 */

/** Deux seuils, et un seul endroit qui les connait : ceux que la veille
 *  applique. Les recopier ici en ferait une troisieme source de verite. */
const THRESHOLDS = [75, 100] as const;

function SetBudgetSheet({
  state,
  open,
  onOpenChange,
}: {
  state: BudgetState | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [value, setValue] = React.useState('');

  React.useEffect(() => {
    if (open) setValue(state && state.vcpuHours > 0 ? String(state.vcpuHours) : '');
  }, [open, state]);

  const save = useMutation({
    mutationFn: () => adminApi.setBudget(state?.projectId ?? '', Number(value)),
    onSuccess: () => {
      invalidate(qk.adminBudgets);
      onOpenChange(false);
      toast.success(t('budgets.saved'), t('budgets.title'));
    },
    onError: (error) => toast.error(error, t('budgets.set')),
  });

  const parsed = Number(value);
  const valid = value.trim() !== '' && Number.isFinite(parsed) && parsed > 0;
  /* Zero n'est pas une dotation, c'est le retrait - et le formulaire ne le
   *  fait pas. Sans cette phrase, taper 0 desactive le bouton et n'explique
   *  rien : on reste devant un champ rempli qui refuse d'etre enregistre. */
  const zeroTyped = value.trim() !== '' && Number.isFinite(parsed) && parsed === 0;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (valid) save.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>
              {t('budgets.setTitle', { project: state?.projectName || state?.projectId || '' })}
            </SheetTitle>
            <SheetDescription>{t('budgets.setHint')}</SheetDescription>
          </SheetHeader>

          <SheetBody>
            <Field label={t('budgets.vcpuHoursLabel')} required>
              <Input
                type="number"
                min={1}
                step={1}
                inputMode="numeric"
                value={value}
                onChange={(event) => setValue(event.target.value)}
                placeholder="500"
                className="tabular-nums"
              />
            </Field>

            {/* Ce que la dotation declenche, et ce qu'elle ne declenche pas.
              *  Dit ici, au moment ou on la pose : c'est le seul moment ou la
              *  question « et si on depasse ? » se presente a l'esprit. */}
            <div className="space-y-2 rounded-md border border-border bg-surface-muted p-3">
              <p className="text-xs font-medium">
                {t('budgets.thresholds', { first: String(THRESHOLDS[0]), second: String(THRESHOLDS[1]) })}
              </p>
              <p className="text-xs leading-relaxed text-muted-foreground">{t('budgets.thresholdsHint')}</p>
              <p className="text-xs font-medium">{t('budgets.doesNotStop')}</p>
              <p className="text-xs leading-relaxed text-muted-foreground">{t('budgets.doesNotStopHint')}</p>
            </div>

            {zeroTyped ? (
              <p className="text-xs leading-relaxed text-warning-foreground">
                {t('budgets.zeroIsUnlimited')}
              </p>
            ) : null}

            {state && state.consumed > 0 ? (
              <p className="text-xs text-muted-foreground">
                {t('budgets.consumed')} : <span className="tabular-nums">{state.consumed.toFixed(1)}</span>{' '}
                {t('usage.vcpuHours')}
              </p>
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

export function BudgetsSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const budgets = useBudgets();
  const [editing, setEditing] = React.useState<BudgetState | null>(null);

  const clear = useMutation({
    mutationFn: (projectId: string) => adminApi.setBudget(projectId, 0),
    onSuccess: () => {
      invalidate(qk.adminBudgets);
      toast.success(t('budgets.cleared'), t('budgets.title'));
    },
    onError: (error) => toast.error(error, t('budgets.clear')),
  });

  const columns: Column<BudgetState>[] = [
    {
      id: 'project',
      header: t('common.project'),
      sortValue: (item) => item.projectName ?? item.projectId,
      searchValue: (item) => `${item.projectName ?? ''} ${item.projectId}`,
      cell: (item) => (
        <span className="truncate font-medium">{item.projectName || item.projectId}</span>
      ),
    },
    {
      id: 'allowance',
      header: t('budgets.allowance'),
      sortValue: (item) => item.vcpuHours,
      cell: (item) =>
        item.vcpuHours > 0 ? (
          <span className="tabular-nums">{formatNumber(item.vcpuHours, locale)}</span>
        ) : (
          // L'infini, nomme. Ni un tiret ni « aucune dotation » : l'absence de
          // plafond est le reglage par defaut et un etat choisi, pas une
          // donnee qui manque - et une colonne qui se lit comme une donnee
          // manquante envoie chercher ce qu'il faudrait y mettre.
          <Badge tone="neutral" title={t('budgets.noBudgetHint')}>
            <InfinityIcon className="size-3" aria-hidden />
            {t('budgets.noBudget')}
          </Badge>
        ),
    },
    {
      id: 'consumed',
      header: t('budgets.consumed'),
      sortValue: (item) => item.consumed,
      cell: (item) => <span className="tabular-nums">{item.consumed.toFixed(1)}</span>,
    },
    {
      id: 'share',
      header: t('budgets.share'),
      // Trie sur la part quand il y a une dotation, et sur la consommation
      // sinon : sans ca, les projets sans budget - dont la part vaut zero -
      // remonteraient tous ensemble en tete, y compris celui qui consomme le
      // plus de la plateforme.
      sortValue: (item) => (item.vcpuHours > 0 ? item.percent : -1),
      cell: (item) =>
        item.vcpuHours > 0 ? (
          <div className="min-w-32 space-y-1">
            <div className="flex items-center justify-between gap-2 text-xs">
              <span className="tabular-nums">{item.percent.toFixed(0)} %</span>
              {item.percent >= 100 ? (
                <Badge tone="danger">{t('budgets.overBudget')}</Badge>
              ) : null}
            </div>
            <Progress
              value={Math.min(100, item.percent)}
              label={`${item.percent.toFixed(0)} %`}
              className={item.percent >= 100 ? '[&>div]:bg-danger' : item.percent >= THRESHOLDS[0] ? '[&>div]:bg-warning' : undefined}
            />
          </div>
        ) : (
          // Pas de part d'un infini. Le tiret porte sa raison, parce qu'il est
          // le seul endroit du tableau ou il reste un tiret.
          <span className="text-xs text-muted-foreground" title={t('budgets.unlimitedShare')}>
            —
          </span>
        ),
    },
  ];

  return (
    <section className="space-y-3">
      <SectionHeader title={t('budgets.title')} description={t('budgets.hint')} />

      {/* En tete et non en bas de page : les deux faits qui changent la
        *  lecture de chaque ligne du tableau. */}
      <Card className="flex items-start gap-3 p-3">
        <Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
        <div className="space-y-1.5 text-xs leading-relaxed">
          <p>
            <span className="font-medium">{t('budgets.doesNotStop')}</span> —{' '}
            <span className="text-muted-foreground">{t('budgets.doesNotStopHint')}</span>
          </p>
          <p>
            <span className="font-medium">
              {t('budgets.thresholds', { first: String(THRESHOLDS[0]), second: String(THRESHOLDS[1]) })}
            </span>{' '}
            — <span className="text-muted-foreground">{t('budgets.thresholdsHint')}</span>
          </p>
          <p className="text-muted-foreground">{t('budgets.convertToMoney')}</p>
        </div>
      </Card>

      <Card>
        <DataTable
          data={budgets.data?.items ?? []}
          columns={columns}
          rowKey={(item) => item.projectId}
          isLoading={budgets.isLoading}
          isError={budgets.isError}
          error={budgets.error}
          onRetry={() => void budgets.refetch()}
          defaultSort={{ columnId: 'share', direction: 'desc' }}
          emptyState={
            <EmptyState icon={Wallet} title={t('budgets.empty')} description={t('budgets.emptyHint')} />
          }
          rowActions={(item) => (
            <>
              <DropdownMenuItem onSelect={() => setEditing(item)}>
                <Wallet aria-hidden />
                {item.vcpuHours > 0 ? t('common.edit') : t('budgets.set')}
              </DropdownMenuItem>
              {item.vcpuHours > 0 ? (
                <DropdownMenuItem
                  destructive
                  onSelect={() =>
                    ask({
                      title: t('budgets.clearTitle'),
                      description: t('budgets.clearWarning'),
                      confirmLabel: t('budgets.clear'),
                      destructive: true,
                      onConfirm: () => clear.mutateAsync(item.projectId),
                    })
                  }
                >
                  {t('budgets.clear')}
                </DropdownMenuItem>
              ) : null}
            </>
          )}
        />
      </Card>

      <SetBudgetSheet
        state={editing}
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
      />
      {dialog}
    </section>
  );
}
