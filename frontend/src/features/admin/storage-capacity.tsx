import { HardDrive } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useStorageCapacity } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import type { StorageCapacityNode } from '@/lib/api/types';

/**
 * Ce que le cluster peut encore accepter comme volume.
 *
 * Kubernetes ne repond pas a cette question : une demande de volume est
 * acceptee a vue et echoue bien plus tard, au moment de l'attachement, sur un
 * pod que personne ne lit. Une installation peut donc etre a un workspace de
 * refuser tout le monde sans que rien ne l'affiche nulle part. C'est l'ecran
 * qui l'affiche.
 *
 * Deux nombres cote a cote, jamais fusionnes : ce que les volumes ont
 * *reserve*, et le disque reellement inutilise. Ils divergent enormement - un
 * cluster peut etre vide a 78% et incapable de placer un volume de plus - et
 * un administrateur a qui l'on ne montre que le second est rassure exactement
 * le matin ou il faudrait l'alerter.
 */
export function StorageCapacityPanel() {
  const t = useT();
  const { locale } = useI18n();
  const capacity = useStorageCapacity();

  const report = capacity.data;
  const nodes = report?.nodes ?? [];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('storageCapacity.title')}</CardTitle>
          <CardDescription>{t('storageCapacity.intro')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        {capacity.isLoading ? (
          <Skeleton className="h-10 w-full" />
        ) : !report?.available ? (
          // Pas de jauge dessinee a partir de zeros : elle se lirait comme un
          // cluster vide, soit l'inverse de ce qui est rapporte.
          <p className="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
            {report?.detail || t('storageCapacity.unavailable')}
          </p>
        ) : (
          <>
            <div className="flex flex-wrap items-baseline gap-x-6 gap-y-1 text-sm">
              <span>
                <span className="font-medium tabular-nums">{bytes(report.totalClaimed, locale)}</span>{' '}
                <span className="text-muted-foreground">
                  {t('storageCapacity.claimedOf').replace('{total}', bytes(report.totalAllocatable, locale))}
                </span>
              </span>
              <span className="text-muted-foreground">
                {t('storageCapacity.freeDisk').replace('{free}', bytes(report.totalFreeDisk, locale))}
              </span>
            </div>

            <ul className="space-y-3">
              {nodes.map((node) => (
                <NodeBar key={node.name} node={node} warnBelow={report.warnBelow} />
              ))}
            </ul>

            <p className="max-w-prose text-xs text-muted-foreground">{t('storageCapacity.explain')}</p>
          </>
        )}
      </CardContent>
    </Card>
  );
}

function NodeBar({ node, warnBelow }: { node: StorageCapacityNode; warnBelow: number }) {
  const t = useT();
  const { locale } = useI18n();
  const used = node.allocatable > 0 ? node.claimed / node.allocatable : 0;
  const tight = node.headroomRatio <= warnBelow;
  const critical = node.headroomRatio <= 0.03;

  return (
    <li className="space-y-1.5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="flex items-center gap-2 text-sm font-medium">
          <HardDrive className="size-4 text-muted-foreground" aria-hidden />
          {node.name}
          {critical ? (
            <Badge tone="danger">{t('storageCapacity.full')}</Badge>
          ) : tight ? (
            <Badge tone="warning">{t('storageCapacity.tight')}</Badge>
          ) : null}
        </span>
        <span className="text-xs tabular-nums text-muted-foreground">
          {t('storageCapacity.leftToAllocate').replace('{left}', bytes(node.schedulable, locale))}
        </span>
      </div>
      {/* La barre mesure ce qui est reserve, pas ce qui est occupe : c'est la
          reservation qui decide si un volume de plus peut etre place. */}
      <div className="h-2 w-full overflow-hidden rounded-full bg-surface-muted" role="presentation">
        <div
          className={`h-full rounded-full ${
            critical ? 'bg-[var(--noryx-danger)]' : tight ? 'bg-[var(--noryx-warning)]' : 'bg-brand'
          }`}
          style={{ width: `${Math.min(100, Math.round(used * 100))}%` }}
        />
      </div>
      <p className="text-xs text-muted-foreground">
        {t('storageCapacity.nodeDetail')
          .replace('{claimed}', bytes(node.claimed, locale))
          .replace('{allocatable}', bytes(node.allocatable, locale))
          .replace('{free}', bytes(node.freeDisk, locale))}
      </p>
    </li>
  );
}

/** Une taille comme un administrateur la dit a voix haute. */
function bytes(value: number, locale: 'fr' | 'en'): string {
  if (!Number.isFinite(value) || value < 1024) return `${Math.max(0, Math.round(value))} B`;
  const units = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  let amount = value / 1024;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  const rounded = amount >= 100 ? Math.round(amount) : Math.round(amount * 10) / 10;
  return `${rounded.toLocaleString(locale)} ${units[unit]}`;
}
