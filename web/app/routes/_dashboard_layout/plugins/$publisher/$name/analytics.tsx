import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
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
import {
  useRegistryPluginDetail,
  useRegistryPluginVersions,
} from "~/hooks/queries";
import { usePluginAnalytics } from "~/hooks/queries";
import { Icons } from "~/components/icons";

export const Route = createFileRoute(
  "/_dashboard_layout/plugins/$publisher/$name/analytics",
)({
  component: PluginAnalyticsPage,
});

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function PluginAnalyticsPage() {
  const { publisher, name } = Route.useParams();
  const [days, setDays] = useState(30);

  const { data: detail, isLoading } = useRegistryPluginDetail(publisher, name);
  const { data: versions } = useRegistryPluginVersions(publisher, name);
  const { data: analytics, isLoading: analyticsLoading } = usePluginAnalytics(
    publisher,
    name,
    days,
  );

  const plugin = detail?.plugin;

  const downloadsChartConfig: ChartConfig = {
    downloads: {
      label: "Downloads",
      color: "var(--color-blue-500, #3b82f6)",
    },
  };

  const versionChartConfig: ChartConfig = {
    downloads: {
      label: "Downloads",
      color: "var(--color-violet-500, #8b5cf6)",
    },
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!plugin) {
    return (
      <Card className="border-destructive mx-6 mt-8">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>Plugin not found</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const versionData = (versions ?? detail?.versions ?? [])
    .filter((v) => !v.yanked)
    .map((v) => ({
      version: v.version,
      downloads: v.downloads,
    }))
    .reverse();

  return (
    <div className="space-y-6 px-6 @container/main py-3">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/plugins/$publisher/$name" params={{ publisher, name }}>
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">
              Plugin Analytics
            </h1>
            <Badge variant="secondary">
              {publisher}/{name}
            </Badge>
          </div>
          <p className="text-muted-foreground">
            Download trends and version statistics
          </p>
        </div>
        <Select value={String(days)} onValueChange={(v) => setDays(Number(v))}>
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

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">
              Total Downloads
            </CardTitle>
            <Icons.Download className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatNumber(
                analytics?.total_downloads ?? plugin.total_downloads,
              )}
            </div>
            <p className="text-xs text-muted-foreground">All time</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">
              Weekly Downloads
            </CardTitle>
            <Icons.TrendUp className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatNumber(
                analytics?.weekly_downloads ?? plugin.weekly_downloads,
              )}
            </div>
            <p className="text-xs text-muted-foreground">Last 7 days</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Versions</CardTitle>
            <Icons.Package className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {(versions ?? detail?.versions)?.length ?? 0}
            </div>
            <p className="text-xs text-muted-foreground">
              Latest: v{plugin.latest_version}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Download Trend */}
        <Card>
          <CardHeader>
            <CardTitle>Download Trend</CardTitle>
            <CardDescription>
              Daily downloads over the selected period
            </CardDescription>
          </CardHeader>
          <CardContent>
            {analyticsLoading ? (
              <div className="flex items-center justify-center h-75">
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : analytics?.daily_stats && analytics.daily_stats.length > 0 ? (
              <ChartContainer config={downloadsChartConfig} className="h-75">
                <AreaChart data={analytics.daily_stats}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="date"
                    tickFormatter={(v) =>
                      new Date(v).toLocaleDateString("en-US", {
                        month: "short",
                        day: "numeric",
                      })
                    }
                    tickLine={false}
                    axisLine={false}
                  />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Area
                    type="monotone"
                    dataKey="downloads"
                    stroke="var(--color-blue-500, #3b82f6)"
                    fill="var(--color-blue-500, #3b82f6)"
                    fillOpacity={0.2}
                  />
                </AreaChart>
              </ChartContainer>
            ) : (
              <div className="flex items-center justify-center h-75 text-muted-foreground">
                No download data available
              </div>
            )}
          </CardContent>
        </Card>

        {/* Downloads per Version */}
        <Card>
          <CardHeader>
            <CardTitle>Downloads per Version</CardTitle>
            <CardDescription>
              Total downloads breakdown by version
            </CardDescription>
          </CardHeader>
          <CardContent>
            {versionData.length > 0 ? (
              <ChartContainer config={versionChartConfig} className="h-75">
                <BarChart data={versionData}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis dataKey="version" tickLine={false} axisLine={false} />
                  <YAxis tickLine={false} axisLine={false} />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Bar
                    dataKey="downloads"
                    fill="var(--color-violet-500, #8b5cf6)"
                    radius={[4, 4, 0, 0]}
                  />
                </BarChart>
              </ChartContainer>
            ) : (
              <div className="flex items-center justify-center h-75 text-muted-foreground">
                No version data available
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
