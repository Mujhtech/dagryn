import type { RefObject } from "react";
import type { Artifact, TaskResult } from "~/lib/api";
import { api } from "~/lib/api";
import { WorkflowDag, type TaskStatusInfo } from "~/components/workflow-dag";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { ScrollArea } from "~/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Input } from "~/components/ui/input";
import { Icons } from "~/components/icons";
import { cn } from "~/lib/utils";
import {
  TaskStatusIcon,
  StatusBadge,
  formatBytes,
  formatDuration,
} from "./status-ui";
import { AIAnalysisTab } from "./ai-analysis-tab";

export interface LogLine {
  id?: number;
  task_name?: string;
  stream: "stdout" | "stderr";
  line: string;
  line_num: number;
}

type RunDetailTabsProps = {
  runId: string;
  projectId: string;
  tasks: TaskResult[];
  logs: LogLine[];
  logsLoading: boolean;
  searchQuery: string;
  setSearchQuery: (value: string) => void;
  taskFilter: string | null;
  setTaskFilter: (value: string | null) => void;
  isRunning: boolean;
  artifactsLoading: boolean;
  artifacts: Artifact[];
  deleteArtifactPending: boolean;
  onDeleteArtifact: (artifact: Artifact) => void;
  workflow?: Parameters<typeof WorkflowDag>[0]["workflow"];
  taskStatusMap: Map<string, TaskStatusInfo>;
  logsEndRef: RefObject<HTMLDivElement | null>;
  runStatus?: string;
};

export function RunDetailTabs({
  runId,
  projectId,
  tasks,
  logs,
  logsLoading,
  searchQuery,
  setSearchQuery,
  taskFilter,
  setTaskFilter,
  isRunning,
  artifactsLoading,
  artifacts,
  deleteArtifactPending,
  onDeleteArtifact,
  workflow,
  taskStatusMap,
  logsEndRef,
  runStatus,
}: RunDetailTabsProps) {
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

  const uniqueTasks = Array.from(
    new Set(logs.map((log) => log.task_name).filter(Boolean)),
  );

  const handleDownloadLogs = () => {
    const content = logs
      .map((log) => `[${log.task_name || "system"}] ${log.line}`)
      .join("\n");
    const blob = new Blob([content], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `run-${runId}-logs.txt`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  const handleDownloadArtifact = async (artifact: Artifact) => {
    const result = await api.downloadArtifact(projectId, runId, artifact.id);
    const url = URL.createObjectURL(result.blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = result.filename || artifact.file_name || "artifact";
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
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
        <TabsTrigger value="artifacts" className="gap-2">
          <Icons.HardDrive className="h-4 w-4" />
          Artifacts ({artifacts.length})
        </TabsTrigger>
        {workflow ? (
          <TabsTrigger value="dag" className="gap-2">
            <Icons.Network className="h-4 w-4" />
            DAG
          </TabsTrigger>
        ) : null}
        <TabsTrigger value="ai" className="gap-2">
          <Icons.Lightbulb className="h-4 w-4" />
          AI
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
                    onChange={(event) => setSearchQuery(event.target.value)}
                    className="pl-8 h-8 bg-zinc-900 border-zinc-700 text-sm"
                  />
                </div>
                {uniqueTasks.length > 0 ? (
                  <select
                    value={taskFilter || ""}
                    onChange={(event) => setTaskFilter(event.target.value || null)}
                    className="h-8 px-2 text-sm bg-zinc-900 border border-zinc-700 rounded-none text-zinc-300"
                  >
                    <option value="">All tasks</option>
                    {uniqueTasks.map((task) => (
                      <option key={task} value={task || ""}>
                        {task}
                      </option>
                    ))}
                  </select>
                ) : null}
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-zinc-500">
                  {filteredLogs.length}
                  {filteredLogs.length !== logs.length ? ` / ${logs.length}` : ""} lines
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
          <ScrollArea className="h-125">
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
                    {log.task_name ? (
                      <span className="text-zinc-500 shrink-0">[{log.task_name}]</span>
                    ) : null}
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
                          {task.duration_ms != null ? (
                            <span className="flex items-center gap-1">
                              <Icons.Clock className="h-3 w-3" />
                              {formatDuration(task.duration_ms)}
                            </span>
                          ) : null}
                          {task.cache_hit ? (
                            <span className="flex items-center gap-1 text-purple-500">
                              <Icons.Database className="h-3 w-3" />
                              cached
                            </span>
                          ) : null}
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

      <TabsContent value="artifacts">
        <Card>
          <CardHeader>
            <CardTitle>Artifacts</CardTitle>
            <CardDescription>
              Files captured from task outputs for this run.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {artifactsLoading ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                <Icons.Loader className="h-5 w-5 animate-spin mr-2" />
                Loading artifacts...
              </div>
            ) : artifacts.length === 0 ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                No artifacts yet
              </div>
            ) : (
              <div className="space-y-2">
                {artifacts.map((artifact) => (
                  <div
                    key={artifact.id}
                    className="flex items-center justify-between p-3 rounded-none border"
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <p className="font-medium truncate">{artifact.name}</p>
                        {artifact.task_name ? (
                          <Badge variant="secondary">{artifact.task_name}</Badge>
                        ) : null}
                      </div>
                      <div className="flex items-center gap-3 text-xs text-muted-foreground">
                        <span>{artifact.file_name}</span>
                        <span>{formatBytes(artifact.size_bytes)}</span>
                        <span>{new Date(artifact.created_at).toLocaleString()}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDownloadArtifact(artifact)}
                        className="h-8"
                      >
                        <Icons.Download className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => onDeleteArtifact(artifact)}
                        className="h-8 text-destructive hover:text-destructive"
                        disabled={deleteArtifactPending}
                      >
                        <Icons.Trash className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </TabsContent>

      {workflow ? (
        <TabsContent value="dag">
          <Card>
            <CardContent className="py-6">
              <WorkflowDag workflow={workflow} taskStatuses={taskStatusMap} />
            </CardContent>
          </Card>
        </TabsContent>
      ) : null}

      <TabsContent value="ai">
        <AIAnalysisTab
          projectId={projectId}
          runId={runId}
          runStatus={runStatus ?? ""}
        />
      </TabsContent>
    </Tabs>
  );
}
