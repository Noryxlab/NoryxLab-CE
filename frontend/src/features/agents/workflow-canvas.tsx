import * as React from "react";
import {
  Background,
  BackgroundVariant,
  BaseEdge,
  Controls,
  EdgeLabelRenderer,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  getSmoothStepPath,
  type Edge,
  type EdgeProps,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Bot, Clock, FileText, Plus, UserCheck } from "lucide-react";
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
 * noeuds ne se deplacent pas - la position est l'ordre, et l'ordre se change
 * dans l'inspecteur, pas en tirant une boite.
 *
 * En edition, le meme canvas montre le brouillon : chaque fleche porte un +
 * qui insere un pas a cet endroit, et un pas qui manque de consigne ou
 * d'approbateur se voit. Rien ne part tant qu'on n'a pas enregistre.
 */

/** Ce qui est ouvert dans l'inspecteur : un pas par son index, le
 *  declencheur, ou rien. */
export type CanvasSelection = number | "trigger" | null;

interface WorkflowCanvasProps {
  workflow: Workflow;
  latest?: WorkflowRun;
  selectedIndex: CanvasSelection;
  onSelectStep: (selection: CanvasSelection) => void;
  /** Le brouillon s'edite : les fleches portent un +, les pas incomplets se
   *  signalent, rien ne s'anime. */
  editing?: boolean;
  /** Inserer un pas a cette position (0 = avant le premier). */
  onInsertAt?: (at: number) => void;
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
  /** En edition : le pas manque de ce qu'il lui faut pour etre ecrit. */
  invalid?: boolean;
  editing?: boolean;
};

type InsertEdgeData = { at: number };

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
  selectedIndex: CanvasSelection,
  t: Translate,
  editing = false,
): { nodes: CanvasNode[]; edges: Edge[] } {
  const active = !editing && Boolean(latest && !isTerminal(latest.status));
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
        selected: selectedIndex === "trigger",
        attempts: 0,
        verbs: [],
        editing,
      },
    },
  ];
  workflow.steps.forEach((step, i) => {
    const stepRun = editing
      ? undefined
      : latest?.steps.find((item) => item.index === step.index);
    const isCurrent = current !== null && current === step.index;
    const invalid =
      editing &&
      (step.kind === "agent"
        ? !(step.instruction ?? "").trim()
        : !(step.approverUserId ?? "").trim());
    nodes.push({
      id: `s${step.index}`,
      type: "step",
      position: { x: GAP * (i + 1), y: 0 },
      draggable: false,
      data: {
        kind: step.kind,
        eyebrow: `${step.index + 1} · ${stepKindLabel(step, t)}`,
        title:
          step.name || (editing ? t("workflows.stepUntitled") : stepKindLabel(step, t)),
        tone: invalid ? "warning" : toneFor(stepRun?.status, isCurrent),
        current: isCurrent,
        selected: selectedIndex === step.index,
        attempts: stepRun?.attempts ?? 0,
        verbs: step.kind === "agent" ? (step.actions ?? []) : [],
        footer:
          step.kind === "approval"
            ? t("workflows.waitingFor", { name: step.approverUserId ?? "" })
            : undefined,
        status: stepRun?.status,
        invalid,
        editing,
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
      tone:
        !editing && finished
          ? "success"
          : !editing && latest?.status === "failed"
            ? "danger"
            : "muted",
      current: false,
      selected: false,
      attempts: 0,
      verbs: [],
      editing,
    },
  });

  // Une arete par voisinage. Elle s'anime vers le pas qui travaille, et
  // seulement vers lui : c'est ainsi qu'on voit ou en est la nuit. En
  // edition elle porte a la place l'endroit ou un + insere un pas : la
  // i-eme arete insere en position i.
  const edges = nodes.flatMap((from, i): Edge[] => {
    const to = nodes[i + 1];
    if (!to) return [];
    const live = active && (to.data.current || to.data.status === "running");
    const data: InsertEdgeData | undefined = editing ? { at: i } : undefined;
    return [
      {
        id: `${from.id}-${to.id}`,
        source: from.id,
        target: to.id,
        type: editing ? "insert" : "smoothstep",
        animated: live,
        data,
        markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
      },
    ];
  });
  return { nodes, edges };
}

/** Le + d'une arete appelle ce que le canvas a recu, sans le transporter
 *  dans la donnee de l'arete - la derivation reste une valeur pure. */
const InsertContext = React.createContext<((at: number) => void) | undefined>(undefined);

export function WorkflowCanvas({
  workflow,
  latest,
  selectedIndex,
  onSelectStep,
  editing = false,
  onInsertAt,
  className,
}: WorkflowCanvasProps) {
  const t = useT();
  const { nodes, edges } = React.useMemo(
    () => deriveCanvas(workflow, latest, selectedIndex, t, editing),
    [workflow, latest, selectedIndex, t, editing],
  );

  return (
    <div
      className={cn(
        "h-64 w-full overflow-hidden rounded-xl border bg-surface-muted/40",
        editing ? "border-dashed border-brand/60" : "border-border",
        className,
      )}
      aria-label={t("workflows.canvasTitle")}
    >
      <InsertContext.Provider value={onInsertAt}>
        <ReactFlow
          // Le nombre de pas change en edition : on recadre a chaque fois,
          // sinon le pas insere a droite sort du cadre.
          key={`${workflow.id}-${editing}-${workflow.steps.length}`}
          nodes={nodes}
          edges={edges}
          nodeTypes={NODE_TYPES}
          edgeTypes={EDGE_TYPES}
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
            if (node.id === "trigger") onSelectStep("trigger");
            else if (node.id.startsWith("s")) onSelectStep(Number(node.id.slice(1)));
            else onSelectStep(null);
          }}
          onPaneClick={() => onSelectStep(null)}
        >
          <Background variant={BackgroundVariant.Dots} gap={18} size={1} />
          <Controls showInteractive={false} position="bottom-right" />
        </ReactFlow>
      </InsertContext.Provider>
    </div>
  );
}

