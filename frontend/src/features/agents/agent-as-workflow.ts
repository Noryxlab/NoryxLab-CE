import type { Agent, AgentRun, Workflow, WorkflowRun } from "@/lib/api/types";

/**
 * Un agent est un workflow d'un seul pas.
 *
 * C'est ce que dit ADR-046, et c'est ce que la page doit montrer : le meme
 * canvas, le meme inspecteur, pour qu'un agent et un workflow se lisent avec
 * le meme vocabulaire. Rien n'est stocke sous cette forme - l'agent reste un
 * agent cote plateforme - c'est une lecture, derivee a l'affichage.
 */
export function agentAsWorkflow(agent: Agent): Workflow {
  return {
    id: agent.id,
    ownerUserId: agent.ownerUserId,
    projectId: agent.projectId ?? "",
    name: agent.name,
    schedule: agent.schedule,
    enabled: agent.enabled,
    createdAt: agent.createdAt,
    updatedAt: agent.updatedAt,
    lastRunAt: agent.lastRunAt,
    steps: [
      {
        index: 0,
        kind: "agent",
        name: agent.name,
        instruction: agent.mission,
        actions: agent.actions,
      },
    ],
  };
}

/**
 * Le passage d'un agent, lu comme un run. Une panne est un echec ; un passage
 * qui a rendu quelque chose - un rapport, ou le silence d'une heure calme -
 * est termine ; ce qui n'a encore rien rendu est en cours.
 */
export function agentRunAsWorkflowRun(run: AgentRun | undefined): WorkflowRun | undefined {
  if (!run) return undefined;
  const status = run.error
    ? "failed"
    : run.finishedAt || run.quiet || run.report
      ? "succeeded"
      : "running";
  return {
    id: run.id,
    workflowId: run.agentId,
    status,
    startedAt: run.startedAt,
    updatedAt: run.finishedAt ?? run.startedAt,
    finishedAt: run.finishedAt,
    error: run.error,
    steps: [
      {
        index: 0,
        status,
        attempts: 1,
        actions: run.actions,
        output: run.quiet ? undefined : run.report || undefined,
        error: run.error,
        startedAt: run.startedAt,
        finishedAt: run.finishedAt,
      },
    ],
  };
}
