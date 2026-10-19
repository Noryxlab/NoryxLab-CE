import { describe, expect, it } from "vitest";
import { agentAsWorkflow, agentRunAsWorkflowRun } from "./agent-as-workflow";
import type { Agent, AgentRun } from "@/lib/api/types";

const agent: Agent = {
  id: "a1",
  ownerUserId: "stef",
  projectId: "p1",
  role: "operator",
  name: "Charlotte",
  mission: "Surveille les workspaces.",
  schedule: "hourly",
  actions: ["restart_app"],
  enabled: true,
  createdAt: "2026-10-19T08:00:00Z",
  updatedAt: "2026-10-19T08:00:00Z",
  lastQuiet: false,
};

const run: AgentRun = {
  id: "r1",
  agentId: "a1",
  report: "Deux workspaces dorment.",
  quiet: false,
  actions: ["restart_app:demo"],
  startedAt: "2026-10-19T09:00:00Z",
  finishedAt: "2026-10-19T09:00:12Z",
};

describe("un agent lu comme un workflow", () => {
  it("fait un seul pas agent, avec la consigne et les droits de l'agent", () => {
    const workflow = agentAsWorkflow(agent);
    expect(workflow.steps).toHaveLength(1);
    expect(workflow.steps[0]).toMatchObject({
      index: 0,
      kind: "agent",
      instruction: "Surveille les workspaces.",
      actions: ["restart_app"],
    });
    expect(workflow.schedule).toBe("hourly");
  });

  it("lit un passage rendu comme un run termine, avec son rapport en sortie", () => {
    const mapped = agentRunAsWorkflowRun(run);
    expect(mapped?.status).toBe("succeeded");
    expect(mapped?.steps[0]?.output).toBe("Deux workspaces dorment.");
    expect(mapped?.steps[0]?.actions).toEqual(["restart_app:demo"]);
  });

  it("lit une panne comme un echec et une heure calme comme un run sans sortie", () => {
    expect(agentRunAsWorkflowRun({ ...run, error: "assistant injoignable" })?.status).toBe("failed");
    const quiet = agentRunAsWorkflowRun({ ...run, quiet: true, report: "" });
    expect(quiet?.status).toBe("succeeded");
    expect(quiet?.steps[0]?.output).toBeUndefined();
  });

  it("ne lit rien quand il n'y a pas de passage", () => {
    expect(agentRunAsWorkflowRun(undefined)).toBeUndefined();
  });
});
