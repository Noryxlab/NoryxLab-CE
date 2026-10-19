import { useT } from "@/lib/i18n";
import type {
  WorkflowRun,
  WorkflowRunStatus,
  WorkflowStep,
} from "@/lib/api/types";

/**
 * Ce qu'un run dit de lui-meme, derive comme le fait le serveur.
 *
 * Ce fichier portait la sequence dessinee en ligne - un declencheur, des pas,
 * un rapport, et une fleche entre chacun. Le canvas (workflow-canvas.tsx) l'a
 * remplacee le 2026-10-19 : meme matiere, meme derivation, mais une surface
 * qui se deplace, se zoome et s'ouvre au clic, qui est le vocabulaire que
 * toute personne ayant vu un outil d'automatisation a en tete. Le composant
 * n'avait plus d'appelant et a ete retire plutot que laisse - un mecanisme
 * sans appelant est exactement ce que cette semaine a passe son temps a
 * trouver. Les trois helpers restent : le canvas, l'inspecteur et la fiche
 * de mission en lisent l'etat.
 */

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
