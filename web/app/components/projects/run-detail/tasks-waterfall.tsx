import { useMemo } from "react";
import type { TaskResult } from "~/lib/api";
import { cn } from "~/lib/utils";
import { TaskStatusIcon, StatusBadge, formatDuration } from "./status-ui";

interface TasksWaterfallProps {
  tasks: TaskResult[];
}

function barColor(status: string): string {
  switch (status) {
    case "success":
      return "bg-green-500";
    case "failed":
      return "bg-red-500";
    case "running":
      return "bg-blue-500 animate-pulse";
    case "cached":
      return "bg-purple-500";
    case "skipped":
    case "cancelled":
      return "bg-gray-400";
    default:
      return "";
  }
}

export function TasksWaterfall({ tasks }: TasksWaterfallProps) {
  const timeline = useMemo(() => {
    const tasksWithTiming = tasks.filter((t) => t.started_at);
    if (tasksWithTiming.length === 0) return null;

    const now = Date.now();
    let minStart = Infinity;
    let maxEnd = -Infinity;

    for (const t of tasksWithTiming) {
      const start = new Date(t.started_at!).getTime();
      const end = t.finished_at ? new Date(t.finished_at).getTime() : now;
      if (start < minStart) minStart = start;
      if (end > maxEnd) maxEnd = end;
    }

    const span = maxEnd - minStart || 1;

    const bars = tasks.map((t) => {
      if (!t.started_at) {
        return { task: t, left: 0, width: 0, hasTiming: false };
      }
      const start = new Date(t.started_at).getTime();
      const end = t.finished_at ? new Date(t.finished_at).getTime() : now;
      const left = ((start - minStart) / span) * 100;
      const width = Math.max(((end - start) / span) * 100, 0.5);
      return { task: t, left, width, hasTiming: true };
    });

    return { bars, totalMs: maxEnd - minStart };
  }, [tasks]);

  if (!timeline) {
    return (
      <div className="flex items-center justify-center h-32 text-muted-foreground">
        No timing data available
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {/* Header */}
      <div className="grid grid-cols-[1fr_minmax(0,2fr)] md:grid-cols-[180px_70px_1fr_70px] gap-2 px-2 py-1 text-xs text-muted-foreground font-medium border-b">
        <span>Task</span>
        <span className="hidden md:inline">Status</span>
        <span>Timeline</span>
        <span className="hidden md:inline text-right">Duration</span>
      </div>

      {/* Rows */}
      {timeline.bars.map(({ task, left, width, hasTiming }) => (
        <div
          key={task.id}
          className="grid grid-cols-[1fr_minmax(0,2fr)] md:grid-cols-[180px_70px_1fr_70px] gap-2 items-center px-2 py-1.5 rounded-none hover:bg-muted/50"
        >
          {/* Task name */}
          <div className="flex items-center gap-2 min-w-0">
            <TaskStatusIcon status={task.status} />
            <span className="truncate text-sm font-medium">
              {task.task_name}
            </span>
          </div>

          {/* Status badge */}
          <div className="hidden md:block">
            <StatusBadge status={task.status} />
          </div>

          {/* Timeline bar */}
          <div className="relative h-6 bg-muted/30 rounded-none overflow-hidden">
            {hasTiming ? (
              <div
                className={cn(
                  "absolute top-1 bottom-1 rounded-none min-w-0.75",
                  barColor(task.status),
                )}
                style={{ left: `${left}%`, width: `${width}%` }}
              />
            ) : (
              <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
                --
              </div>
            )}
          </div>

          {/* Duration */}
          <span className="hidden md:inline text-xs text-muted-foreground text-right tabular-nums">
            {task.duration_ms != null ? formatDuration(task.duration_ms) : "--"}
          </span>
        </div>
      ))}

      {/* Total duration footer */}
      <div className="flex justify-end px-2 pt-2 border-t text-xs text-muted-foreground">
        <span>Total span: {formatDuration(timeline.totalMs)}</span>
      </div>
    </div>
  );
}
