import {
  Bot,
  Check,
  Hourglass,
  ShieldCheck,
  Workflow as WorkflowIcon,
  X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Stat, StatGrid } from "@/components/common/stat";
import { relativeTime } from "./agent-roster";
import { useT } from "@/lib/i18n";
import type { Agent, Workflow, WorkflowRun } from "@/lib/api/types";

/**
 * Le bandeau, et ce qui attend quelqu'un.
 *
 * Quatre chiffres qu'on lit en arrivant : qui travaille, ce qui tourne, ce
 * qui attend une decision, qui a le droit d'agir. Le troisieme est le seul qui
 * demande quelque chose au lecteur, et il a sa boite juste en dessous : un run
 * gare sur une approbation ne fait rien tant que personne ne tranche, et le
 * trouver au fond d'un onglet est la facon la plus sure de l'oublier.
 */

export interface WaitingRun {
  workflow: Workflow;
  run: WorkflowRun;
}

export function AgentsOverview({
  agents,
  workflows,
  waiting,
  loading,
}: {
  agents: Agent[];
  workflows: Workflow[];
  waiting: WaitingRun[];
  loading: boolean;
}) {
  const t = useT();
  const granted =
    agents.filter((agent) => agent.actions.length > 0).length +
    workflows.reduce(
      (count, workflow) =>
        count +
        workflow.steps.filter((step) => (step.actions ?? []).length > 0).length,
      0,
    );
  const readers =
    agents.filter((agent) => agent.actions.length === 0).length +
    workflows.reduce(
      (count, workflow) =>
        count +
        workflow.steps.filter(
          (step) => step.kind === "agent" && (step.actions ?? []).length === 0,
        ).length,
      0,
    );
  return (
    <StatGrid>
      <Stat
        icon={Bot}
        label={t("agents.statAgents")}
        value={agents.length}
        hint={t("agents.statAgentsHint", {
          active: agents.filter((agent) => agent.enabled).length,
          total: agents.length,
        })}
        loading={loading}
      />
      <Stat
        icon={WorkflowIcon}
        label={t("agents.statWorkflows")}
        value={workflows.length}
        hint={t("agents.statWorkflowsHint", {
          active: workflows.filter((workflow) => workflow.enabled).length,
          total: workflows.length,
        })}
        loading={loading}
      />
      <Stat
        icon={Hourglass}
        label={t("agents.statDecisions")}
        value={waiting.length}
        hint={
          waiting.length > 0
            ? t("agents.statDecisionsHint")
            : t("agents.statDecisionsNone")
        }
        loading={loading}
        className={
          waiting.length > 0 ? "border-[var(--noryx-warning)]/50" : undefined
        }
      />
      <Stat
        icon={ShieldCheck}
        label={t("agents.statGrants")}
        value={granted}
        hint={t("agents.statGrantsHint", { granted, readers })}
        loading={loading}
      />
    </StatGrid>
  );
}

export function DecisionInbox({
  waiting,
  deciding,
  onDecide,
}: {
  waiting: WaitingRun[];
  deciding: boolean;
  onDecide: (runId: string, approve: boolean) => void;
}) {
  const t = useT();
  if (waiting.length === 0) return null;
  return (
    <section className="rounded-xl border border-[var(--noryx-warning)]/50 bg-warning-subtle/40 p-4">
      <div className="flex items-center gap-2">
        <Hourglass className="size-4 text-warning-foreground" aria-hidden />
        <h2 className="text-sm font-semibold">{t("agents.inboxTitle")}</h2>
        <Badge tone="warning">{waiting.length}</Badge>
      </div>
      <p className="mt-1 max-w-prose text-xs text-muted-foreground">
        {t("agents.inboxIntro")}
      </p>
      <ul className="mt-3 space-y-2">
        {waiting.map(({ workflow, run }) => {
          const stepRun = run.steps.find(
            (item) => item.status === "waiting_approval",
          );
          const step = stepRun ? workflow.steps[stepRun.index] : undefined;
          const previous =
            stepRun && stepRun.index > 0
              ? run.steps[stepRun.index - 1]?.output
              : undefined;
          return (
            <li
              key={run.id}
              className="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-border bg-background px-3 py-2.5"
            >
              <div className="min-w-0 flex-1">
                <p className="flex flex-wrap items-baseline gap-2">
                  <span className="text-sm font-medium">{workflow.name}</span>
                  <span className="text-xs text-placeholder">
                    {t("agents.inboxStep", {
                      index: (stepRun?.index ?? 0) + 1,
                      step: step?.name || t("workflows.stepApproval"),
                    })}
                  </span>
                  <span className="text-xs text-placeholder tabular-nums">
                    {relativeTime(run.startedAt, t)}
                  </span>
                </p>
                {previous ? (
                  <p className="mt-1 line-clamp-3 text-sm leading-relaxed text-muted-foreground whitespace-pre-line">
                    {previous}
                  </p>
                ) : null}
              </div>
              <div className="flex shrink-0 gap-2">
                <Button
                  size="sm"
                  onClick={() => onDecide(run.id, true)}
                  disabled={deciding}
                >
                  <Check className="size-4" aria-hidden />
                  {t("workflows.approve")}
                </Button>
                <Button
                  size="sm"
                  variant="danger-outline"
                  onClick={() => onDecide(run.id, false)}
                  disabled={deciding}
                >
                  <X className="size-4" aria-hidden />
                  {t("workflows.reject")}
                </Button>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
