import { useMemo } from "react";
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

/**
 * Visualizes a workflow DAG showing task dependencies.
 * Uses a simple CSS-based layout instead of a full graph library.
 */
export function WorkflowDag({ workflow, className }: WorkflowDagProps) {
  // Build the DAG structure with levels
  const { levels } = useMemo(() => {
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

  if (workflow.tasks.length === 0) {
    return (
      <div className={cn("text-center text-muted-foreground py-8", className)}>
        No tasks defined in this workflow
      </div>
    );
  }

  return (
    <div className={cn("overflow-auto", className)}>
      <div className="flex flex-col items-center gap-6 p-4 min-w-fit">
        {levels.map((levelNodes, levelIndex) => (
          <div key={levelIndex} className="flex flex-wrap gap-4 justify-center">
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
    <div className="relative group">
      {/* Connection lines to dependencies */}
      {hasDeps && (
        <div className="absolute left-1/2 -top-6 w-px h-6 bg-border" />
      )}

      <div className="bg-card border rounded-lg p-4 w-64 shadow-sm hover:shadow-md transition-shadow">
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

      {/* Arrow pointing down to dependents */}
      {task.dependents.length > 0 && (
        <div className="absolute left-1/2 -bottom-6 flex flex-col items-center">
          <div className="w-px h-4 bg-border" />
          <div className="w-0 h-0 border-l-4 border-r-4 border-t-4 border-l-transparent border-r-transparent border-t-border" />
        </div>
      )}
    </div>
  );
}
