import { useMemo, useRef, useEffect, useState } from "react";
import type { Workflow, WorkflowTask, TaskStatus } from "~/lib/api";
import { cn } from "~/lib/utils";
import { Icons } from "~/components/icons";

export interface TaskStatusInfo {
  status: TaskStatus;
  duration_ms?: number;
  cache_hit?: boolean;
}

interface WorkflowDagProps {
  workflow: Workflow;
  taskStatuses?: Map<string, TaskStatusInfo>;
  className?: string;
}

interface TaskNode extends WorkflowTask {
  level: number;
  dependents: string[];
}

interface NodePosition {
  x: number;
  y: number;
  width: number;
  height: number;
}

// Subtle background colors for group containers
const GROUP_COLORS = [
  "bg-blue-500/5 border-blue-500/20",
  "bg-purple-500/5 border-purple-500/20",
  "bg-green-500/5 border-green-500/20",
  "bg-amber-500/5 border-amber-500/20",
  "bg-rose-500/5 border-rose-500/20",
  "bg-cyan-500/5 border-cyan-500/20",
];

/**
 * Visualizes a workflow DAG showing task dependencies.
 * Uses SVG for connector lines between nodes.
 */
export function WorkflowDag({
  workflow,
  taskStatuses,
  className,
}: WorkflowDagProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [nodePositions, setNodePositions] = useState<Map<string, NodePosition>>(
    new Map(),
  );
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });

  // Build the DAG structure with levels
  const { levels, nodeMap } = useMemo(() => {
    const nodeMap = new Map<string, TaskNode>();

    // Initialize nodes with dependents tracking
    workflow.tasks.forEach((task) => {
      nodeMap.set(task.name, { ...task, level: 0, dependents: [] });
    });

    // Build dependents list (reverse edges)
    workflow.tasks.forEach((task) => {
      task.needs?.forEach((dep) => {
        const depNode = nodeMap.get(dep);
        if (depNode) {
          depNode.dependents.push(task.name);
        }
      });
    });

    // Calculate levels using topological order
    const visited = new Set<string>();
    const calculateLevel = (name: string): number => {
      const node = nodeMap.get(name);
      if (!node) return 0;
      if (visited.has(name)) return node.level;

      visited.add(name);

      if (!node.needs || node.needs.length === 0) {
        node.level = 0;
        return 0;
      }

      const maxDepLevel = Math.max(
        ...node.needs.map((dep) => calculateLevel(dep)),
      );
      node.level = maxDepLevel + 1;
      return node.level;
    };

    workflow.tasks.forEach((task) => {
      calculateLevel(task.name);
    });

    // Group by levels
    const levels: TaskNode[][] = [];
    nodeMap.forEach((node) => {
      if (!levels[node.level]) {
        levels[node.level] = [];
      }
      levels[node.level].push(node);
    });

    return { levels, nodeMap };
  }, [workflow.tasks]);

  // Build group color map
  const groupColorMap = useMemo(() => {
    const groups = new Set<string>();
    workflow.tasks.forEach((t) => {
      if (t.group) groups.add(t.group);
    });
    const map = new Map<string, string>();
    let idx = 0;
    groups.forEach((g) => {
      map.set(g, GROUP_COLORS[idx % GROUP_COLORS.length]);
      idx++;
    });
    return map;
  }, [workflow.tasks]);

  // Measure node positions after render
  useEffect(() => {
    if (!containerRef.current) return;

    const updatePositions = () => {
      const container = containerRef.current;
      if (!container) return;

      const positions = new Map<string, NodePosition>();
      const nodeElements = container.querySelectorAll("[data-node-id]");

      nodeElements.forEach((el) => {
        const nodeId = el.getAttribute("data-node-id");
        if (nodeId) {
          const rect = el.getBoundingClientRect();
          const containerRect = container.getBoundingClientRect();
          positions.set(nodeId, {
            x: rect.left - containerRect.left + rect.width / 2,
            y: rect.top - containerRect.top,
            width: rect.width,
            height: rect.height,
          });
        }
      });

      setNodePositions(positions);
      setContainerSize({
        width: container.scrollWidth,
        height: container.scrollHeight,
      });
    };

    // Update positions after a short delay to ensure layout is complete
    const timer = setTimeout(updatePositions, 100);

    // Also update on resize
    const resizeObserver = new ResizeObserver(updatePositions);
    resizeObserver.observe(containerRef.current);

    return () => {
      clearTimeout(timer);
      resizeObserver.disconnect();
    };
  }, [levels]);

  // Derive edge status from source/target task statuses
  type EdgeStatus = "idle" | "active" | "success" | "failed" | "cached";

  const getEdgeStatus = (from: string, to: string): EdgeStatus => {
    if (!taskStatuses) return "idle";
    const toStatus = taskStatuses.get(to)?.status;
    const fromStatus = taskStatuses.get(from)?.status;

    // Target is actively running — the edge is "flowing"
    if (toStatus === "running") return "active";
    // Target completed successfully (or was cached)
    if (toStatus === "success") return "success";
    if (toStatus === "cached") return "cached";
    // Target failed
    if (toStatus === "failed") return "failed";
    // Source completed but target hasn't started — edge is ready/success
    if (fromStatus === "success" || fromStatus === "cached") return "success";

    return "idle";
  };

  // Generate connector paths
  const connectors = useMemo(() => {
    const paths: { from: string; to: string; path: string }[] = [];

    nodeMap.forEach((node) => {
      node.needs?.forEach((depName) => {
        const fromPos = nodePositions.get(depName);
        const toPos = nodePositions.get(node.name);

        if (fromPos && toPos) {
          // Calculate path from bottom of parent to top of child
          const startX = fromPos.x;
          const startY = fromPos.y + fromPos.height + 2; // Small offset from bottom
          const endX = toPos.x;
          const endY = toPos.y - 8; // End before the node to make room for arrowhead

          // Use a bezier curve for smooth connections
          const controlOffset = Math.min(Math.abs(endY - startY) * 0.5, 40);

          const path = `M ${startX} ${startY} C ${startX} ${startY + controlOffset}, ${endX} ${endY - controlOffset}, ${endX} ${endY}`;

          paths.push({ from: depName, to: node.name, path });
        }
      });
    });

    return paths;
  }, [nodeMap, nodePositions]);

  if (workflow.tasks.length === 0) {
    return (
      <div className={cn("text-center text-muted-foreground py-8", className)}>
        No tasks defined in this workflow
      </div>
    );
  }

  return (
    <div className={cn("overflow-auto relative", className)} ref={containerRef}>
      {/* Trigger & config info */}
      {workflow.trigger && <TriggerInfo trigger={workflow.trigger} />}
      <ConfigInfo workflow={workflow} />

      {/* SVG layer for connectors */}
      <svg
        className="absolute inset-0 pointer-events-none"
        width={containerSize.width || "100%"}
        height={containerSize.height || "100%"}
        style={{ minWidth: "100%", minHeight: "100%" }}
      >
        <defs>
          <marker
            id="arrowhead"
            markerWidth="8"
            markerHeight="8"
            refX="8"
            refY="4"
            orient="auto"
            markerUnits="userSpaceOnUse"
          >
            <path d="M 0 0 L 8 4 L 0 8 Z" className="fill-muted-foreground/60" />
          </marker>
          <marker
            id="arrowhead-active"
            markerWidth="8"
            markerHeight="8"
            refX="8"
            refY="4"
            orient="auto"
            markerUnits="userSpaceOnUse"
          >
            <path d="M 0 0 L 8 4 L 0 8 Z" fill="#3b82f6" />
          </marker>
          <marker
            id="arrowhead-success"
            markerWidth="8"
            markerHeight="8"
            refX="8"
            refY="4"
            orient="auto"
            markerUnits="userSpaceOnUse"
          >
            <path d="M 0 0 L 8 4 L 0 8 Z" fill="#22c55e" />
          </marker>
          <marker
            id="arrowhead-failed"
            markerWidth="8"
            markerHeight="8"
            refX="8"
            refY="4"
            orient="auto"
            markerUnits="userSpaceOnUse"
          >
            <path d="M 0 0 L 8 4 L 0 8 Z" fill="#ef4444" />
          </marker>
          <marker
            id="arrowhead-cached"
            markerWidth="8"
            markerHeight="8"
            refX="8"
            refY="4"
            orient="auto"
            markerUnits="userSpaceOnUse"
          >
            <path d="M 0 0 L 8 4 L 0 8 Z" fill="#a855f7" />
          </marker>
        </defs>
        {connectors.map(({ from, to, path }) => {
          const status = getEdgeStatus(from, to);
          const isActive = status === "active";

          const strokeColor =
            status === "active"
              ? "#3b82f6"
              : status === "success"
                ? "#22c55e"
                : status === "failed"
                  ? "#ef4444"
                  : status === "cached"
                    ? "#a855f7"
                    : undefined;

          const markerId =
            status === "idle" ? "arrowhead" : `arrowhead-${status}`;

          return isActive ? (
            <g key={`${from}-${to}`}>
              {/* Faint static background stroke */}
              <path
                d={path}
                fill="none"
                stroke={strokeColor}
                strokeOpacity={0.25}
                strokeWidth="1.5"
                strokeLinecap="round"
              />
              {/* Animated dashed foreground stroke */}
              <path
                d={path}
                fill="none"
                stroke={strokeColor}
                strokeWidth="2"
                strokeLinecap="round"
                strokeDasharray="6 4"
                markerEnd={`url(#${markerId})`}
              >
                <animate
                  attributeName="stroke-dashoffset"
                  from="20"
                  to="0"
                  dur="0.6s"
                  repeatCount="indefinite"
                />
              </path>
            </g>
          ) : (
            <path
              key={`${from}-${to}`}
              d={path}
              fill="none"
              stroke={strokeColor}
              className={strokeColor ? undefined : "stroke-muted-foreground/60"}
              strokeWidth="1.5"
              strokeLinecap="round"
              markerEnd={`url(#${markerId})`}
            />
          );
        })}
      </svg>

      {/* Node layout */}
      <div className="flex flex-col items-center gap-12 p-6 min-w-fit relative z-10">
        {levels.map((levelNodes, levelIndex) => {
          // Group nodes within this level by their group
          const grouped = new Map<string, TaskNode[]>();
          const ungrouped: TaskNode[] = [];
          levelNodes.forEach((node) => {
            if (node.group) {
              const list = grouped.get(node.group) || [];
              list.push(node);
              grouped.set(node.group, list);
            } else {
              ungrouped.push(node);
            }
          });

          return (
            <div key={levelIndex} className="flex flex-wrap gap-6 justify-center items-stretch">
              {/* Render grouped tasks in containers */}
              {Array.from(grouped.entries()).map(([groupName, nodes]) => (
                <div
                  key={groupName}
                  className={cn(
                    "flex flex-wrap gap-4 p-3 rounded-none border items-stretch",
                    groupColorMap.get(groupName),
                  )}
                >
                  <div className="w-full text-xs font-medium text-muted-foreground mb-1">
                    {groupName}
                  </div>
                  {nodes.map((node) => (
                    <TaskCard
                      key={node.name}
                      task={node}
                      statusInfo={taskStatuses?.get(node.name)}
                    />
                  ))}
                </div>
              ))}
              {/* Render ungrouped tasks */}
              {ungrouped.map((node) => (
                <TaskCard
                  key={node.name}
                  task={node}
                  statusInfo={taskStatuses?.get(node.name)}
                />
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function TriggerInfo({
  trigger,
}: {
  trigger: NonNullable<Workflow["trigger"]>;
}) {
  const parts: string[] = [];
  if (trigger.push?.branches?.length) {
    parts.push(`Push: ${trigger.push.branches.join(", ")}`);
  }
  if (trigger.pull_request) {
    const pr = trigger.pull_request;
    let text = "PR";
    if (pr.branches?.length) text += `: ${pr.branches.join(", ")}`;
    if (pr.types?.length) text += ` (${pr.types.join(", ")})`;
    parts.push(text);
  }

  if (parts.length === 0) return null;

  return (
    <div className="flex items-center gap-2 px-6 pt-4 pb-0 text-xs text-muted-foreground">
      <span className="font-medium">Triggers:</span>
      {parts.map((part, i) => (
        <span key={i} className="bg-muted px-2 py-0.5 rounded-none">
          {part}
        </span>
      ))}
    </div>
  );
}

function ConfigInfo({ workflow }: { workflow: Workflow }) {
  const badges: string[] = [];

  if (workflow.cache) {
    if (workflow.cache.enabled) badges.push("Cache: Local");
    if (workflow.cache.remote_cloud) badges.push("Cache: Cloud");
    else if (workflow.cache.remote_enabled) badges.push("Cache: Remote");
  }

  if (workflow.ai?.enabled) {
    const parts = [workflow.ai.mode, workflow.ai.provider, workflow.ai.model]
      .filter(Boolean)
      .join(" / ");
    badges.push(parts ? `AI: ${parts}` : "AI");
  }

  if (workflow.container?.enabled) {
    const parts = [
      workflow.container.image,
      workflow.container.memory_limit ? `mem:${workflow.container.memory_limit}` : "",
      workflow.container.cpu_limit ? `cpu:${workflow.container.cpu_limit}` : "",
    ]
      .filter(Boolean)
      .join(" / ");
    badges.push(parts ? `Container: ${parts}` : "Container");
  }

  if (badges.length === 0) return null;

  return (
    <div className="flex items-center gap-2 px-6 pt-2 pb-0 text-xs text-muted-foreground flex-wrap">
      <span className="font-medium">Config:</span>
      {badges.map((badge, i) => (
        <span key={i} className="bg-muted px-2 py-0.5 rounded-none">
          {badge}
        </span>
      ))}
    </div>
  );
}

interface TaskCardProps {
  task: TaskNode;
  statusInfo?: TaskStatusInfo;
}

const STATUS_BORDER_COLORS: Record<string, string> = {
  success: "border-l-green-500",
  running: "border-l-blue-500",
  failed: "border-l-red-500",
  cached: "border-l-purple-500",
  pending: "border-l-yellow-500",
  skipped: "border-l-gray-400",
  cancelled: "border-l-gray-400",
};

function DagTaskStatusIcon({ status }: { status: TaskStatus }) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className="h-4 w-4 text-green-500" />;
    case "failed":
      return <Icons.XCircle className="h-4 w-4 text-red-500" />;
    case "running":
      return <Icons.Loader className="h-4 w-4 text-blue-500 animate-spin" />;
    case "cached":
      return <Icons.Database className="h-4 w-4 text-purple-500" />;
    case "pending":
      return <Icons.Circle className="h-4 w-4 text-yellow-500" />;
    case "skipped":
      return <Icons.Minus className="h-4 w-4 text-gray-400" />;
    case "cancelled":
      return <Icons.XCircle className="h-4 w-4 text-gray-400" />;
    default:
      return null;
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

function TaskCard({ task, statusInfo }: TaskCardProps) {
  const hasDeps = task.needs && task.needs.length > 0;
  const hasPlugins = task.plugins && task.plugins.length > 0;
  const hasCondition = !!task.condition;
  const status = statusInfo?.status;
  const isPending = !statusInfo || status === "pending";

  return (
    <div className="relative" data-node-id={task.name}>
      <div
        className={cn(
          "bg-card border border-border rounded-none p-4 w-64 h-full shadow-sm hover:shadow-md transition-shadow hover:border-primary/50",
          statusInfo && "border-l-4",
          statusInfo && STATUS_BORDER_COLORS[status || ""],
          status === "running" && "animate-pulse",
          statusInfo && isPending && "opacity-50",
        )}
      >
        <div className="flex items-center justify-between mb-2">
          <h4 className="font-semibold text-sm truncate flex-1">{task.name}</h4>
          <div className="flex items-center gap-1.5 shrink-0">
            {hasPlugins && (
              <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded-none">
                {task.plugins?.length} plugin
                {task.plugins && task.plugins.length > 1 ? "s" : ""}
              </span>
            )}
            {statusInfo && status && <DagTaskStatusIcon status={status} />}
          </div>
        </div>

        <code className="block text-xs text-muted-foreground bg-muted p-2 rounded-none truncate">
          {task.command}
        </code>

        {statusInfo?.duration_ms != null && (
          <div className="mt-2 flex items-center gap-1 text-xs text-muted-foreground">
            <Icons.Clock className="h-3 w-3" />
            {formatDuration(statusInfo.duration_ms)}
            {statusInfo.cache_hit && (
              <span className="ml-1.5 text-purple-500 flex items-center gap-0.5">
                <Icons.Database className="h-3 w-3" />
                cached
              </span>
            )}
          </div>
        )}

        {hasCondition && (
          <div className="mt-2 flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
            <svg
              className="w-3.5 h-3.5 shrink-0"
              viewBox="0 0 16 16"
              fill="currentColor"
            >
              <path d="M8 1a.75.75 0 01.75.75v6.5a.75.75 0 01-1.5 0v-6.5A.75.75 0 018 1zM8 11a1 1 0 100 2 1 1 0 000-2z" />
              <path
                fillRule="evenodd"
                d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"
              />
            </svg>
            <span className="truncate" title={task.condition}>
              if: {task.condition}
            </span>
          </div>
        )}

        {hasDeps && (
          <div className="mt-2 text-xs text-muted-foreground">
            <span className="font-medium">Depends on:</span>{" "}
            {task.needs?.join(", ")}
          </div>
        )}

        {task.inputs && task.inputs.length > 0 && (
          <div className="mt-1 text-xs text-muted-foreground truncate">
            <span className="font-medium">Inputs:</span>{" "}
            {task.inputs.slice(0, 2).join(", ")}
            {task.inputs.length > 2 && ` +${task.inputs.length - 2} more`}
          </div>
        )}
      </div>
    </div>
  );
}
