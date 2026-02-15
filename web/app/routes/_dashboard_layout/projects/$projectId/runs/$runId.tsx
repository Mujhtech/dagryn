import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useState, useMemo, useRef, useCallback } from "react";
import { useRunArtifacts, useRunDetail, useRunLogs, useRunWorkflow } from "~/hooks/queries";
import { useCancelRun, useDeleteArtifact } from "~/hooks/mutations";
import type { TaskResult, LogEntry, Artifact } from "~/lib/api";
import { RunStreamClient, type LogEventData, type TaskEventData } from "~/lib/sse";
import type { TaskStatusInfo } from "~/components/workflow-dag";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Progress } from "~/components/ui/progress";
import { cn } from "~/lib/utils";
import { Icons } from "~/components/icons";
import {
  RunStatusIcon,
  StatusBadge,
} from "~/components/projects/run-detail/status-ui";
import {
  RunDetailTabs,
  type LogLine,
} from "~/components/projects/run-detail/run-detail-tabs";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/runs/$runId",
)({
  component: RunDetailPage,
});

function RunDetailPage() {
  const { projectId, runId } = Route.useParams();

  const {
    data: run,
    isLoading: runLoading,
    error: runError,
    refetch: refetchRunDetail,
  } = useRunDetail(projectId, runId);

  const {
    data: historicalLogs,
    isLoading: logsLoading,
    refetch: refetchLogs,
  } = useRunLogs(projectId, runId, { perPage: 2000, enabled: !!run });

  const {
    data: artifacts,
    isLoading: artifactsLoading,
    refetch: refetchArtifacts,
  } = useRunArtifacts(projectId, runId);

  const { data: workflow } = useRunWorkflow(projectId, runId);

  const cancelRunMutation = useCancelRun();
  const deleteArtifactMutation = useDeleteArtifact();

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
    if (!run) return;

    setTasks(run.data.tasks || []);
    setRunStatus(run.data.status);
    setErrorMessage(run.data.error_message);
    setClientDisconnected(run.data.client_disconnected || false);
    setLastHeartbeatAt(run.data.last_heartbeat_at);
  }, [run]);

  useEffect(() => {
    if (!historicalLogs?.data?.data || historicalLogs.data.data.length === 0) {
      return;
    }

    const histLogs: LogLine[] = historicalLogs.data.data.map((log: LogEntry) => ({
      id: log.id,
      task_name: log.task_name,
      stream: log.stream,
      line: log.content,
      line_num: log.line_num,
    }));

    setLogs(histLogs);
    const maxId = Math.max(...historicalLogs.data.data.map((entry) => entry.id));
    lastLogIdRef.current = maxId;
  }, [historicalLogs]);

  const updateTaskStatus = useCallback((data: TaskEventData) => {
    setTasks((prev) =>
      prev.map((task) =>
        task.task_name === data.task_name
          ? {
              ...task,
              status: data.status as TaskResult["status"],
              exit_code: data.exit_code,
              duration_ms: data.duration_ms,
              cache_hit: data.cache_hit ?? false,
              cache_key: data.cache_key,
            }
          : task,
      ),
    );
  }, []);

  useEffect(() => {
    if (!run) return;

    const stream = new RunStreamClient();
    streamRef.current = stream;

    stream.onConnected(() => setConnected(true));
    stream.onError(() => setConnected(false));

    stream.onRunCompleted(() => {
      setRunStatus("success");
      refetchArtifacts();
    });

    stream.onRunFailed((data) => {
      setRunStatus("failed");
      setErrorMessage(data.error_message);
      refetchArtifacts();
    });

    stream.onRunCancelled((data) => {
      setRunStatus("cancelled");
      setErrorMessage(data.error_message);
      refetchArtifacts();
    });

    stream.onTaskStarted((data) => {
      setTasks((prev) => {
        const exists = prev.find((task) => task.task_name === data.task_name);
        if (exists) {
          return prev.map((task) =>
            task.task_name === data.task_name
              ? { ...task, status: "running" as const }
              : task,
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
  }, [projectId, runId, run, updateTaskStatus, refetchArtifacts]);

  useEffect(() => {
    if (!run) return;

    const baseStatus = run.data.status;
    const effectiveStatus = runStatus || baseStatus;
    const isTerminal =
      effectiveStatus === "success" ||
      effectiveStatus === "failed" ||
      effectiveStatus === "cancelled";

    if (connected || isTerminal) return;

    const interval = setInterval(() => {
      refetchRunDetail();
      refetchLogs();
      refetchArtifacts();
    }, 2000);

    return () => clearInterval(interval);
  }, [
    connected,
    run,
    runStatus,
    refetchRunDetail,
    refetchLogs,
    refetchArtifacts,
  ]);

  useEffect(() => {
    if (autoScroll) {
      logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, autoScroll]);

  const taskStatusMap = useMemo(() => {
    const map = new Map<string, TaskStatusInfo>();
    for (const task of tasks) {
      map.set(task.task_name, {
        status: task.status,
        duration_ms: task.duration_ms,
        cache_hit: task.cache_hit,
      });
    }
    return map;
  }, [tasks]);

  const handleCancel = () => {
    cancelRunMutation.mutate(
      { projectId, runId },
      {
        onSuccess: () => {
          setRunStatus("cancelled");
          setErrorMessage("Cancelled by user");
        },
      },
    );
  };

  const handleDeleteArtifact = (artifact: Artifact) => {
    const confirmed = window.confirm(
      `Delete artifact "${artifact.name}"? This cannot be undone.`,
    );
    if (!confirmed) return;

    deleteArtifactMutation.mutate({
      projectId,
      runId,
      artifactId: artifact.id,
    });
  };

  if (runLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  if (runError || !run) {
    return (
      <Card className="border-destructive">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>{runError?.message || "Run not found"}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const currentStatus = runStatus || run.data.status;
  const currentError = errorMessage || run.data.error_message;
  const isClientDisconnected = clientDisconnected || run.data.client_disconnected;
  const completedTasks = tasks.filter(
    (task) => task.status === "success" || task.status === "cached",
  ).length;
  const isRunning = currentStatus === "running" || currentStatus === "pending";
  const progress = tasks.length > 0 ? (completedTasks / tasks.length) * 100 : 0;
  const artifactsList = artifacts?.data ?? [];
  const displayStatus = isRunning && isClientDisconnected ? "stale" : currentStatus;

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/projects/$projectId" params={{ projectId }}>
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <RunStatusIcon status={displayStatus} className="h-6 w-6" />
            <h1 className="text-2xl font-bold tracking-tight">{run.data.workflow_name}</h1>
            <StatusBadge status={displayStatus} />
          </div>
          <p className="text-muted-foreground">
            {run.data.trigger_source}
            {run.data.trigger_ref && ` - ${run.data.trigger_ref}`}
            {run.data.commit_sha ? (
              <span className="ml-2 font-mono text-xs">{run.data.commit_sha.slice(0, 7)}</span>
            ) : null}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Icons.BroadCast
              className={cn("h-3 w-3", connected ? "text-green-500" : "text-gray-400")}
            />
            <span className="text-sm text-muted-foreground">
              {connected ? "Live" : "Offline"}
            </span>
          </div>
          {isRunning ? (
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
              {cancelRunMutation.isPending ? "Cancelling..." : "Cancel"}
            </Button>
          ) : null}
        </div>
      </div>

      {isClientDisconnected && isRunning ? (
        <div className="rounded-none border border-yellow-500/50 bg-yellow-500/10 p-4">
          <div className="flex items-start gap-3">
            <Icons.WifiOff className="h-5 w-5 text-yellow-500 mt-0.5" />
            <div>
              <h3 className="font-medium text-yellow-500">Connection Lost</h3>
              <p className="text-sm text-muted-foreground">
                The CLI client has lost connection. The run status may be outdated.
                {lastHeartbeatAt ? (
                  <span className="block mt-1">
                    Last heartbeat: {new Date(lastHeartbeatAt).toLocaleString()}
                  </span>
                ) : null}
              </p>
            </div>
          </div>
        </div>
      ) : null}

      <Card>
        <CardContent className="py-6">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium">Progress</span>
            <span className="text-sm text-muted-foreground">
              {completedTasks} / {tasks.length} tasks
            </span>
          </div>
          <Progress value={progress} className="h-2" />
          {currentError ? (
            <div className="mt-4 rounded-none bg-destructive/10 p-3 text-sm text-destructive">
              {currentError}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <RunDetailTabs
        runId={runId}
        projectId={projectId}
        tasks={tasks}
        logs={logs}
        logsLoading={logsLoading}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        taskFilter={taskFilter}
        setTaskFilter={setTaskFilter}
        isRunning={isRunning}
        artifactsLoading={artifactsLoading}
        artifacts={artifactsList}
        deleteArtifactPending={deleteArtifactMutation.isPending}
        onDeleteArtifact={handleDeleteArtifact}
        workflow={workflow}
        taskStatusMap={taskStatusMap}
        logsEndRef={logsEndRef}
        runStatus={currentStatus}
      />
    </div>
  );
}
