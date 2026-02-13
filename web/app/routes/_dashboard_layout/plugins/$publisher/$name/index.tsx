import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Separator } from "~/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "~/components/ui/chart";
import { Area, AreaChart, XAxis, YAxis, CartesianGrid } from "recharts";
import {
  useRegistryPluginDetail,
  useRegistryPluginVersions,
} from "~/hooks/queries";
import { usePluginAnalytics } from "~/hooks/queries";
import { Icons } from "~/components/icons";

import type { PluginVersionInfo } from "~/lib/api";
import { MarkdownRenderer } from "~/components/markdown-renderer";

export const Route = createFileRoute(
  "/_dashboard_layout/plugins/$publisher/$name/",
)({
  component: RegistryPluginDetailPage,
});

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function RegistryPluginDetailPage() {
  const { publisher, name } = Route.useParams();
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);

  const {
    data: detail,
    isLoading,
    error,
  } = useRegistryPluginDetail(publisher, name);
  const { data: versions } = useRegistryPluginVersions(publisher, name);
  const { data: analytics } = usePluginAnalytics(publisher, name, 30);

  const plugin = detail?.plugin;

  const installSnippet = plugin
    ? `[plugins]\n${plugin.name} = "registry:${publisher}/${plugin.name}@${plugin.latest_version}"`
    : "";

  const handleCopy = () => {
    navigator.clipboard.writeText(installSnippet);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const downloadsChartConfig: ChartConfig = {
    downloads: {
      label: "Downloads",
      color: "var(--color-blue-500, #3b82f6)",
    },
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !plugin) {
    return (
      <Card className="border-destructive mx-6 mt-8">
        <CardHeader>
          <CardTitle className="text-destructive">Error</CardTitle>
          <CardDescription>Plugin not found</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="container mx-auto py-8 space-y-6 max-w-4xl px-6">
      {/* Header */}
      <div className="flex items-center space-x-4">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => navigate({ to: "/plugins/browse" })}
        >
          <Icons.ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <div className="flex items-center space-x-3">
            <Icons.Package className="h-8 w-8 text-primary" />
            <div>
              <h1 className="text-3xl font-bold tracking-tight">
                {publisher}/{name}
              </h1>
              <p className="text-muted-foreground mt-1">{plugin.description}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Badges */}
      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">{plugin.type}</Badge>
        <Badge variant="outline">v{plugin.latest_version}</Badge>
        {plugin.publisher_verified && (
          <Badge variant="default">
            <Icons.CheckCircle className="h-3 w-3 mr-1" />
            Verified
          </Badge>
        )}
        {plugin.featured && <Badge variant="secondary">Featured</Badge>}
        {plugin.deprecated && <Badge variant="destructive">Deprecated</Badge>}
        {plugin.license && <Badge variant="outline">{plugin.license}</Badge>}
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">
              {formatNumber(plugin.total_downloads)}
            </div>
            <p className="text-xs text-muted-foreground">Total Downloads</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">
              {formatNumber(plugin.weekly_downloads)}
            </div>
            <p className="text-xs text-muted-foreground">Weekly Downloads</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">
              {formatNumber(plugin.stars)}
            </div>
            <p className="text-xs text-muted-foreground">Stars</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">
              {(versions ?? detail?.versions)?.length ?? 0}
            </div>
            <p className="text-xs text-muted-foreground">Versions</p>
          </CardContent>
        </Card>
      </div>

      <Separator />

      {/* Publisher */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Publisher</CardTitle>
        </CardHeader>
        <CardContent>
          <Link
            to="/plugins/publishers/$publisher"
            params={{ publisher: plugin.publisher_name }}
          >
            <div className="flex items-center gap-3 hover:bg-muted/50 p-2 rounded-md transition-colors">
              <Icons.User className="h-8 w-8 text-muted-foreground" />
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-medium">{plugin.publisher_name}</span>
                  {plugin.publisher_verified && (
                    <Icons.CheckCircle className="h-4 w-4 text-primary" />
                  )}
                </div>
              </div>
            </div>
          </Link>
        </CardContent>
      </Card>

      {/* README Section */}
      {plugin.readme && (
        <Card>
          <CardHeader>
            <CardTitle>README</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="prose prose-sm dark:prose-invert max-w-none">
              <MarkdownRenderer content={plugin.readme} />
            </div>
          </CardContent>
        </Card>
      )}

      {/* Download Trend */}
      {analytics &&
        analytics.daily_stats &&
        analytics.daily_stats.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Download Trend</CardTitle>
              <CardDescription>Downloads over the last 30 days</CardDescription>
            </CardHeader>
            <CardContent>
              <ChartContainer config={downloadsChartConfig} className="h-62.5">
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
            </CardContent>
          </Card>
        )}

      {/* Versions */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Versions</CardTitle>
              <CardDescription>Release history</CardDescription>
            </div>
            <Link
              to="/plugins/$publisher/$name/analytics"
              params={{ publisher, name }}
            >
              <Button variant="outline" size="sm">
                <Icons.TrendUp className="h-4 w-4 mr-1" />
                Full Analytics
              </Button>
            </Link>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Version</TableHead>
                <TableHead>Downloads</TableHead>
                <TableHead>Published</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(versions ?? detail?.versions ?? []).map(
                (v: PluginVersionInfo) => (
                  <TableRow key={v.id}>
                    <TableCell className="font-mono">{v.version}</TableCell>
                    <TableCell>{formatNumber(v.downloads)}</TableCell>
                    <TableCell>
                      {new Date(v.published_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      {v.yanked ? (
                        <Badge variant="destructive">Yanked</Badge>
                      ) : (
                        <Badge variant="outline">Active</Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ),
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Links */}
      {(plugin.homepage || plugin.repository_url) && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Links</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            {plugin.homepage && (
              <Button variant="outline" size="sm" asChild>
                <a
                  href={plugin.homepage}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Icons.Link className="h-4 w-4 mr-1" />
                  Homepage
                </a>
              </Button>
            )}
            {plugin.repository_url && (
              <Button variant="outline" size="sm" asChild>
                <a
                  href={plugin.repository_url}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Icons.Github className="h-4 w-4 mr-1" />
                  Repository
                </a>
              </Button>
            )}
          </CardContent>
        </Card>
      )}

      {/* Installation */}
      <Card>
        <CardHeader>
          <CardTitle>Installation</CardTitle>
          <CardDescription>Add this plugin to your dagryn.toml</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative">
            <pre className="bg-muted p-4 rounded-none overflow-x-auto text-sm">
              <code>{installSnippet}</code>
            </pre>
            <Button
              variant="ghost"
              size="sm"
              className="absolute top-2 right-2"
              onClick={handleCopy}
            >
              {copied ? (
                <>
                  <Icons.Check className="h-4 w-4 mr-2" />
                  Copied
                </>
              ) : (
                <>
                  <Icons.Copy className="h-4 w-4 mr-2" />
                  Copy
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
