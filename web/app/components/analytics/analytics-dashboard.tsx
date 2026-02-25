import { Link } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "~/components/ui/chart";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";
import { Icons } from "~/components/icons";
import type { AnalyticsOverview } from "~/lib/api";

interface AnalyticsDashboardProps {
  data: AnalyticsOverview | undefined;
  isLoading: boolean;
  days: number;
  onDaysChange: (days: number) => void;
  title: string;
  subtitle: string;
  backLink?: { to: string; params?: Record<string, string> };
  badgeLabel?: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

// function formatDuration(ms: number): string {
//   if (ms < 1000) return `${ms}ms`;
//   const seconds = ms / 1000;
//   if (seconds < 60) return `${seconds.toFixed(1)}s`;
//   const minutes = seconds / 60;
//   return `${minutes.toFixed(1)}m`;
// }

function formatDate(value: string): string {
  const d = new Date(value);
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

// Chart configs
const runChartConfig: ChartConfig = {
  success: {
    label: "Success",
    color: "var(--color-emerald-500, #10b981)",
  },
  failed: {
    label: "Failed",
    color: "var(--color-rose-500, #f43f5e)",
  },
  cancelled: {
    label: "Cancelled",
    color: "var(--color-amber-500, #f59e0b)",
  },
};

const cacheChartConfig: ChartConfig = {
  cache_hits: {
    label: "Hits",
    color: "var(--color-emerald-500, #10b981)",
  },
  cache_misses: {
    label: "Misses",
    color: "var(--color-rose-500, #f43f5e)",
  },
};

const bandwidthChartConfig: ChartConfig = {
  upload_bytes: {
    label: "Upload",
    color: "var(--color-blue-500, #3b82f6)",
  },
  download_bytes: {
    label: "Download",
    color: "var(--color-violet-500, #8b5cf6)",
  },
};

const artifactChartConfig: ChartConfig = {
  count: {
    label: "Artifacts",
    color: "var(--color-blue-500, #3b82f6)",
  },
  size_bytes: {
    label: "Size",
    color: "var(--color-violet-500, #8b5cf6)",
  },
};

const aiChartConfig: ChartConfig = {
  analyses: {
    label: "Analyses",
    color: "var(--color-blue-500, #3b82f6)",
  },
  suggestions: {
    label: "Suggestions",
    color: "var(--color-violet-500, #8b5cf6)",
  },
};

const auditChartConfig: ChartConfig = {
  events: {
    label: "Events",
    color: "var(--color-slate-500, #64748b)",
  },
};

export function AnalyticsDashboard({
  data,
  isLoading,
  days,
  onDaysChange,
  title,
  subtitle,
  backLink,
  badgeLabel,
}: AnalyticsDashboardProps) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const storageTotal =
    (data?.cache.total_size_bytes ?? 0) +
    (data?.artifacts.total_size_bytes ?? 0);

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      {/* Header */}
      <div className="flex items-center gap-4">
        {backLink && (
          <Button variant="ghost" size="icon" asChild>
            <Link to={backLink.to} params={backLink.params}>
              <Icons.ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
        )}
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
            {badgeLabel && <Badge variant="secondary">{badgeLabel}</Badge>}
          </div>
          <p className="text-muted-foreground">{subtitle}</p>
        </div>
        <Select
          value={String(days)}
          onValueChange={(v) => onDaysChange(Number(v))}
        >
          <SelectTrigger className="w-35">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="7">Last 7 days</SelectItem>
            <SelectItem value="14">Last 14 days</SelectItem>
            <SelectItem value="30">Last 30 days</SelectItem>
            <SelectItem value="90">Last 90 days</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Total Runs</CardTitle>
            <Icons.Play className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {data?.runs.total_runs ?? 0}
            </div>
            <p className="text-xs text-muted-foreground">
              {data?.runs.success_rate?.toFixed(1) ?? 0}% success rate
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">
              Cache Hit Rate
            </CardTitle>
            <Icons.Target className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {data?.cache.hit_rate?.toFixed(1) ?? 0}%
            </div>
            <p className="text-xs text-muted-foreground">
              {data?.cache.total_hits ?? 0} hits /{" "}
              {(data?.cache.total_hits ?? 0) + (data?.cache.total_misses ?? 0)}{" "}
              total
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Storage</CardTitle>
            <Icons.HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatBytes(storageTotal)}
            </div>
            <p className="text-xs text-muted-foreground">
              Cache: {formatBytes(data?.cache.total_size_bytes ?? 0)},
              Artifacts: {formatBytes(data?.artifacts.total_size_bytes ?? 0)}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Bandwidth</CardTitle>
            <Icons.TrendUp className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatBytes(data?.bandwidth.total_bytes ?? 0)}
            </div>
            <p className="text-xs text-muted-foreground">
              Upload {formatBytes(data?.bandwidth.upload_bytes ?? 0)} / Download{" "}
              {formatBytes(data?.bandwidth.download_bytes ?? 0)}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Run & Cache Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Run Activity</CardTitle>
            <CardDescription>
              Daily run outcomes across all projects
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ChartSection
              chart={data?.runs.chart}
              config={runChartConfig}
              renderChart={(chartData) => (
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatDate}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Bar
                    dataKey="success"
                    fill="var(--color-success)"
                    radius={[0, 0, 0, 0]}
                    stackId="a"
                  />
                  <Bar
                    dataKey="failed"
                    fill="var(--color-failed)"
                    radius={[0, 0, 0, 0]}
                    stackId="a"
                  />
                  <Bar
                    dataKey="cancelled"
                    fill="var(--color-cancelled)"
                    radius={[0, 0, 0, 0]}
                    stackId="a"
                  />
                </BarChart>
              )}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Cache Performance</CardTitle>
            <CardDescription>Daily cache hits vs misses</CardDescription>
          </CardHeader>
          <CardContent>
            <ChartSection
              chart={data?.cache.chart}
              config={cacheChartConfig}
              renderChart={(chartData) => (
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatDate}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Bar
                    dataKey="cache_hits"
                    fill="var(--color-cache_hits)"
                    radius={[0, 0, 0, 0]}
                    stackId="a"
                  />
                  <Bar
                    dataKey="cache_misses"
                    fill="var(--color-cache_misses)"
                    radius={[0, 0, 0, 0]}
                    stackId="a"
                  />
                </BarChart>
              )}
            />
          </CardContent>
        </Card>
      </div>

      {/* Bandwidth & Artifact Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Bandwidth Usage</CardTitle>
            <CardDescription>Daily upload and download volume</CardDescription>
          </CardHeader>
          <CardContent>
            <ChartSection
              chart={data?.bandwidth.chart}
              config={bandwidthChartConfig}
              renderChart={(chartData) => (
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatDate}
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(value) => formatBytes(value)}
                  />
                  <ChartTooltip
                    content={
                      <ChartTooltipContent
                        formatter={(value) => formatBytes(value as number)}
                      />
                    }
                  />
                  <Area
                    dataKey="upload_bytes"
                    fill="var(--color-upload_bytes)"
                    fillOpacity={0.2}
                    stroke="var(--color-upload_bytes)"
                    stackId="a"
                    type="monotone"
                  />
                  <Area
                    dataKey="download_bytes"
                    fill="var(--color-download_bytes)"
                    fillOpacity={0.2}
                    stroke="var(--color-download_bytes)"
                    stackId="a"
                    type="monotone"
                  />
                </AreaChart>
              )}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Artifact Growth</CardTitle>
            <CardDescription>Daily artifact count and size</CardDescription>
          </CardHeader>
          <CardContent>
            <ChartSection
              chart={data?.artifacts.chart}
              config={artifactChartConfig}
              renderChart={(chartData) => (
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatDate}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Area
                    dataKey="count"
                    fill="var(--color-count)"
                    fillOpacity={0.2}
                    stroke="var(--color-count)"
                    type="monotone"
                  />
                </AreaChart>
              )}
            />
          </CardContent>
        </Card>
      </div>

      {/* AI & Audit Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>AI Analysis Activity</CardTitle>
            <CardDescription>
              Daily analyses and suggestions generated
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ChartSection
              chart={data?.ai.chart}
              config={aiChartConfig}
              renderChart={(chartData) => (
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatDate}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Bar
                    dataKey="analyses"
                    fill="var(--color-analyses)"
                    radius={[0, 0, 0, 0]}
                  />
                  <Bar
                    dataKey="suggestions"
                    fill="var(--color-suggestions)"
                    radius={[0, 0, 0, 0]}
                  />
                </BarChart>
              )}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Audit Log Activity</CardTitle>
                <CardDescription>
                  Daily audit events ({data?.audit_log.total_events ?? 0} total)
                </CardDescription>
              </div>
              {data?.audit_log.top_actions &&
                data.audit_log.top_actions.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {data.audit_log.top_actions.slice(0, 3).map((a) => (
                      <Badge
                        key={a.action}
                        variant="outline"
                        className="text-xs"
                      >
                        {a.action} ({a.count})
                      </Badge>
                    ))}
                  </div>
                )}
            </div>
          </CardHeader>
          <CardContent>
            <ChartSection
              chart={data?.audit_log.chart}
              config={auditChartConfig}
              renderChart={(chartData) => (
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={formatDate}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Area
                    dataKey="events"
                    fill="var(--color-events)"
                    fillOpacity={0.2}
                    stroke="var(--color-events)"
                    type="monotone"
                  />
                </AreaChart>
              )}
            />
          </CardContent>
        </Card>
      </div>

      {/* Project Leaderboard */}
      {data?.projects && data.projects.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Project Leaderboard</CardTitle>
            <CardDescription>
              Activity summary across {data.projects.length} projects
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 px-3 font-medium text-muted-foreground">
                      Project
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Runs
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Success Rate
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Cache
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Artifacts
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Bandwidth
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.projects.map((project) => (
                    <tr
                      key={project.project_id}
                      className="border-b last:border-b-0 hover:bg-muted/50"
                    >
                      <td className="py-2 px-3">
                        <Link
                          to="/projects/$projectId"
                          params={{ projectId: project.project_id }}
                          className="font-medium text-primary hover:underline"
                        >
                          {project.project_name}
                        </Link>
                      </td>
                      <td className="py-2 px-3 text-right font-mono">
                        {project.total_runs}
                      </td>
                      <td className="py-2 px-3 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <div className="w-16 h-2 bg-muted rounded-full overflow-hidden">
                            <div
                              className="h-full bg-emerald-500 rounded-full"
                              style={{
                                width: `${Math.min(project.success_rate, 100)}%`,
                              }}
                            />
                          </div>
                          <span className="font-mono text-xs w-12 text-right">
                            {project.success_rate.toFixed(1)}%
                          </span>
                        </div>
                      </td>
                      <td className="py-2 px-3 text-right font-mono">
                        {formatBytes(project.cache_size_bytes)}
                      </td>
                      <td className="py-2 px-3 text-right font-mono">
                        {formatBytes(project.artifact_size_bytes)}
                      </td>
                      <td className="py-2 px-3 text-right font-mono">
                        {formatBytes(project.bandwidth_bytes)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

// Reusable chart section with loading/empty states
function ChartSection<T>({
  chart,
  config,
  renderChart,
}: {
  chart: T[] | undefined;
  config: ChartConfig;
  renderChart: (data: T[]) => React.ReactElement;
}) {
  if (!chart || chart.length === 0) {
    return (
      <div className="flex items-center justify-center h-75 text-muted-foreground">
        No data available for the selected period
      </div>
    );
  }

  return (
    <ChartContainer config={config} className="h-75 w-full">
      {renderChart(chart)}
    </ChartContainer>
  );
}
