import * as React from "react";
import { Check, Loader2, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useT } from "@/lib/i18n";
import { actionTakenLabel } from "./action-label";
import { stepKindLabel } from "./workflow-pipeline";
import type {
  Workflow,
  WorkflowRun,
  WorkflowRunStatus,
  WorkflowStep,
} from "@/lib/api/types";

/**
 * Le pas, ouvert.
 *
 * Ce que montre un inspecteur de noeud partout ailleurs : ce qui est entre,
 * ce qui est sorti, ce qui s'est passe. Ici, dans l'ordre ou l'on pose les
 * questions - sa consigne, ou il en est, ce qu'il a produit, ce que la
 * plateforme l'a vu faire, et pourquoi il a echoue s'il a echoue.
 *
 * Les actions sont ce que la plateforme a enregistre, jamais ce que le modele
 * a raconte : c'est la seule partie d'un run qui ne peut pas se tromper avec
 * aplomb. Et un pas qui attend une personne porte ses deux boutons ici, a
 * l'endroit ou l'on vient de lire ce qu'il y a a approuver.
 */

interface WorkflowInspectorProps {
  workflow: Workflow;
  latest?: WorkflowRun;
  index: number | null;
  deciding: boolean;
  onDecide: (runId: string, approve: boolean) => void;
  className?: string;
}

export function WorkflowInspector({
  workflow,
  latest,
  index,
  deciding,
  onDecide,
  className,
}: WorkflowInspectorProps) {
  const t = useT();
  const step = index === null ? undefined : workflow.steps[index];

  return (
    <aside
      className={[
        "flex min-h-64 flex-col gap-3 rounded-xl border border-border bg-surface p-4",
        className ?? "",
      ].join(" ")}
      aria-label={t("workflows.inspectorTitle")}
    >
      {!step ? (
        <p className="m-auto max-w-[22ch] text-center text-sm text-placeholder">
          {t("workflows.selectStep")}
        </p>
      ) : (
        <StepDetail
          step={step}
          latest={latest}
          deciding={deciding}
          onDecide={onDecide}
        />
      )}
    </aside>
  );
}

function StepDetail({
  step,
  latest,
  deciding,
  onDecide,
}: {
  step: WorkflowStep;
  latest?: WorkflowRun;
  deciding: boolean;
  onDecide: (runId: string, approve: boolean) => void;
}) {
  const t = useT();
  const index = step.index;
  const run = latest?.steps.find((item) => item.index === index);
  const waiting = latest && run?.status === "waiting_approval";

  return (
    <>
      <header>
        <p className="text-[11px] uppercase tracking-wide text-placeholder">
          {index + 1} · {stepKindLabel(step, t)}
        </p>
        <h4 className="text-sm font-semibold">{step.name || stepKindLabel(step, t)}</h4>
      </header>

      <Section title={t("workflows.stepStatusTitle")}>
        {run ? (
          <span className="flex flex-wrap items-center gap-2">
            <StatusBadge status={run.status} />
            {run.attempts > 1 ? (
              <span className="text-xs text-muted-foreground">
                {t("workflows.attempts", { count: run.attempts })}
              </span>
            ) : null}
          </span>
        ) : (
          <span className="text-sm text-placeholder">{t("workflows.stepNoRun")}</span>
        )}
      </Section>

      {step.kind === "agent" ? (
        <Section title={t("workflows.stepInstructionTitle")}>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
            {step.instruction}
          </p>
        </Section>
      ) : (
        <Section title={t("workflows.stepApproverLabel")}>
          <p className="text-sm text-muted-foreground">
            {t("workflows.waitingFor", { name: step.approverUserId ?? "" })}
          </p>
        </Section>
      )}

      {run?.output ? (
        <Section title={t("workflows.stepOutput")}>
          <p className="max-h-40 overflow-y-auto whitespace-pre-wrap text-sm leading-relaxed">
            {run.output}
          </p>
        </Section>
      ) : null}

      {run && run.actions.length > 0 ? (
        <Section title={t("workflows.stepActions")}>
          <ul className="space-y-1 text-sm">
            {run.actions.map((action, i) => (
              <li key={`${action}-${i}`} className="flex items-start gap-2">
                <Check className="mt-0.5 size-3.5 shrink-0 text-[var(--noryx-success)]" aria-hidden />
                <span>{actionTakenLabel(action, t)}</span>
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {run?.error && run.status !== "succeeded" ? (
        <Section title={t("workflows.stepError")}>
          <p className="whitespace-pre-wrap text-sm text-[var(--noryx-danger)]">{run.error}</p>
        </Section>
      ) : null}

      {waiting && latest ? (
        <div className="mt-auto flex gap-2 border-t border-border pt-3">
          <Button
            size="sm"
            onClick={() => onDecide(latest.id, true)}
            disabled={deciding}
          >
            {deciding ? <Loader2 className="size-4 animate-spin" aria-hidden /> : <Check className="size-4" aria-hidden />}
            {t("workflows.approve")}
          </Button>
          <Button
            size="sm"
            variant="danger-outline"
            onClick={() => onDecide(latest.id, false)}
            disabled={deciding}
          >
            <X className="size-4" aria-hidden />
            {t("workflows.reject")}
          </Button>
        </div>
      ) : null}
    </>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <p className="mb-1 text-[11px] uppercase tracking-wide text-placeholder">{title}</p>
      {children}
    </section>
  );
}

function StatusBadge({ status }: { status: WorkflowRunStatus }) {
  const t = useT();
  const [tone, key] = (
    {
      pending: ["neutral", "workflows.statusPending"],
      running: ["brand", "workflows.statusRunning"],
      waiting_approval: ["warning", "workflows.statusWaiting"],
      succeeded: ["success", "workflows.statusSucceeded"],
      failed: ["danger", "workflows.statusFailed"],
      cancelled: ["outline", "workflows.statusCancelled"],
    } as const
  )[status];
  return <Badge tone={tone}>{t(key)}</Badge>;
}