/**
 * Une arete qui porte un +. Le bouton est rendu hors du SVG, au milieu du
 * trace, et insere un pas la ou l'on clique - ce que fait tout outil du
 * genre, et ce que la main cherche d'elle-meme.
 */
function InsertEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  markerEnd,
  data,
}: EdgeProps<Edge<InsertEdgeData>>) {
  const t = useT();
  const insert = React.useContext(InsertContext);
  const [path, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  });
  return (
    <>
      <BaseEdge id={id} path={path} markerEnd={markerEnd} />
      <EdgeLabelRenderer>
        <button
          type="button"
          title={t("workflows.insertHere")}
          onClick={(event) => {
            event.stopPropagation();
            if (data && insert) insert(data.at);
          }}
          className={cn(
            "nodrag nopan pointer-events-auto absolute flex size-6 items-center justify-center rounded-full",
            "border border-brand bg-background text-brand shadow-sm transition hover:bg-brand hover:text-brand-foreground",
            "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--noryx-ring)]",
          )}
          style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}
        >
          <Plus className="size-3.5" aria-hidden />
          <span className="sr-only">{t("workflows.insertHere")}</span>
        </button>
      </EdgeLabelRenderer>
    </>
  );
}

const EDGE_TYPES = { insert: InsertEdge };

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
  const clickable =
    data.kind === "agent" || data.kind === "approval" || (data.kind === "trigger" && data.editing);
  return (
    <div
      className={cn(
        "flex w-48 flex-col gap-1 rounded-xl border px-3 py-2.5 text-left transition",
        data.kind === "approval" ? "border-dashed bg-background" : "bg-surface",
        borderFor(data.tone),
        data.current && "shadow-[0_0_0_2px_var(--noryx-brand)]",
        data.selected && "outline-2 outline-offset-2 outline-[var(--noryx-ring)]",
        data.invalid && "border-dashed",
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
