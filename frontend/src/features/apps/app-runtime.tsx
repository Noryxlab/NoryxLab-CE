import * as React from 'react';
import { Info } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { CopyButton } from '@/components/common/copy-button';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useEnvironments, useHardwareTiers } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { presentTier } from '@/lib/presenters';
import type { App, Environment, HardwareTier } from '@/lib/api/types';

/**
 * Ce sur quoi une application tourne, et ce qu'elle execute.
 *
 * La tuile disait son nom, son adresse et son etat, et rien de ce qui la fait
 * tourner : ni l'environnement, ni la taille de machine, ni la commande. Le
 * 2026-11-23 l'application de comptabilite est morte parce que l'image que son
 * etiquette designe avait change d'utilisateur et d'interpreteur entre deux
 * demarrages - et le seul moyen de voir sur quelle image elle partait a ete de
 * lire la base.
 *
 * Deux choses sont donc dites ici, et la seconde est la plus utile : l'image
 * **par son etiquette**, et le fait qu'une etiquette ne garantit rien.
 */

/** L'environnement qui publie cette image, quand il y en a un. Resolu sur
 *  l'image complete : c'est ce que le lancement a reellement reçu, et deux
 *  environnements peuvent porter le meme nom court. */
function environmentFor(app: App, environments: Environment[] | undefined): Environment | undefined {
  return (environments ?? []).find((item) => item.destinationImage === app.image);
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1 border-b border-border py-2.5 last:border-0 sm:grid-cols-[10rem_1fr] sm:gap-4">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-sm">{children}</dd>
    </div>
  );
}

/** La commande telle qu'elle est lancee, et non celle qui a ete tapee.
 *
 *  La plateforme garde le script qu'elle a genere, pas la ligne de son auteur,
 *  donc c'est le script qui est montre - et c'est de toute facon lui qui
 *  s'execute. Dire laquelle des deux on regarde evite de chercher une
 *  difference qui est voulue. */
function LaunchCommand({ app }: { app: App }) {
  const t = useT();
  const command = (app.command ?? []).join(' ');
  const script = (app.args ?? []).join('\n');
  if (!command && !script) {
    return <p className="text-xs text-muted-foreground">{t('apps.runtimeNoCommand')}</p>;
  }
  return (
    <div className="space-y-2">
      <div className="flex items-start gap-2">
        <code className="min-w-0 flex-1 break-all rounded bg-surface-muted px-1.5 py-1 font-mono text-xs">
          {command}
        </code>
        <CopyButton value={command} size="sm" variant="ghost" />
      </div>
      {script ? (
        <>
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs text-muted-foreground">{t('apps.runtimeScriptHint')}</p>
            <CopyButton value={script} size="sm" variant="ghost" />
          </div>
          <pre className="max-h-72 overflow-auto rounded border border-border bg-surface-muted p-2 font-mono text-xs leading-relaxed">
            {script}
          </pre>
        </>
      ) : null}
    </div>
  );
}

export function AppRuntimeSheet({
  app,
  open,
  onOpenChange,
}: {
  app: App | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const environments = useEnvironments();
  const tiers = useHardwareTiers();

  if (!app) return null;

  const environment = environmentFor(app, environments.data);
  const tier = (tiers.data ?? []).find((item: HardwareTier) => item.id === app.hardwareTier);
  const presented = tier ? presentTier(tier, locale) : null;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent size="lg" aria-describedby={undefined}>
        <SheetHeader>
          <SheetTitle>{t('apps.runtimeTitle')}</SheetTitle>
          <SheetDescription>{app.name || app.slug}</SheetDescription>
        </SheetHeader>

        <SheetBody>
          <dl>
            <Row label={t('workspaces.environmentLabel')}>
              {environment ? (
                <p className="font-medium">{environment.name}</p>
              ) : (
                // Pas d'environnement du catalogue : l'image a ete donnee
                // directement, ou l'environnement a disparu depuis. Les deux
                // se lisent sur l'image, donc on la montre et on le dit.
                <p className="text-muted-foreground">{t('apps.runtimeNoEnvironment')}</p>
              )}
              <div className="flex items-start gap-2 pt-0.5">
                <code className="min-w-0 flex-1 break-all font-mono text-xs">{app.image}</code>
                <CopyButton value={app.image} size="sm" variant="ghost" />
              </div>
              {/* L'avertissement qui manquait le 2026-11-23. */}
              {app.image.includes('@') ? null : (
                <p className="flex items-start gap-1.5 pt-1 text-xs text-muted-foreground">
                  <Info className="mt-px size-3.5 shrink-0" aria-hidden />
                  {t('apps.runtimeTagMoves')}
                </p>
              )}
            </Row>

            <Row label={t('workspaces.tierLabel')}>
              {presented ? (
                <>
                  <p className="font-medium">{presented.name}</p>
                  <p className="text-xs text-muted-foreground">{presented.specs}</p>
                </>
              ) : app.hardwareTier ? (
                // La taille est enregistree et la plateforme ne l'offre plus :
                // le dire, parce qu'un redemarrage retombera sur la valeur par
                // defaut sans que personne l'ait demande.
                <p className="text-warning-foreground">
                  {t('apps.runtimeTierGone', { tier: app.hardwareTier })}
                </p>
              ) : (
                <p className="text-muted-foreground">{t('apps.runtimeTierDefault')}</p>
              )}
            </Row>

            <Row label={t('apps.runtimeCommand')}>
              <LaunchCommand app={app} />
            </Row>

            <Row label={t('apps.runtimePort')}>
              <span className="tabular-nums">{app.port}</span>
            </Row>

            <Row label={t('apps.runtimePod')}>
              {app.podName ? (
                <code className="break-all font-mono text-xs">{app.podName}</code>
              ) : (
                // Vu en vrai : une fiche qui avait perdu son nom de pod, donc
                // une application que le redemarrage ne pouvait plus relancer.
                <Badge tone="danger">{t('apps.runtimePodMissing')}</Badge>
              )}
            </Row>
          </dl>
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
