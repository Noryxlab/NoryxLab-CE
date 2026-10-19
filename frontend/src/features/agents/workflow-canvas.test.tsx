import { describe, expect, it } from "vitest";
import { deriveCanvas } from "./workflow-canvas";
import type { Workflow, WorkflowRun } from "@/lib/api/types";

/**
 * Le canvas est derive, jamais dessine : les noeuds sont les pas, et l'arete
 * qui bouge est celle qui mene au pas qui travaille. Ce test fixe la
 * derivation et non le rendu - React Flow mesure ses noeuds dans un vrai
 * navigateur, et les tests de ce depot tournent sans.
 */

const t = ((key: string, values?: Record<string, string | number>) =>
  values ? `${key}:${Object.values(values).join(",")}` : key) as never;

const workflow: Workflow = {
  id: "w1",
  ownerUserId: "stef",
  projectId: "p1",
  name: "Veille puis analyse",
  schedule: "daily",
  enabled: true,
  createdAt: "2026-10-19T08:00:00Z",
  updatedAt: "2026-10-19T08:00:00Z",
  steps: [
    { index: 0, kind: "agent", name: "Chercher", instruction: "Cherche.", actions: [] },
    { index: 1, kind: "approval", name: "Relire", approverUserId: "cyril" },
    { index: 2, kind: "agent", name: "Analyser", instruction: "Analyse.", actions: ["restart_app"] },
  ],
};

const waiting: WorkflowRun = {
  id: "r1",
  workflowId: "w1",
  status: "waiting_approval",
  startedAt: "2026-10-19T09:00:00Z",
  updatedAt: "2026-10-19T09:01:00Z",
  steps: [
    { index: 0, status: "succeeded", attempts: 1, actions: [], output: "Trouve 3 choses." },
    { index: 1, status: "waiting_approval", attempts: 2, actions: [] },
    { index: 2, status: "pending", attempts: 0, actions: [] },
  ],
};

describe("deriveCanvas", () => {
  it("fait un noeud par pas, entre un declencheur et un rapport, relies dans l'ordre", () => {
    const { nodes, edges } = deriveCanvas(workflow, undefined, null, t);
    expect(nodes.map((n) => n.id)).toEqual(["trigger", "s0", "s1", "s2", "report"]);
    expect(edges.map((e) => `${e.source}>${e.target}`)).toEqual([
      "trigger>s0",
      "s0>s1",
      "s1>s2",
      "s2>report",
    ]);
    // Les positions avancent de gauche a droite : une ligne, pas un graphe.
    const xs = nodes.map((n) => n.position.x);
    expect([...xs].sort((a, b) => a - b)).toEqual(xs);
  });

  it("marque le pas courant et n'anime que l'arete qui y mene", () => {
    const { nodes, edges } = deriveCanvas(workflow, waiting, null, t);
    expect(nodes.find((n) => n.id === "s1")?.data.current).toBe(true);
    expect(nodes.find((n) => n.id === "s1")?.data.tone).toBe("warning");
    expect(nodes.find((n) => n.id === "s0")?.data.tone).toBe("success");
    expect(edges.filter((e) => e.animated).map((e) => e.id)).toEqual(["s0-s1"]);
  });

  it("porte le nombre de tentatives quand il y en a eu plus d'une", () => {
    const { nodes } = deriveCanvas(workflow, waiting, null, t);
    expect(nodes.find((n) => n.id === "s1")?.data.attempts).toBe(2);
    expect(nodes.find((n) => n.id === "s0")?.data.attempts).toBe(1);
  });

  it("n'anime rien et clot le rapport quand le run est termine", () => {
    const done: WorkflowRun = {
      ...waiting,
      status: "succeeded",
      steps: waiting.steps.map((s) => ({ ...s, status: "succeeded" as const })),
    };
    const { nodes, edges } = deriveCanvas(workflow, done, null, t);
    expect(edges.some((e) => e.animated)).toBe(false);
    expect(nodes.find((n) => n.id === "report")?.data.tone).toBe("success");
    expect(nodes.some((n) => n.data.current)).toBe(false);
  });

  it("reflete la selection sur le seul noeud choisi", () => {
    const { nodes } = deriveCanvas(workflow, waiting, 2, t);
    expect(nodes.filter((n) => n.data.selected).map((n) => n.id)).toEqual(["s2"]);
  });
});
