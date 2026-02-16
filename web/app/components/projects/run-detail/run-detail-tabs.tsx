import { type RefObject, useMemo, useState } from "react";
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
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "~/components/ui/accordion";
import { ToggleGroup, ToggleGroupItem } from "~/components/ui/toggle-group";
import { Icons } from "~/components/icons";
import { cn } from "~/lib/utils";
import {
  TaskStatusIcon,
  StatusBadge,
  formatBytes,
  formatDuration,
} from "./status-ui";
import { AIAnalysisTab } from "./ai-analysis-tab";
import { TasksWaterfall } from "./tasks-waterfall";
// import {
//   Select,
//   SelectContent,
//   SelectGroup,
//   SelectItem,
//   SelectTrigger,
//   SelectValue,
// } from "~/components/ui/select";

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
  const [tasksView, setTasksView] = useState<"table" | "canvas" | "waterfall">(
    "table",
  );

  const filteredLogs = useMemo(() => {
    if (!searchQuery) return logs;
    const query = searchQuery.toLowerCase();
    return logs.filter(
      (log) =>
        log.line.toLowerCase().includes(query) ||
        log.task_name?.toLowerCase().includes(query),
    );
  }, [logs, searchQuery]);

  const groupedLogs = useMemo(() => {
    const groups = new Map<string, LogLine[]>();
    for (const log of filteredLogs) {
      const key = log.task_name || "__system__";
      const list = groups.get(key) || [];
      list.push(log);
      groups.set(key, list);
    }
    return groups;
  }, [filteredLogs]);

  const expandedGroups = useMemo(() => {
    if (searchQuery) return Array.from(groupedLogs.keys());
    return [];
  }, [searchQuery, groupedLogs]);

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
        <TabsTrigger value="ai" className="gap-2">
          <Icons.Lightbulb className="h-4 w-4" />
          AI
        </TabsTrigger>
      </TabsList>

      {/* Logs Tab - Accordion by task */}
      <TabsContent value="logs">
        <Card className="bg-zinc-950 px-0 py-0 gap-0">
          <CardHeader className="border-b border-zinc-800 pt-3 px-4 gap-0 [.border-b]:pb-3">
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
                {/* <Select>
                  <SelectTrigger className="w-45">
                    <SelectValue placeholder="Theme" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="light">Light</SelectItem>
                      <SelectItem value="dark">Dark</SelectItem>
                      <SelectItem value="system">System</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select> */}
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-zinc-500">
                  {filteredLogs.length}
                  {filteredLogs.length !== logs.length
                    ? ` / ${logs.length}`
                    : ""}{" "}
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
          <ScrollArea className="h-125">
            <div className="p-4 font-mono text-sm">
              {logsLoading ? (
                <div className="flex items-center justify-center h-32 text-zinc-500">
                  <Icons.Loader className="h-5 w-5 animate-spin mr-2" />
                  Loading logs...
                </div>
              ) : groupedLogs.size === 0 ? (
                <div className="flex items-center justify-center h-32 text-zinc-500">
                  {searchQuery
                    ? "No matching logs"
                    : isRunning
                      ? "Waiting for output..."
                      : "No output"}
                </div>
              ) : (
                <Accordion
                  type="multiple"
                  defaultValue={expandedGroups}
                  key={searchQuery}
                >
                  {Array.from(groupedLogs.entries()).map(
                    ([groupKey, groupLogs]) => {
                      const displayName =
                        groupKey === "__system__" ? "System" : groupKey;
                      const taskStatus = taskStatusMap.get(groupKey);

                      return (
                        <AccordionItem
                          key={groupKey}
                          value={groupKey}
                          className="border-zinc-800"
                        >
                          <AccordionTrigger className="text-zinc-300 hover:no-underline py-2 px-2">
                            <div className="flex items-center gap-2">
                              {taskStatus ? (
                                <TaskStatusIcon status={taskStatus.status} />
                              ) : (
                                <Icons.Terminal className="h-4 w-4 text-zinc-500" />
                              )}
                              <span className="font-medium">{displayName}</span>
                              <span className="text-xs text-zinc-500">
                                ({groupLogs.length} lines)
                              </span>
                            </div>
                          </AccordionTrigger>
                          <AccordionContent className="pb-0">
                            <div className="pl-2">
                              {groupLogs.map((log, index) => (
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
                                  <span className="text-zinc-200 whitespace-pre-wrap break-all">
                                    {log.line}
                                  </span>
                                </div>
                              ))}
                            </div>
                          </AccordionContent>
                        </AccordionItem>
                      );
                    },
                  )}
                </Accordion>
              )}
              <div ref={logsEndRef} />
            </div>
          </ScrollArea>
        </Card>
      </TabsContent>

      {/* Tasks Tab - Canvas / Table / Waterfall views */}
      <TabsContent value="tasks">
        <Card className="py-0 gap-0">
          <CardContent className="py-4 px-4">
            <div className="flex items-center justify-between mb-4">
              <ToggleGroup
                type="single"
                value={tasksView}
                onValueChange={(v) => v && setTasksView(v as typeof tasksView)}
                variant="outline"
                className="rounded-none"
                size="sm"
              >
                <ToggleGroupItem value="canvas">
                  <Icons.Network className="h-3.5 w-3.5 mr-1" />
                  Canvas
                </ToggleGroupItem>
                <ToggleGroupItem value="table">
                  <Icons.ListDetails className="h-3.5 w-3.5 mr-1" />
                  Table
                </ToggleGroupItem>
                <ToggleGroupItem value="waterfall">
                  <Icons.GripVertical className="h-3.5 w-3.5 mr-1" />
                  Waterfall
                </ToggleGroupItem>
              </ToggleGroup>
            </div>

            {tasks.length === 0 ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                No tasks yet
              </div>
            ) : tasksView === "canvas" ? (
              workflow ? (
                <WorkflowDag workflow={workflow} taskStatuses={taskStatusMap} />
              ) : (
                <div className="flex items-center justify-center h-32 text-muted-foreground">
                  No workflow data
                </div>
              )
            ) : tasksView === "waterfall" ? (
              <TasksWaterfall tasks={tasks} />
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
          <CardHeader className="px-4">
            <CardTitle>Artifacts</CardTitle>
            <CardDescription>
              Files captured from task outputs for this run.
            </CardDescription>
          </CardHeader>
          <CardContent className="px-4">
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
                          <Badge variant="secondary">
                            {artifact.task_name}
                          </Badge>
                        ) : null}
                      </div>
                      <div className="flex items-center gap-3 text-xs text-muted-foreground">
                        <span>{artifact.file_name}</span>
                        <span>{formatBytes(artifact.size_bytes)}</span>
                        <span>
                          {new Date(artifact.created_at).toLocaleString()}
                        </span>
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
