import * as React from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Bot, Clock, FileText, UserCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import { scheduleKey } from "./agent-roster";
import { currentStep, isTerminal, stepKindLabel } from "./workflow-pipeline";
import type {
  Workflow,
  WorkflowRun,
  WorkflowRunStatus,
} from "@/lib/api/types";

/**
 * La sequence, sur un canvas.
 *
 * Meme matiere que la pipeline a cote : un declencheur, des pas, un rapport,
 * derives et jamais dessines a la main. Ce qui change est la surface. Un
 * canvas qui se deplace et se zoome, des aretes qui s'animent vers le pas en
 * cours, un noeud qui se clique pour s'ouvrir - c'est le vocabulaire que
 * toute personne ayant vu un outil d'automatisation a en tete, et c'est a ce
 * vocabulaire qu'une direction se projette.
 *
 * La disposition est une ligne de gauche a droite, pas un graphe libre : la
 * sequence est ce que le dossier recommande la ou le controle compte
 * (ADR-046), et les aretes arriveront avec les mandats qui les porteront. Les
 * noeuds ne se deplacent pas - rien ici n'est un reglage a sauvegarder, tout
 * est l'etat que la plateforme a enregistre.
 */

interface WorkflowCanvasProps {
  workflow: Workflow;
  latest?: WorkflowRun;
  selectedIndex: number | null;
  onSelectStep: (index: number | null) => void;
  className?: string;
}

type Tone = "brand" | "success" | "warning" | "danger" | "muted";

type CanvasData = {
  kind: "trigger" | "agent" | "approval" | "report";
  eyebrow: string;
  title: string;
  tone: Tone;
  current: boolean;
  selected: boolean;
  attempts: number;
  verbs: string[];
  footer?: string;
  status?: WorkflowRunStatus;
};

type CanvasNode = Node<CanvasData, "step">;

const GAP = 232;

type Translate = ReturnType<typeof useT>;

/**
 * Les noeuds et les aretes, derives du workflow et de son dernier run.
 *
 * Une fonction pure et non un hook : c'est la partie qui peut se tromper -
 * quel pas est courant, quelle arete bouge, ce que porte chaque noeud - et
 * c'est donc celle qui se teste, sans navigateur. Le composant n'en fait que
 * le rendu.
 */
export function deriveCanvas(
  workflow: Workflow,
  latest: WorkflowRun | undefined,
  selectedIndex: number | null,
  t: Translate,
): { nodes: CanvasNode[]; edges: Edge[] } {
  const active = Boolean(latest && !isTerminal(latest.status));
  const current = active && latest ? currentStep(latest) : null;
  const finished = latest?.status === "succeeded";

  const nodes: CanvasNode[] = [
    {
      id: "trigger",
      type: "step",
      position: { x: 0, y: 0 },
      draggable: false,
      data: {
        kind: "trigger",
        eyebrow: t("workflows.triggerNode"),
        title: t(scheduleKey(workflow.schedule)),
        tone: workflow.enabled ? "brand" : "muted",
        current: false,
        selected: false,
        attempts: 0,
        verbs: [],
      },
    },
  ];
  workflow.steps.forEach((step, i) => {
    const stepRun = latest?.steps.find((item) => item.index === step.index);
    const isCurrent = current !== null && current === step.index;
    nodes.push({
      id: `s${step.index}`,
      type: "step",
      position: { x: GAP * (i + 1), y: 0 },
      draggable: false,
      data: {
        kind: step.kind,
        eyebrow: `${step.index + 1} · ${stepKindLabel(step, t)}`,
        title: step.name || stepKindLabel(step, t),
        tone: toneFor(stepRun?.status, isCurrent),
        current: isCurrent,
        selected: selectedIndex === step.index,
        attempts: stepRun?.attempts ?? 0,
        verbs: step.kind === "agent" ? (step.actions ?? []) : [],
        footer:
          step.kind === "approval"
            ? t("workflows.waitingFor", { name: step.approverUserId ?? "" })
            : undefined,
        status: stepRun?.status,
      },
    });
  });
  nodes.push({
    id: "report",
    type: "step",
    position: { x: GAP * (workflow.steps.length + 1), y: 0 },
    draggable: false,
    data: {
      kind: "report",
      eyebrow: t("workflows.reportNode"),
      title: finished
        ? t("workflows.statusSucceeded")
        : latest?.status === "failed"
          ? t("workflows.statusFailed")
          : "…",
      tone: finished ? "success" : latest?.status === "failed" ? "danger" : "muted",
      current: false,
      selected: false,
      attempts: 0,
      verbs: [],
    },
  });

  // Une arete par voisinage. Elle s'anime vers le pas qui travaille, et
  // seulement vers lui : c'est ainsi qu'on voit ou en est la nuit.
  const edges = nodes.flatMap((from, i): Edge[] => {
    const to = nodes[i + 1];
    if (!to) return [];
    const live = active && (to.data.current || to.data.status === "running");
    return [
      {
        id: `${from.id}-${to.id}`,
        source: from.id,
        target: to.id,
        type: "smoothstep",
        animated: live,
        markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
      },
    ];
  });
  return { nodes, edges };
}

