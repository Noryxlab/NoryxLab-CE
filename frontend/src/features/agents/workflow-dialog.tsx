import * as React from "react";
import { useMutation } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, Trash2 } from "lucide-react";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { actionsFrom } from "./action-label";
import { scheduleKey } from "./agent-roster";
import { platformApi } from "@/lib/api/endpoints";
import { qk, useInvalidate } from "@/lib/api/queries";
import { useToast } from "@/components/ui/toast";
import { useT } from "@/lib/i18n";
import type { Workflow, WorkflowInput } from "@/lib/api/types";

/**
 * Ecrire un workflow.
 *
 * Le formulaire est une suite de pas, et rien d'autre : un nom, des horaires,
 * puis les pas dans l'ordre ou ils se suivent. Pas de graphe, pas de
 * condition, pas d'expression - la sequence est ce que le dossier recommande
 * la ou le controle compte, et c'est aussi ce qu'une personne sait ecrire sans
 * apprendre un outil.
 *
 * Un pas agent se remplit comme une fiche de poste, avec le meme interrupteur
 * de droit d'agir que l'agent. Un pas d'approbation ne demande qu'une chose :
 * qui. Cette personne seule pourra laisser passer ou arreter, et elle est
 * choisie ici, a l'ecriture, pas au moment ou le run attend.
 */

type Draft = {
  kind: "agent" | "approval";
  name: string;
  instruction: string;
  approverUserId: string;
  mayRestart: boolean;
  mayCallApi: boolean;
};

const EMPTY_AGENT: Draft = {
  kind: "agent",
  name: "",
  instruction: "",
  approverUserId: "",
  mayRestart: false,
  mayCallApi: false,
};
const EMPTY_APPROVAL: Draft = { ...EMPTY_AGENT, kind: "approval" };

