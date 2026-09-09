import * as React from 'react';
import { AlertTriangle, Check, Circle, Loader2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { useWorkspaceStartup } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { WorkspaceStartupStep } from '@/lib/api/types';

/**
 * Where a workspace is in its start, and where it stopped.
 *
 * A workspace that does not open used to show "launching" and nothing else. The
 * explanation existed — Kubernetes had recorded that a volume could not be
 * attached — but it lived in `kubectl describe`, which is not a place a
 * researcher goes. Waiting was the only available strategy, and on EMSE people
 * waited eight minutes for something that was never going to happen.
 *
 * Five steps, in the order they occur, with the engine's own words folded away
 * for whoever wants them. Nobody has to read those to know where it stopped.
 */
const ICONS: Record<WorkspaceStartupStep['state'], React.ComponentType<{ className?: string }>> = {
  done: Check,
  running: Loader2,
  failed: AlertTriangle,
  waiting: Circle,
};

export function WorkspaceStartup({ workspaceId, ready }: { workspaceId: string; ready: boolean }) {
  const t = useT();
  // Nothing to explain once it is open: the panel exists for the wait.
  const startup = useWorkspaceStartup(workspaceId, !ready);
  if (ready || !startup.data) return null;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('workspaces.startupTitle')}</CardTitle>
          <CardDescription>
            {startup.data.stuck ? t('workspaces.startupStuck') : t('workspaces.startupHint')}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent>
        <ol className="space-y-2">
          {startup.data.steps.map((step) => {
            const Icon = ICONS[step.state];
            const failed = step.state === 'failed';
            return (
              <li key={step.key} className="flex items-start gap-2.5">
                <Icon
                  aria-hidden
                  className={
                    failed
                      ? 'mt-0.5 size-4 shrink-0 text-warning-foreground'
                      : step.state === 'done'
                        ? 'mt-0.5 size-4 shrink-0 text-success-foreground'
                        : step.state === 'running'
                          ? 'mt-0.5 size-4 shrink-0 animate-spin text-muted-foreground'
                          : 'mt-0.5 size-4 shrink-0 text-muted-foreground opacity-40'
                  }
                />
                <div className="min-w-0">
                  <p className={failed ? 'text-sm font-medium' : 'text-sm'}>
                    {t(`workspaces.step_${step.key}` as 'workspaces.step_requested')}
                  </p>
                  {step.detail ? (
                    <p className="text-xs text-warning-foreground">
                      {t(`workspaces.reason_${step.detail}` as 'workspaces.reason_no_capacity')}
                    </p>
                  ) : null}
                  {/* The engine's words, folded: useful to an operator, noise
                      to everybody else. */}
                  {step.technical ? (
                    <details className="mt-1">
                      <summary className="cursor-pointer text-xs text-muted-foreground">
                        {t('workspaces.startupTechnical')}
                      </summary>
                      <p className="mt-1 break-words font-mono text-xs text-muted-foreground">
                        {step.technical}
                      </p>
                    </details>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ol>
      </CardContent>
    </Card>
  );
}
