import { useMemo, useRef, useEffect, useState } from "react";
import type { Workflow, WorkflowTask } from "~/lib/api";
import { cn } from "~/lib/utils";

interface WorkflowDagProps {
  workflow: Workflow;
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

/**
 * Visualizes a workflow DAG showing task dependencies.
 * Uses SVG for connector lines between nodes.
 */
export function WorkflowDag({ workflow, className }: WorkflowDagProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [nodePositions, setNodePositions] = useState<Map<string, NodePosition>>(
    new Map()
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
        ...node.needs.map((dep) => calculateLevel(dep))
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
            markerWidth="10"
            markerHeight="10"
            refX="5"
            refY="5"
            orient="auto-start-reverse"
            markerUnits="strokeWidth"
          >
            <path
              d="M 0 0 L 10 5 L 0 10 z"
              className="fill-muted-foreground/60"
            />
          </marker>
        </defs>
        {connectors.map(({ from, to, path }) => (
          <path
            key={`${from}-${to}`}
            d={path}
            fill="none"
            className="stroke-muted-foreground/60"
            strokeWidth="1.5"
            strokeLinecap="round"
            markerEnd="url(#arrowhead)"
          />
        ))}
      </svg>

      {/* Node layout */}
      <div className="flex flex-col items-center gap-12 p-6 min-w-fit relative z-10">
        {levels.map((levelNodes, levelIndex) => (
          <div key={levelIndex} className="flex flex-wrap gap-6 justify-center">
            {levelNodes.map((node) => (
              <TaskCard key={node.name} task={node} />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

interface TaskCardProps {
  task: TaskNode;
}

function TaskCard({ task }: TaskCardProps) {
  const hasDeps = task.needs && task.needs.length > 0;
  const hasPlugins = task.plugins && task.plugins.length > 0;

  return (
    <div className="relative" data-node-id={task.name}>
      <div className="bg-card border border-border rounded-lg p-4 w-64 shadow-sm hover:shadow-md transition-shadow hover:border-primary/50">
        <div className="flex items-center justify-between mb-2">
          <h4 className="font-semibold text-sm truncate">{task.name}</h4>
          {hasPlugins && (
            <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded-full">
              {task.plugins?.length} plugin
              {task.plugins && task.plugins.length > 1 ? "s" : ""}
            </span>
          )}
        </div>

        <code className="block text-xs text-muted-foreground bg-muted p-2 rounded truncate">
          {task.command}
        </code>

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