export function WorkflowDialog({
  open,
  onOpenChange,
  projectId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string | undefined;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const [name, setName] = React.useState("");
  const [schedule, setSchedule] =
    React.useState<Workflow["schedule"]>("manual");
  const [steps, setSteps] = React.useState<Draft[]>([{ ...EMPTY_AGENT }]);

  React.useEffect(() => {
    if (!open) return;
    setName("");
    setSchedule("manual");
    setSteps([{ ...EMPTY_AGENT }]);
  }, [open]);

  const update = (index: number, patch: Partial<Draft>) =>
    setSteps((current) =>
      current.map((step, at) => (at === index ? { ...step, ...patch } : step)),
    );
  const remove = (index: number) =>
    setSteps((current) => current.filter((_, at) => at !== index));
  const move = (index: number, delta: -1 | 1) =>
    setSteps((current) => {
      const target = index + delta;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      [next[index], next[target]] = [
        next[target] as Draft,
        next[index] as Draft,
      ];
      return next;
    });

  const save = useMutation({
    mutationFn: () => {
      const body: WorkflowInput = {
        projectId,
        name: name.trim(),
        schedule,
        steps: steps.map((step) =>
          step.kind === "agent"
            ? {
                kind: "agent",
                name: step.name.trim(),
                instruction: step.instruction.trim(),
                actions: actionsFrom(step.mayRestart, step.mayCallApi),
              }
            : {
                kind: "approval",
                name: step.name.trim(),
                approverUserId: step.approverUserId.trim(),
              },
        ),
      };
      return platformApi.createWorkflow(body);
    },
    onSuccess: () => {
      invalidate(qk.workflows);
      onOpenChange(false);
      toast.success(
        t("workflows.written", { name: name.trim() }),
        t("workflows.title"),
      );
    },
    onError: (error) => toast.error(error, t("workflows.title")),
  });

  const ready =
    name.trim().length > 0 &&
    Boolean(projectId) &&
    steps.length > 0 &&
    steps.every((step) =>
      step.kind === "agent"
        ? step.instruction.trim().length > 0
        : step.approverUserId.trim().length > 0,
    );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{t("workflows.editTitle")}</DialogTitle>
          <DialogDescription>{t("workflows.createIntro")}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-5">
          <Field label={t("workflows.nameLabel")} required>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("workflows.namePlaceholder")}
              maxLength={80}
              autoFocus
            />
          </Field>

          <Field label={t("workflows.scheduleLabel")}>
            <div className="flex flex-wrap gap-2">
              {(["manual", "hourly", "daily"] as const).map((option) => (
                <button
                  key={option}
                  type="button"
                  aria-pressed={schedule === option}
                  onClick={() => setSchedule(option)}
                  className={
                    schedule === option
                      ? "rounded-lg border border-brand bg-brand-subtle px-3 py-1.5 text-sm text-brand-subtle-foreground"
                      : "rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground transition hover:border-border-strong"
                  }
                >
                  {t(scheduleKey(option))}
                </button>
              ))}
            </div>
          </Field>

          <Field
            label={t("workflows.stepsLabel")}
            description={t("workflows.stepsHelp")}
          >
            <ol className="space-y-3">
              {steps.map((step, index) => (
                <li
                  key={index}
                  className="rounded-lg border border-border bg-surface p-3"
                >
                  <div className="flex items-center gap-2">
                    <span className="w-5 text-xs text-placeholder tabular-nums">
                      {index + 1}
                    </span>
                    <Badge
                      tone={step.kind === "approval" ? "outline" : "neutral"}
                    >
                      {step.kind === "approval"
                        ? t("workflows.stepApproval")
                        : t("workflows.stepAgent")}
                    </Badge>
                    <Input
                      value={step.name}
                      onChange={(event) =>
                        update(index, { name: event.target.value })
                      }
                      placeholder={t("workflows.stepNamePlaceholder")}
                      maxLength={60}
                      className="flex-1"
                      aria-label={t("workflows.stepNameLabel")}
                    />
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => move(index, -1)}
                      disabled={index === 0}
                      title={t("workflows.moveUp")}
                    >
                      <ArrowUp className="size-4" aria-hidden />
                      <span className="sr-only">{t("workflows.moveUp")}</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => move(index, 1)}
                      disabled={index === steps.length - 1}
                      title={t("workflows.moveDown")}
                    >
                      <ArrowDown className="size-4" aria-hidden />
                      <span className="sr-only">{t("workflows.moveDown")}</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => remove(index)}
                      disabled={steps.length === 1}
                      title={t("workflows.removeStep")}
                    >
                      <Trash2 className="size-4" aria-hidden />
                      <span className="sr-only">
                        {t("workflows.removeStep")}
                      </span>
                    </Button>
                  </div>

                  {step.kind === "agent" ? (
                    <div className="mt-3 space-y-3">
                      <Textarea
                        value={step.instruction}
                        onChange={(event) =>
                          update(index, { instruction: event.target.value })
                        }
                        rows={3}
                        maxLength={4000}
                        placeholder={t("workflows.stepInstructionPlaceholder")}
                        aria-label={t("workflows.stepInstructionLabel")}
                      />
                      <div className="flex flex-wrap gap-x-5 gap-y-2">
                        <label className="flex items-center gap-2 text-xs text-muted-foreground">
                          <Switch
                            checked={step.mayRestart}
                            onCheckedChange={(checked) =>
                              update(index, { mayRestart: checked })
                            }
                          />
                          {t("workflows.stepMayRestart")}
                        </label>
                        <label className="flex items-center gap-2 text-xs text-muted-foreground">
                          <Switch
                            checked={step.mayCallApi}
                            onCheckedChange={(checked) =>
                              update(index, { mayCallApi: checked })
                            }
                          />
                          {t("workflows.stepMayCallApi")}
                        </label>
                      </div>
                    </div>
                  ) : (
                    <div className="mt-3">
                      <Field
                        label={t("workflows.stepApproverLabel")}
                        description={t("workflows.stepApproverHelp")}
                      >
                        <Input
                          value={step.approverUserId}
                          onChange={(event) =>
                            update(index, {
                              approverUserId: event.target.value,
                            })
                          }
                          maxLength={120}
                        />
                      </Field>
                    </div>
                  )}
                </li>
              ))}
            </ol>
            <div className="mt-3 flex flex-wrap gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  setSteps((current) => [...current, { ...EMPTY_AGENT }])
                }
              >
                {t("workflows.addAgentStep")}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  setSteps((current) => [...current, { ...EMPTY_APPROVAL }])
                }
              >
                {t("workflows.addApprovalStep")}
              </Button>
            </div>
          </Field>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            onClick={() => save.mutate()}
            disabled={!ready || save.isPending}
          >
            {t("workflows.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
