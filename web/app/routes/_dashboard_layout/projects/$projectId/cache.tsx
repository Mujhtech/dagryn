import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { useProject, useCacheStats, useCacheAnalytics } from "~/hooks/queries";
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
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/cache",
)({
  component: CacheAnalyticsPage,
  head: () => {
    return generateMetadata({ title: "Cache Analytics" });
  },
});

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function CacheAnalyticsPage() {
  const { projectId } = Route.useParams();

  const [days, setDays] = useState(30);

  const { data: project, isLoading: projectLoading } = useProject(projectId);
  const { data: stats, isLoading: statsLoading } = useCacheStats(projectId);
  const { data: analytics, isLoading: analyticsLoading } = useCacheAnalytics(
    projectId,
    days,
  );

  const loading = projectLoading;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!project) {
    return (
      <div className="flex items-center justify-center h-screen">
        <p className="text-muted-foreground">Project not found</p>
      </div>
    );
  }

  const hitRateChartConfig: ChartConfig = {
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
    bytes_uploaded: {
      label: "Uploaded",
      color: "var(--color-blue-500, #3b82f6)",
    },
    bytes_downloaded: {
      label: "Downloaded",
      color: "var(--color-violet-500, #8b5cf6)",
    },
  };

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
            <h1 className="text-3xl font-bold tracking-tight">
              Cache Analytics
            </h1>
            <Badge variant="secondary">{project.name}</Badge>
          </div>
          <p className="text-muted-foreground">
            Monitor cache usage, hit rates, and storage consumption
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select
            value={String(days)}
            onValueChange={(v) => setDays(Number(v))}
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
          <Button variant="outline" size="icon" asChild>
            <Link to="/projects/$projectId/settings" params={{ projectId }}>
              <Icons.Settings className="h-4 w-4" />
            </Link>
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Hit Rate</CardTitle>
            <Icons.Target className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {analyticsLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin" />
            ) : (
              <>
                <div className="text-2xl font-bold">
                  {analytics?.hit_rate?.toFixed(1) ?? 0}%
                </div>
                <p className="text-xs text-muted-foreground">
                  {analytics?.total_hits ?? 0} hits /{" "}
                  {(analytics?.total_hits ?? 0) +
                    (analytics?.total_misses ?? 0)}{" "}
                  total
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Storage Used</CardTitle>
            <Icons.HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {statsLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin" />
            ) : (
              <>
                <div className="text-2xl font-bold">
                  {formatBytes(stats?.total_size_bytes ?? 0)}
                </div>
                <div className="mt-2">
                  <Progress
                    value={stats?.quota_used_pct ?? 0}
                    className="h-2"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {(stats?.quota_used_pct ?? 0).toFixed(1)}% of quota used
                  </p>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Cache Entries</CardTitle>
            <Icons.Database className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {statsLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin" />
            ) : (
              <>
                <div className="text-2xl font-bold">
                  {stats?.total_entries ?? 0}
                </div>
                <p className="text-xs text-muted-foreground">
                  {stats?.hit_count ?? 0} total hits across all entries
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Bandwidth</CardTitle>
            <Icons.TrendUp className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {analyticsLoading ? (
              <Icons.Loader className="h-4 w-4 animate-spin" />
            ) : (
              <>
                <div className="text-2xl font-bold">
                  {formatBytes(
                    (analytics?.total_bytes_uploaded ?? 0) +
                      (analytics?.total_bytes_downloaded ?? 0),
                  )}
                </div>
                <div className="flex items-center gap-3 mt-1">
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Icons.Upload className="h-3 w-3" />
                    {formatBytes(analytics?.total_bytes_uploaded ?? 0)}
                  </span>
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Icons.Download className="h-3 w-3" />
                    {formatBytes(analytics?.total_bytes_downloaded ?? 0)}
                  </span>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Hit Rate Chart */}
        <Card>
          <CardHeader>
            <CardTitle>Cache Hits vs Misses</CardTitle>
            <CardDescription>Daily cache hit and miss counts</CardDescription>
          </CardHeader>
          <CardContent>
            {analyticsLoading ? (
              <div className="flex items-center justify-center h-75">
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : analytics?.days && analytics.days.length > 0 ? (
              <ChartContainer config={hitRateChartConfig} className="h-75">
                <BarChart data={analytics.days}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(value) => {
                      const d = new Date(value);
                      return `${d.getMonth() + 1}/${d.getDate()}`;
                    }}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Bar
                    dataKey="cache_hits"
                    fill="var(--color-cache_hits)"
                    radius={[4, 4, 0, 0]}
                    stackId="a"
                  />
                  <Bar
                    dataKey="cache_misses"
                    fill="var(--color-cache_misses)"
                    radius={[4, 4, 0, 0]}
                    stackId="a"
                  />
                </BarChart>
              </ChartContainer>
            ) : (
              <div className="flex items-center justify-center h-75 text-muted-foreground">
                No data available for the selected period
              </div>
            )}
          </CardContent>
        </Card>

        {/* Bandwidth Chart */}
        <Card>
          <CardHeader>
            <CardTitle>Bandwidth Usage</CardTitle>
            <CardDescription>
              Daily upload and download volume (bytes)
            </CardDescription>
          </CardHeader>
          <CardContent>
            {analyticsLoading ? (
              <div className="flex items-center justify-center h-75">
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : analytics?.days && analytics.days.length > 0 ? (
              <ChartContainer config={bandwidthChartConfig} className="h-75">
                <AreaChart data={analytics.days}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(value) => {
                      const d = new Date(value);
                      return `${d.getMonth() + 1}/${d.getDate()}`;
                    }}
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
                    dataKey="bytes_uploaded"
                    fill="var(--color-bytes_uploaded)"
                    fillOpacity={0.2}
                    stroke="var(--color-bytes_uploaded)"
                    stackId="a"
                    type="monotone"
                  />
                  <Area
                    dataKey="bytes_downloaded"
                    fill="var(--color-bytes_downloaded)"
                    fillOpacity={0.2}
                    stroke="var(--color-bytes_downloaded)"
                    stackId="a"
                    type="monotone"
                  />
                </AreaChart>
              </ChartContainer>
            ) : (
              <div className="flex items-center justify-center h-75 text-muted-foreground">
                No data available for the selected period
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Top Tasks by Cache Size */}
      {stats?.top_tasks && stats.top_tasks.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Top Tasks by Cache Size</CardTitle>
            <CardDescription>
              Tasks consuming the most cache storage
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {stats.top_tasks.map((task) => {
                const pct =
                  stats.total_size_bytes > 0
                    ? (task.size_bytes / stats.total_size_bytes) * 100
                    : 0;
                return (
                  <div key={task.task_name} className="space-y-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-mono font-medium">
                          {task.task_name}
                        </span>
                        <Badge variant="outline" className="text-xs">
                          {task.entries}{" "}
                          {task.entries === 1 ? "entry" : "entries"}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-3 text-sm text-muted-foreground">
                        <span>{formatBytes(task.size_bytes)}</span>
                        <span>{task.total_hits} hits</span>
                      </div>
                    </div>
                    <Progress value={pct} className="h-2" />
                  </div>
                );
              })}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Daily Usage Table */}
      {analytics?.days && analytics.days.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Daily Usage</CardTitle>
            <CardDescription>
              Detailed daily cache usage breakdown
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 px-3 font-medium text-muted-foreground">
                      Date
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Hits
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Misses
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Hit Rate
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Uploaded
                    </th>
                    <th className="text-right py-2 px-3 font-medium text-muted-foreground">
                      Downloaded
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {[...analytics.days].reverse().map((day) => (
                    <tr
                      key={day.date}
                      className="border-b last:border-b-0 hover:bg-muted/50"
                    >
                      <td className="py-2 px-3 font-mono">{day.date}</td>
                      <td className="py-2 px-3 text-right text-emerald-600">
                        {day.cache_hits}
                      </td>
                      <td className="py-2 px-3 text-right text-rose-600">
                        {day.cache_misses}
                      </td>
                      <td className="py-2 px-3 text-right">
                        {day.hit_rate.toFixed(1)}%
                      </td>
                      <td className="py-2 px-3 text-right">
                        {formatBytes(day.bytes_uploaded)}
                      </td>
                      <td className="py-2 px-3 text-right">
                        {formatBytes(day.bytes_downloaded)}
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
