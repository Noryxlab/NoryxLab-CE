import { Bot, ChevronRight, Clock, FileText, UserCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import { scheduleKey } from "./agent-roster";
import type {
  Workflow,
  WorkflowRun,
  WorkflowRunStatus,
  WorkflowStep,
} from "@/lib/api/types";

/**
 * La sequence, dessinee.
 *
 * Un declencheur, des pas, un rapport : trois formes de noeud et une fleche
 * entre chacun. C'est le "workflow graphique" qu'on demande, sans outil de
 * dessin - la sequence est ce que le dossier recommande la ou le controle
 * compte (ADR-046), et une suite de noeuds se lit de gauche a droite par
 * quelqu'un qui n'a jamais ouvert un editeur de graphes.
 *
 * Elle est derivee et jamais dessinee a la main : les noeuds sont les pas, le
 * pas courant vient du dernier run, l'etat de chaque noeud vient de ce que la
 * plateforme a enregistre. Un pas agent porte ses verbes sous son nom, parce
 * que "peut agir" est la premiere chose qu'on veut lire sur une sequence qui
 * tourne la nuit.
 */

interface WorkflowPipelineProps {
  workflow: Workflow;
  latest?: WorkflowRun;
  className?: string;
}

export function WorkflowPipeline({
  workflow,
  latest,
  className,
}: WorkflowPipelineProps) {
  const t = useT();
  const current =
    latest && !isTerminal(latest.status) ? currentStep(latest) : null;
  const finished = latest && latest.status === "succeeded";

  return (
    <ol
      className={cn(
        "scroll-x flex items-stretch gap-1 overflow-x-auto py-1",
        className,
      )}
      aria-label={t("workflows.pipelineTitle")}
    >
      <Node
        icon={Clock}
        eyebrow={t("workflows.triggerNode")}
        title={t(scheduleKey(workflow.schedule))}
        tone={workflow.enabled ? "brand" : "muted"}
      />
      {workflow.steps.map((step) => {
        const stepRun = latest?.steps.find((item) => item.index === step.index);
        const isCurrent = current !== null && current === step.index;
        return (
          <Node
            key={step.index}
            icon={step.kind === "approval" ? UserCheck : Bot}
            eyebrow={`${step.index + 1} · ${stepKindLabel(step, t)}`}
            title={step.name || stepKindLabel(step, t)}
            tone={toneFor(stepRun?.status, isCurrent)}
            current={isCurrent}
            hollow={step.kind === "approval"}
            footer={
              step.kind === "approval" ? (
                <span className="truncate text-[11px] text-muted-foreground">
                  {t("workflows.waitingFor", {
                    name: step.approverUserId ?? "",
                  })}
                </span>
              ) : (
                <Verbs actions={step.actions ?? []} />
              )
            }
          />
        );
      })}
      <Node
        icon={FileText}
        eyebrow={t("workflows.reportNode")}
        title={
          finished
            ? t("workflows.statusSucceeded")
            : latest?.status === "failed"
              ? t("workflows.statusFailed")
              : "…"
        }
        tone={
          finished
            ? "success"
            : latest?.status === "failed"
              ? "danger"
              : "muted"
        }
        last
      />
    </ol>
  );
}

type NodeTone = "brand" | "success" | "warning" | "danger" | "muted";

function Node({
  icon: Icon,
  eyebrow,
  title,
  tone,
  current = false,
  hollow = false,
  last = false,
  footer,
}: {
  icon: React.ComponentType<{ className?: string }>;
  eyebrow: string;
  title: string;
  tone: NodeTone;
  current?: boolean;
  hollow?: boolean;
  last?: boolean;
  footer?: React.ReactNode;
}) {
  return (
    <li className="flex shrink-0 items-center">
      <div
        className={cn(
          "flex w-40 flex-col gap-1 rounded-xl border px-3 py-2.5",
          hollow ? "border-dashed bg-background" : "bg-surface",
          current
            ? "border-brand shadow-[0_0_0_1px_var(--noryx-brand)]"
            : "border-border",
        )}
      >
        <span className="flex items-center gap-1.5 text-[11px] uppercase tracking-wide text-placeholder">
          <span
            className={cn("size-1.5 rounded-full", dotFor(tone))}
            aria-hidden
          />
          <Icon className="size-3.5" aria-hidden />
          <span className="truncate">{eyebrow}</span>
        </span>
        <span className="truncate text-sm font-medium">{title}</span>
        {footer ? (
          <span className="flex min-h-4 items-center">{footer}</span>
        ) : null}
      </div>
      {!last ? (
        <ChevronRight
          className="mx-0.5 size-4 shrink-0 text-border-strong"
          aria-hidden
        />
      ) : null}
    </li>
  );
}

/** Ce qu'un pas peut faire, en deux mots sous son nom. */
function Verbs({ actions }: { actions: string[] }) {
  const t = useT();
  if (actions.length === 0) {
    return (
      <span className="truncate text-[11px] text-placeholder">
        {t("workflows.stepReadsOnly")}
      </span>
    );
  }
  return (
    <span className="flex flex-wrap gap-1">
      {actions.map((action) => (
        <Badge key={action} tone="warning" className="text-[10px]">
          {action === "call_api"
            ? "API"
            : action === "restart_app"
              ? t("agents.actionRestartApp")
              : action}
        </Badge>
      ))}
    </span>
  );
}

function toneFor(
  status: WorkflowRunStatus | undefined,
  current: boolean,
): NodeTone {
  switch (status) {
    case "succeeded":
      return "success";
    case "failed":
      return "danger";
    case "waiting_approval":
      return "warning";
    case "running":
      return "brand";
    default:
      return current ? "brand" : "muted";
  }
}

function dotFor(tone: NodeTone): string {
  switch (tone) {
    case "success":
      return "bg-[var(--noryx-success)]";
    case "warning":
      return "bg-[var(--noryx-warning)]";
    case "danger":
      return "bg-[var(--noryx-danger)]";
    case "brand":
      return "bg-brand";
    default:
      return "bg-border-strong";
  }
}

export function isTerminal(status: WorkflowRunStatus): boolean {
  return (
    status === "succeeded" || status === "failed" || status === "cancelled"
  );
}

/** Le premier pas non fini, derive du run comme le fait le serveur. */
export function currentStep(run: WorkflowRun): number | null {
  const step = run.steps.find((item) => item.status !== "succeeded");
  return step ? step.index : null;
}

type Translate = ReturnType<typeof useT>;

export function stepKindLabel(step: WorkflowStep, t: Translate): string {
  return step.kind === "approval"
    ? t("workflows.stepApproval")
    : t("workflows.stepAgent");
}
