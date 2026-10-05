import type { useT } from "@/lib/i18n";

/**
 * Les actions d'un agent, telles qu'on les montre.
 *
 * Deux verbes seulement : relancer une application, et appeler l'API de la
 * plateforme. Le second est enregistre avec ce qui a ete appele - "call_api
 * POST /api/v1/projects/abc/jobs" - parce qu'un badge qui dit "appel API" sans
 * dire lequel ne renseigne personne.
 */

type Translate = ReturnType<typeof useT>;

export const AGENT_ACTIONS = ["restart_app", "call_api"] as const;
export type AgentAction = (typeof AGENT_ACTIONS)[number];

/** Ce qu'un agent a fait, d'apres son carnet. */
export function actionTakenLabel(action: string, t: Translate): string {
  if (action === "restart_app") return t("agents.actionRestartApp");
  if (action.startsWith("call_api")) {
    const call = action.slice("call_api".length).trim();
    return t("agents.actionCallApi", { call: call || "API" });
  }
  return action;
}

/** Ce qu'un agent a le droit de faire, d'apres sa fiche. */
export function grantLabel(action: string, t: Translate): string {
  if (action === "restart_app") return t("agents.mayRestart");
  if (action === "call_api") return t("agents.mayCallApi");
  return action;
}

/** Ce qu'un responsable peut demander, dans une phrase de mandat. */
export function askActionLabel(action: string, t: Translate): string {
  if (action === "restart_app") return t("agents.askActionRestartApp");
  if (action === "call_api") return t("agents.askActionCallApi");
  return action;
}

/** Les actions d'une fiche, depuis deux interrupteurs. */
export function actionsFrom(
  mayRestart: boolean,
  mayCallApi: boolean,
): AgentAction[] {
  const actions: AgentAction[] = [];
  if (mayRestart) actions.push("restart_app");
  if (mayCallApi) actions.push("call_api");
  return actions;
}
