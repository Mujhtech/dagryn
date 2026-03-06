import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Icons } from "~/components/icons";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";
import type { RegistryPluginSummary } from "~/lib/api";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/plugins/publishers/$publisher",
)({
  component: PublisherProfilePage,
  head: () => {
    return generateMetadata({ title: "Publisher" });
  },
});

const PER_PAGE = 20;

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function PublisherProfilePage() {
  const { publisher } = Route.useParams();
  const [page, setPage] = useState(1);

  const { data: publisherData, isLoading: publisherLoading } = useQuery({
    queryKey: [...queryKeys.publisher(publisher), page],
    queryFn: async () => {
      const { data } = await api.searchRegistryPlugins({
        publisher,
        page,
        per_page: PER_PAGE,
      });
      return data;
    },
    enabled: !!publisher,
  });

  const plugins: RegistryPluginSummary[] = publisherData?.plugins ?? [];
  const total = publisherData?.total ?? 0;
  const totalPages = Math.ceil(total / PER_PAGE);

  const totalDownloads = plugins.reduce(
    (sum: number, p: RegistryPluginSummary) => sum + p.total_downloads,
    0,
  );

  if (publisherLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const isVerified = plugins.some(
    (p: RegistryPluginSummary) => p.publisher_verified,
  );

  return (
    <div className="container mx-auto py-8 space-y-6 max-w-4xl px-6">
      {/* Header */}
      <div className="flex items-center space-x-4">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/plugins/browse">
            <Icons.ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex items-center gap-4 flex-1">
          <Icons.User className="h-12 w-12 text-muted-foreground" />
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-3xl font-bold tracking-tight">{publisher}</h1>
              {isVerified && (
                <Badge variant="default">
                  <Icons.CheckCircle className="h-3 w-3 mr-1" />
                  Verified
                </Badge>
              )}
            </div>
            <p className="text-muted-foreground mt-1">Plugin publisher</p>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">{plugins.length}</div>
            <p className="text-xs text-muted-foreground">
              {plugins.length === 1 ? "Plugin" : "Plugins"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">
              {formatNumber(totalDownloads)}
            </div>
            <p className="text-xs text-muted-foreground">Total Downloads</p>
          </CardContent>
        </Card>
      </div>

      {/* Plugins */}
      <div>
        <h2 className="text-xl font-semibold mb-4">Plugins</h2>
        {plugins.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Icons.Package className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold mb-2">No plugins found</h3>
              <p className="text-muted-foreground text-center">
                This publisher hasn't published any plugins yet
              </p>
            </CardContent>
          </Card>
        ) : (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {plugins.map((plugin: RegistryPluginSummary) => (
                <Card
                  key={plugin.id}
                  className="hover:shadow-lg transition-shadow"
                >
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center justify-between">
                      <span>{plugin.name}</span>
                      <Badge variant="outline">{plugin.type}</Badge>
                    </CardTitle>
                    <CardDescription className="line-clamp-2">
                      {plugin.description}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex items-center gap-4 mb-4 text-sm text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Icons.Download className="h-3 w-3" />
                        {formatNumber(plugin.total_downloads)}
                      </span>
                      <span>v{plugin.latest_version}</span>
                      {plugin.featured && (
                        <Badge variant="secondary" className="text-xs">
                          Featured
                        </Badge>
                      )}
                    </div>
                    <Link
                      to="/plugins/$publisher/$name"
                      params={{
                        publisher: plugin.publisher_name,
                        name: plugin.name,
                      }}
                    >
                      <Button variant="default" size="sm" className="w-full">
                        View Details
                      </Button>
                    </Link>
                  </CardContent>
                </Card>
              ))}
            </div>
            {totalPages > 1 ? (
              <div className="flex items-center justify-between mt-6">
                <p className="text-sm text-muted-foreground">
                  Showing {(page - 1) * PER_PAGE + 1} -{" "}
                  {Math.min(page * PER_PAGE, total)} of {total} plugins
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(Math.max(1, page - 1))}
                    disabled={page === 1 || publisherLoading}
                  >
                    <Icons.ChevronLeft className="h-4 w-4 mr-1" />
                    Previous
                  </Button>
                  <div className="flex items-center gap-1 px-2">
                    <span className="text-sm font-medium">{page}</span>
                    <span className="text-sm text-muted-foreground">of</span>
                    <span className="text-sm font-medium">{totalPages}</span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(Math.min(totalPages, page + 1))}
                    disabled={page === totalPages || publisherLoading}
                  >
                    Next
                    <Icons.ChevronRight className="h-4 w-4 ml-1" />
                  </Button>
                </div>
              </div>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}