export function WorkflowCanvas({
  workflow,
  latest,
  selectedIndex,
  onSelectStep,
  className,
}: WorkflowCanvasProps) {
  const t = useT();
  const { nodes, edges } = React.useMemo(
    () => deriveCanvas(workflow, latest, selectedIndex, t),
    [workflow, latest, selectedIndex, t],
  );

  return (
    <div
      className={cn(
        "h-64 w-full overflow-hidden rounded-xl border border-border bg-surface-muted/40",
        className,
      )}
      aria-label={t("workflows.canvasTitle")}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        fitView
        fitViewOptions={{ padding: 0.25, maxZoom: 1 }}
        minZoom={0.4}
        maxZoom={1.4}
        nodesDraggable={false}
        nodesConnectable={false}
        panOnScroll
        zoomOnDoubleClick={false}
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_, node) => {
          if (node.id.startsWith("s")) onSelectStep(Number(node.id.slice(1)));
        }}
        onPaneClick={() => onSelectStep(null)}
      >
        <Background variant={BackgroundVariant.Dots} gap={18} size={1} />
        <Controls showInteractive={false} position="bottom-right" />
      </ReactFlow>
    </div>
  );
}

function StepNodeView({ data }: NodeProps<CanvasNode>) {
  const t = useT();
  const Icon =
    data.kind === "trigger"
      ? Clock
      : data.kind === "report"
        ? FileText
        : data.kind === "approval"
          ? UserCheck
          : Bot;
  const clickable = data.kind === "agent" || data.kind === "approval";
  return (
    <div
      className={cn(
        "flex w-48 flex-col gap-1 rounded-xl border px-3 py-2.5 text-left transition",
        data.kind === "approval" ? "border-dashed bg-background" : "bg-surface",
        borderFor(data.tone),
        data.current && "shadow-[0_0_0_2px_var(--noryx-brand)]",
        data.selected && "outline-2 outline-offset-2 outline-[var(--noryx-ring)]",
        clickable ? "cursor-pointer hover:border-border-strong" : "cursor-default",
      )}
    >
      {data.kind !== "trigger" ? (
        <Handle type="target" position={Position.Left} className="!size-2 !border-0" />
      ) : null}
      <span className="flex items-center gap-1.5 text-[11px] uppercase tracking-wide text-placeholder">
        <span className={cn("size-1.5 rounded-full", dotFor(data.tone))} aria-hidden />
        <Icon className="size-3.5" aria-hidden />
        <span className="truncate">{data.eyebrow}</span>
        {data.attempts > 1 ? (
          <Badge tone="warning" className="ml-auto text-[10px]">
            ×{data.attempts}
          </Badge>
        ) : null}
      </span>
      <span className="truncate text-sm font-medium">{data.title}</span>
      {data.kind === "agent" ? (
        data.verbs.length > 0 ? (
          <span className="flex flex-wrap gap-1">
            {data.verbs.map((verb) => (
              <Badge key={verb} tone="warning" className="text-[10px]">
                {verb === "call_api"
                  ? "API"
                  : verb === "restart_app"
                    ? t("agents.actionRestartApp")
                    : verb}
              </Badge>
            ))}
          </span>
        ) : (
          <span className="truncate text-[11px] text-placeholder">
            {t("workflows.stepReadsOnly")}
          </span>
        )
      ) : data.footer ? (
        <span className="truncate text-[11px] text-muted-foreground">{data.footer}</span>
      ) : null}
      {data.kind !== "report" ? (
        <Handle type="source" position={Position.Right} className="!size-2 !border-0" />
      ) : null}
    </div>
  );
}

const NODE_TYPES = { step: StepNodeView };

function toneFor(status: WorkflowRunStatus | undefined, current: boolean): Tone {
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

function borderFor(tone: Tone): string {
  switch (tone) {
    case "success":
      return "border-[var(--noryx-success)]";
    case "warning":
      return "border-[var(--noryx-warning)]";
    case "danger":
      return "border-[var(--noryx-danger)]";
    case "brand":
      return "border-brand";
    default:
      return "border-border";
  }
}

function dotFor(tone: Tone): string {
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
