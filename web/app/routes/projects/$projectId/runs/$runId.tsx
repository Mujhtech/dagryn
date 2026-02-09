import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState, useRef, useCallback } from "react";
import { useAuth } from "~/lib/auth";
import { useRunDetail, useRunLogs } from "~/hooks/queries";
import { useCancelRun } from "~/hooks/mutations";
import type { TaskResult, LogEntry } from "~/lib/api";
import {
  RunStreamClient,
  type LogEventData,
  type TaskEventData,
} from "~/lib/sse";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Progress } from "~/components/ui/progress";
import { ScrollArea } from "~/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Input } from "~/components/ui/input";
import { cn } from "~/lib/utils";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/projects/$projectId/runs/$runId")({
  component: RunDetailPage,
});

interface LogLine {
  id?: number;
  task_name?: string;
  stream: "stdout" | "stderr";
  line: string;
  line_num: number;
}

function RunDetailPage() {
  const { projectId, runId } = Route.useParams();
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  // Use TanStack Query for initial data fetch
  const {
    data: run,
    isLoading: runLoading,
    error: runError,
    refetch: refetchRunDetail,
  } = useRunDetail(projectId, runId);

  // Fetch historical logs
  const {
    data: historicalLogs,
    isLoading: logsLoading,
    refetch: refetchLogs,
  } = useRunLogs(projectId, runId, { perPage: 2000, enabled: !!run });

  // Use mutation for cancelling runs
  const cancelRunMutation = useCancelRun();

  // Local state for real-time updates via SSE
  const [tasks, setTasks] = useState<TaskResult[]>([]);
  const [runStatus, setRunStatus] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | undefined>();
  const [clientDisconnected, setClientDisconnected] = useState(false);
  const [lastHeartbeatAt, setLastHeartbeatAt] = useState<string | undefined>();
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [connected, setConnected] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [taskFilter, setTaskFilter] = useState<string | null>(null);
  const [autoScroll] = useState(true);
  const streamRef = useRef<RunStreamClient | null>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const lastLogIdRef = useRef<number>(0);

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate({ to: "/login" });
    }
  }, [isAuthenticated, authLoading, navigate]);

  // Sync initial data from query to local state
  useEffect(() => {
    if (run) {
      setTasks(run.data.tasks || []);
      setRunStatus(run.data.status);
      setErrorMessage(run.data.error_message);
      setClientDisconnected(run.data.client_disconnected || false);
      setLastHeartbeatAt(run.data.last_heartbeat_at);
    }
  }, [run]);

  // Load historical logs when available
  useEffect(() => {
    if (historicalLogs?.data?.data && historicalLogs.data.data.length > 0) {
      const histLogs: LogLine[] = historicalLogs.data.data.map(
        (log: LogEntry) => ({
          id: log.id,
          task_name: log.task_name,
          stream: log.stream,
          line: log.content,
          line_num: log.line_num,
        }),
      );
      setLogs(histLogs);
      // Track the last log ID for incremental updates
      const maxId = Math.max(...historicalLogs.data.data.map((l) => l.id));
      lastLogIdRef.current = maxId;
    }
  }, [historicalLogs]);

  const updateTaskStatus = useCallback((data: TaskEventData) => {
    setTasks((prev) =>
      prev.map((t) =>
        t.task_name === data.task_name
          ? {
              ...t,
              status: data.status as TaskResult["status"],
              exit_code: data.exit_code,
              duration_ms: data.duration_ms,
              cache_hit: data.cache_hit ?? false,
              cache_key: data.cache_key,
            }
          : t,
      ),
    );
  }, []);

  useEffect(() => {
    if (!isAuthenticated || !run) return;

    // Connect to SSE for real-time updates
    const stream = new RunStreamClient();
    streamRef.current = stream;

    stream.onConnected(() => setConnected(true));
    stream.onError(() => setConnected(false));

    stream.onRunCompleted(() => {
      setRunStatus("success");
    });

    stream.onRunFailed((data) => {
      setRunStatus("failed");
      setErrorMessage(data.error_message);
    });

    stream.onRunCancelled(() => {
      setRunStatus("cancelled");
    });

    stream.onTaskStarted((data) => {
      setTasks((prev) => {
        const exists = prev.find((t) => t.task_name === data.task_name);
        if (exists) {
          return prev.map((t) =>
            t.task_name === data.task_name
              ? { ...t, status: "running" as const }
              : t,
          );
        }
        return [
          ...prev,
          {
            id: crypto.randomUUID(),
            run_id: runId,
            task_name: data.task_name,
            status: "running" as const,
            cache_hit: false,
          },
        ];
      });
    });

    stream.onTaskCompleted(updateTaskStatus);
    stream.onTaskFailed(updateTaskStatus);
    stream.onTaskCached(updateTaskStatus);

    stream.onLog((data: LogEventData) => {
      setLogs((prev) => [
        ...prev,
        {
          task_name: data.task_name,
          stream: data.stream,
          line: data.line,
          line_num: data.line_num,
        },
      ]);
    });

    stream.connect(projectId, runId);

    return () => {
      stream.disconnect();
    };
  }, [projectId, runId, isAuthenticated, run, updateTaskStatus]);

  // Fallback polling when SSE is not connected (e.g. server-side worker runs).
  useEffect(() => {
    if (!run) return;
    const baseStatus = run.data.status;
    const effectiveStatus = runStatus || baseStatus;
    const isTerminal =
      effectiveStatus === "success" ||
      effectiveStatus === "failed" ||
      effectiveStatus === "cancelled";

    // If we are connected to SSE or run is finished, no need to poll.
    if (connected || isTerminal) {
      return;
    }

    const interval = setInterval(() => {
      refetchRunDetail();
      refetchLogs();
    }, 2000);

    return () => clearInterval(interval);
  }, [connected, run, runStatus, refetchRunDetail, refetchLogs]);

  // Auto-scroll logs
  useEffect(() => {
    if (autoScroll) {
      logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, autoScroll]);

  // Filter logs based on search and task filter
  const filteredLogs = logs.filter((log) => {
    if (taskFilter && log.task_name !== taskFilter) return false;
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      return (
        log.line.toLowerCase().includes(query) ||
        log.task_name?.toLowerCase().includes(query)
      );
    }
    return true;
  });

  // Get unique task names for the filter dropdown
  const uniqueTasks = Array.from(
    new Set(logs.map((l) => l.task_name).filter(Boolean)),
  );

  const handleDownloadLogs = () => {
    const content = logs
      .map((log) => `[${log.task_name || "system"}] ${log.line}`)
      .join("\n");
    const blob = new Blob([content], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `run-${runId}-logs.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCancel = () => {
    cancelRunMutation.mutate({ projectId, runId });
  };

  if (authLoading || runLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (runError || !run) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>
            {runError?.message || "Run not found"}
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const currentStatus = runStatus || run.data.status;
  const currentError = errorMessage || run.data.error_message;
  const isClientDisconnected =
    clientDisconnected || run.data.client_disconnected;
  const completedTasks = tasks.filter(
    (t) => t.status === "success" || t.status === "cached",
  ).length;
  const isRunning = currentStatus === "running" || currentStatus === "pending";
  const progress = tasks.length > 0 ? (completedTasks / tasks.length) * 100 : 0;

  // Determine the display status (show 'stale' if client is disconnected while running)
  const displayStatus =
    isRunning && isClientDisconnected ? "stale" : currentStatus;

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects/$projectId" params={{ projectId }}>
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <RunStatusIcon status={displayStatus} className="h-6 w-6" />
            <h1 className="text-2xl font-bold tracking-tight">
              {run.data.workflow_name}
            </h1>
            <StatusBadge status={displayStatus} />
          </div>
          <p className="text-muted-foreground">
            {run.data.trigger_source}
            {run.data.trigger_ref && ` - ${run.data.trigger_ref}`}
            {run.data.commit_sha && (
              <span className="ml-2 font-mono text-xs">
                {run.data.commit_sha.slice(0, 7)}
              </span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Icons.BroadCast
              className={cn(
                "h-3 w-3",
                connected ? "text-green-500" : "text-gray-400",
              )}
            />
            <span className="text-sm text-muted-foreground">
              {connected ? "Live" : "Offline"}
            </span>
          </div>
          {isRunning && (
            <Button
              variant="destructive"
              onClick={handleCancel}
              disabled={cancelRunMutation.isPending}
            >
              {cancelRunMutation.isPending ? (
                <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Icons.Square className="mr-2 h-4 w-4" />
              )}
              Cancel
            </Button>
          )}
        </div>
      </div>

      {/* Stale Run Warning Banner */}
      {isClientDisconnected && isRunning && (
        <div className="rounded-none border border-yellow-500/50 bg-yellow-500/10 p-4">
          <div className="flex items-start gap-3">
            <Icons.WifiOff className="h-5 w-5 text-yellow-500 mt-0.5" />
            <div>
              <h3 className="font-medium text-yellow-500">Connection Lost</h3>
              <p className="text-sm text-muted-foreground">
                The CLI client has lost connection. The run status may be
                outdated.
                {lastHeartbeatAt && (
                  <span className="block mt-1">
                    Last heartbeat: {new Date(lastHeartbeatAt).toLocaleString()}
                  </span>
                )}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Progress */}
      <Card>
        <CardContent className="py-6">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium">Progress</span>
            <span className="text-sm text-muted-foreground">
              {completedTasks} / {tasks.length} tasks
            </span>
          </div>
          <Progress value={progress} className="h-2" />
          {currentError && (
            <div className="mt-4 rounded-none bg-destructive/10 p-3 text-sm text-destructive">
              {currentError}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Tabs for Tasks and Logs */}
      <Tabs defaultValue="logs" className="space-y-4">
        <TabsList>
          <TabsTrigger value="logs" className="gap-2">
            <Icons.Terminal className="h-4 w-4" />
            Logs
          </TabsTrigger>
          <TabsTrigger value="tasks" className="gap-2">
            <Icons.CheckCircle className="h-4 w-4" />
            Tasks ({tasks.length})
          </TabsTrigger>
        </TabsList>

        <TabsContent value="logs">
          <Card className="bg-zinc-950">
            <CardHeader className="border-b border-zinc-800 py-3">
              <div className="flex items-center justify-between gap-4">
                <CardTitle className="text-sm text-zinc-400">Output</CardTitle>
                <div className="flex items-center gap-2 flex-1 max-w-xl">
                  <div className="relative flex-1">
                    <Icons.Search className="absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
                    <Input
                      placeholder="Search logs..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      className="pl-8 h-8 bg-zinc-900 border-zinc-700 text-sm"
                    />
                  </div>
                  {uniqueTasks.length > 0 && (
                    <select
                      value={taskFilter || ""}
                      onChange={(e) => setTaskFilter(e.target.value || null)}
                      className="h-8 px-2 text-sm bg-zinc-900 border border-zinc-700 rounded-none text-zinc-300"
                    >
                      <option value="">All tasks</option>
                      {uniqueTasks.map((task) => (
                        <option key={task} value={task || ""}>
                          {task}
                        </option>
                      ))}
                    </select>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-zinc-500">
                    {filteredLogs.length}
                    {filteredLogs.length !== logs.length &&
                      ` / ${logs.length}`}{" "}
                    lines
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleDownloadLogs}
                    className="h-8 text-zinc-400 hover:text-zinc-100"
                    disabled={logs.length === 0}
                  >
                    <Icons.Download className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </CardHeader>
            <ScrollArea className="h-[500px]">
              <div className="p-4 font-mono text-sm">
                {logsLoading ? (
                  <div className="flex items-center justify-center h-32 text-zinc-500">
                    <Icons.Loader className="h-5 w-5 animate-spin mr-2" />
                    Loading logs...
                  </div>
                ) : filteredLogs.length === 0 ? (
                  <div className="flex items-center justify-center h-32 text-zinc-500">
                    {searchQuery || taskFilter
                      ? "No matching logs"
                      : isRunning
                        ? "Waiting for output..."
                        : "No output"}
                  </div>
                ) : (
                  filteredLogs.map((log, index) => (
                    <div
                      key={log.id || index}
                      className={cn(
                        "flex gap-2 hover:bg-zinc-900/50 px-2 -mx-2 rounded",
                        log.stream === "stderr" && "text-red-400",
                      )}
                    >
                      <span className="select-none text-zinc-600 w-10 text-right shrink-0">
                        {log.line_num}
                      </span>
                      {log.task_name && (
                        <span className="text-zinc-500 shrink-0">
                          [{log.task_name}]
                        </span>
                      )}
                      <span className="text-zinc-200 whitespace-pre-wrap break-all">
                        {log.line}
                      </span>
                    </div>
                  ))
                )}
                <div ref={logsEndRef} />
              </div>
            </ScrollArea>
          </Card>
        </TabsContent>

        <TabsContent value="tasks">
          <Card>
            <CardContent className="py-6">
              {tasks.length === 0 ? (
                <div className="flex items-center justify-center h-32 text-muted-foreground">
                  No tasks yet
                </div>
              ) : (
                <div className="space-y-2">
                  {tasks.map((task) => (
                    <div
                      key={task.id}
                      className="flex items-center justify-between p-3 rounded-none border"
                    >
                      <div className="flex items-center gap-3">
                        <TaskStatusIcon status={task.status} />
                        <div>
                          <p className="font-medium">{task.task_name}</p>
                          <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            {task.duration_ms != null && (
                              <span className="flex items-center gap-1">
                                <Icons.Clock className="h-3 w-3" />
                                {formatDuration(task.duration_ms)}
                              </span>
                            )}
                            {task.cache_hit && (
                              <span className="flex items-center gap-1 text-purple-500">
                                <Icons.Database className="h-3 w-3" />
                                cached
                              </span>
                            )}
                          </div>
                        </div>
                      </div>
                      <StatusBadge status={task.status} />
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function RunStatusIcon({
  status,
  className,
}: {
  status: string;
  className?: string;
}) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className={cn("text-green-500", className)} />;
    case "failed":
      return <Icons.XCircle className={cn("text-red-500", className)} />;
    case "running":
      return (
        <Icons.Loader className={cn("text-blue-500 animate-spin", className)} />
      );
    case "pending":
      return <Icons.Circle className={cn("text-yellow-500", className)} />;
    case "cancelled":
      return <Icons.XCircle className={cn("text-gray-500", className)} />;
    case "stale":
      return <Icons.WifiOff className={cn("text-yellow-500", className)} />;
    default:
      return <Icons.Circle className={cn("text-gray-400", className)} />;
  }
}

function TaskStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "success":
      return <Icons.CheckCircle className="h-5 w-5 text-green-500" />;
    case "failed":
      return <Icons.XCircle className="h-5 w-5 text-red-500" />;
    case "running":
      return <Icons.Loader className="h-5 w-5 text-blue-500 animate-spin" />;
    case "cached":
      return <Icons.Database className="h-5 w-5 text-purple-500" />;
    case "pending":
      return <Icons.Circle className="h-5 w-5 text-yellow-500" />;
    default:
      return <Icons.Circle className="h-5 w-5 text-gray-400" />;
  }
}

function StatusBadge({ status }: { status: string }) {
  const variants: Record<
    string,
    "default" | "secondary" | "destructive" | "outline"
  > = {
    success: "default",
    failed: "destructive",
    running: "default",
    pending: "default",
    cancelled: "secondary",
    cached: "secondary",
    skipped: "outline",
    stale: "outline",
  };

  const labels: Record<string, string> = {
    stale: "Connection Lost",
  };

  return (
    <Badge
      variant={variants[status] || "outline"}
      className={status === "stale" ? "border-yellow-500 text-yellow-500" : ""}
    >
      {labels[status] || status.charAt(0).toUpperCase() + status.slice(1)}
    </Badge>
  );
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}
