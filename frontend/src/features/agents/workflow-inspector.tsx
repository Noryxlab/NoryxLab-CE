import * as React from "react";
import { ArrowLeft, ArrowRight, Check, Loader2, Trash2, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import { actionTakenLabel, actionsFrom } from "./action-label";
import { scheduleKey } from "./agent-roster";
import { stepKindLabel } from "./workflow-pipeline";
import type { CanvasSelection } from "./workflow-canvas";
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
 *
 * En edition, le meme panneau devient le formulaire du pas choisi : sa sorte,
 * son intitule, sa consigne et ses droits - ou la personne qu'il attend. On
 * l'avance, on le recule, on le retire d'ici. Le declencheur s'ouvre de la
 * meme facon, avec ses horaires.
 */

interface WorkflowInspectorProps {
  workflow: Workflow;
  latest?: WorkflowRun;
  index: CanvasSelection;
  deciding: boolean;
  onDecide: (runId: string, approve: boolean) => void;
  editing?: boolean;
  onChangeStep?: (index: number, patch: Partial<WorkflowStep>) => void;
  onRemoveStep?: (index: number) => void;
  onMoveStep?: (index: number, delta: -1 | 1) => void;
  onChangeSchedule?: (schedule: Workflow["schedule"]) => void;
  className?: string;
}

export function WorkflowInspector({
  workflow,
  latest,
  index,
  deciding,
  onDecide,
  editing = false,
  onChangeStep,
  onRemoveStep,
  onMoveStep,
  onChangeSchedule,
  className,
}: WorkflowInspectorProps) {
  const t = useT();
  const step = typeof index === "number" ? workflow.steps[index] : undefined;

  let body: React.ReactNode;
  if (index === "trigger") {
    body = (
      <TriggerPanel
        workflow={workflow}
        editing={editing}
        onChangeSchedule={onChangeSchedule}
      />
    );
  } else if (!step) {
    body = (
      <p className="m-auto max-w-[24ch] text-center text-sm text-placeholder">
        {editing ? t("workflows.editHint") : t("workflows.selectStep")}
      </p>
    );
  } else if (editing && onChangeStep) {
    body = (
      <StepEditor
        step={step}
        count={workflow.steps.length}
        onChange={(patch) => onChangeStep(step.index, patch)}
        onRemove={onRemoveStep ? () => onRemoveStep(step.index) : undefined}
        onMove={onMoveStep ? (delta) => onMoveStep(step.index, delta) : undefined}
      />
    );
  } else {
    body = (
      <StepDetail
        step={step}
        latest={latest}
        deciding={deciding}
        onDecide={onDecide}
      />
    );
  }

  return (
    <aside
      className={cn(
        "flex min-h-64 flex-col gap-3 rounded-xl border bg-surface p-4",
        editing ? "border-dashed border-brand/60" : "border-border",
        className,
      )}
      aria-label={t("workflows.inspectorTitle")}
    >
      {body}
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

/**
 * Le formulaire d'un pas. Les memes champs que le dialogue d'ecriture, un pas
 * a la fois : c'est le pas choisi sur le canvas qu'on remplit, pas une liste.
 */
function StepEditor({
  step,
  count,
  onChange,
  onRemove,
  onMove,
}: {
  step: WorkflowStep;
  count: number;
  onChange: (patch: Partial<WorkflowStep>) => void;
  onRemove?: () => void;
  onMove?: (delta: -1 | 1) => void;
}) {
  const t = useT();
  const actions = step.actions ?? [];
  const mayRestart = actions.includes("restart_app");
  const mayCallApi = actions.includes("call_api");
  return (
    <>
      <header className="flex items-center justify-between gap-2">
        <p className="text-[11px] uppercase tracking-wide text-placeholder">
          {step.index + 1} · {t("workflows.editing")}
        </p>
        <span className="flex gap-0.5">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onMove?.(-1)}
            disabled={!onMove || step.index === 0}
            title={t("workflows.moveLeft")}
          >
            <ArrowLeft className="size-4" aria-hidden />
            <span className="sr-only">{t("workflows.moveLeft")}</span>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onMove?.(1)}
            disabled={!onMove || step.index === count - 1}
            title={t("workflows.moveRight")}
          >
            <ArrowRight className="size-4" aria-hidden />
            <span className="sr-only">{t("workflows.moveRight")}</span>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={onRemove}
            disabled={!onRemove || count === 1}
            title={t("workflows.removeStep")}
          >
            <Trash2 className="size-4" aria-hidden />
            <span className="sr-only">{t("workflows.removeStep")}</span>
          </Button>
        </span>
      </header>

      <Field label={t("workflows.stepKindLabel")}>
        <div className="flex gap-2">
          {(["agent", "approval"] as const).map((kind) => (
            <Chip
              key={kind}
              active={step.kind === kind}
              onClick={() => onChange({ kind })}
            >
              {kind === "agent" ? t("workflows.stepAgent") : t("workflows.stepApproval")}
            </Chip>
          ))}
        </div>
      </Field>

      <Field label={t("workflows.stepNameLabel")}>
        <Input
          value={step.name}
          onChange={(event) => onChange({ name: event.target.value })}
          placeholder={t("workflows.stepNamePlaceholder")}
          maxLength={60}
        />
      </Field>

      {step.kind === "agent" ? (
        <>
          <Field label={t("workflows.stepInstructionLabel")} required>
            <Textarea
              value={step.instruction ?? ""}
              onChange={(event) => onChange({ instruction: event.target.value })}
              rows={4}
              maxLength={4000}
              placeholder={t("workflows.stepInstructionPlaceholder")}
            />
          </Field>
          <div className="flex flex-col gap-2">
            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              <Switch
                checked={mayRestart}
                onCheckedChange={(checked) =>
                  onChange({ actions: actionsFrom(checked, mayCallApi) })
                }
              />
              {t("workflows.stepMayRestart")}
            </label>
            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              <Switch
                checked={mayCallApi}
                onCheckedChange={(checked) =>
                  onChange({ actions: actionsFrom(mayRestart, checked) })
                }
              />
              {t("workflows.stepMayCallApi")}
            </label>
          </div>
        </>
      ) : (
        <Field
          label={t("workflows.stepApproverLabel")}
          description={t("workflows.stepApproverHelp")}
          required
        >
          <Input
            value={step.approverUserId ?? ""}
            onChange={(event) => onChange({ approverUserId: event.target.value })}
            maxLength={120}
          />
        </Field>
      )}
    </>
  );
}

/** Le declencheur, ouvert : ses horaires, a lire ou a changer. */
function TriggerPanel({
  workflow,
  editing,
  onChangeSchedule,
}: {
  workflow: Workflow;
  editing: boolean;
  onChangeSchedule?: (schedule: Workflow["schedule"]) => void;
}) {
  const t = useT();
  return (
    <>
      <header>
        <p className="text-[11px] uppercase tracking-wide text-placeholder">
          {t("workflows.triggerNode")}
        </p>
        <h4 className="text-sm font-semibold">{t("workflows.triggerTitle")}</h4>
      </header>
      <p className="text-sm leading-relaxed text-muted-foreground">
        {t("workflows.triggerHelp")}
      </p>
      <Section title={t("workflows.scheduleLabel")}>
        <div className="flex flex-wrap gap-2">
          {(["manual", "hourly", "daily"] as const).map((option) => (
            <Chip
              key={option}
              active={workflow.schedule === option}
              onClick={editing && onChangeSchedule ? () => onChangeSchedule(option) : undefined}
            >
              {t(scheduleKey(option))}
            </Chip>
          ))}
        </div>
      </Section>
      {!workflow.enabled ? (
        <Badge tone="outline" className="w-fit">
          {t("agents.paused")}
        </Badge>
      ) : null}
    </>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick?: () => void;
  children: React.ReactNode;
}) {
  const base = active
    ? "rounded-lg border border-brand bg-brand-subtle px-3 py-1.5 text-sm text-brand-subtle-foreground"
    : "rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground";
  if (!onClick) return <span className={base}>{children}</span>;
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(base, "transition hover:border-border-strong")}
    >
      {children}
    </button>
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
