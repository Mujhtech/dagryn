import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import type { RegistryPluginSummary } from "~/lib/api";
import {
  useRegistryPlugins,
  useFeaturedPlugins,
  useTrendingPlugins,
} from "~/hooks/queries";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/plugins/browse")({
  component: BrowsePluginsPage,
});

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function PluginCard({ plugin }: { plugin: RegistryPluginSummary }) {
  return (
    <Card className="hover:shadow-lg transition-shadow cursor-pointer">
      <CardHeader>
        <CardTitle className="text-lg flex items-center justify-between">
          <span className="truncate">{plugin.name}</span>
          <div className="flex items-center gap-1">
            {plugin.publisher_verified && (
              <Icons.CheckCircle className="h-4 w-4 text-primary" />
            )}
          </div>
        </CardTitle>
        <CardDescription className="line-clamp-2">
          {plugin.description}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex flex-wrap gap-2 mb-3">
          <Badge variant="outline">{plugin.type}</Badge>
          <Badge variant="outline">v{plugin.latest_version}</Badge>
          {plugin.featured && <Badge variant="secondary">Featured</Badge>}
          {plugin.deprecated && <Badge variant="destructive">Deprecated</Badge>}
        </div>

        <div className="flex items-center gap-3 mb-4 text-sm text-muted-foreground">
          <span className="flex items-center gap-1">
            <Icons.Download className="h-3 w-3" />
            {formatNumber(plugin.total_downloads)}
          </span>
          <span className="flex items-center gap-1">
            <Icons.User className="h-3 w-3" />
            {plugin.publisher_name}
          </span>
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
  );
}

function BrowsePluginsPage() {
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [sort, setSort] = useState("downloads");
  const [page, setPage] = useState(1);

  const { data: searchResults, isLoading } = useRegistryPlugins({
    q: search || undefined,
    type: typeFilter === "all" ? undefined : typeFilter,
    sort,
    page,
    per_page: 12,
  });

  const { data: featured } = useFeaturedPlugins();
  const { data: trending } = useTrendingPlugins();

  const plugins: RegistryPluginSummary[] = searchResults?.plugins || [];
  const totalPages = searchResults
    ? Math.ceil(searchResults.total / searchResults.per_page)
    : 1;

  const showFeaturedSection = !search && typeFilter === "all" && page === 1;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="container mx-auto py-8 space-y-6 px-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Link to="/dashboard">
            <Button variant="ghost" size="icon">
              <Icons.ArrowLeft className="h-4 w-4" />
            </Button>
          </Link>
          <div>
            <h1 className="text-3xl font-bold tracking-tight">
              Plugin Registry
            </h1>
            <p className="text-muted-foreground mt-1">
              Browse, search, and install plugins for your projects
            </p>
          </div>
        </div>
        <Link to="/plugins/publish">
          <Button>
            <Icons.Upload className="h-4 w-4 mr-2" />
            Publish
          </Button>
        </Link>
      </div>

      {/* Search & Filters */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 max-w-md">
          <Icons.Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search plugins..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            className="pl-10"
          />
        </div>
        <Select
          value={typeFilter}
          onValueChange={(v) => {
            setTypeFilter(v);
            setPage(1);
          }}
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue placeholder="Type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Types</SelectItem>
            <SelectItem value="composite">Composite</SelectItem>
            <SelectItem value="tool">Tool</SelectItem>
            <SelectItem value="integration">Integration</SelectItem>
          </SelectContent>
        </Select>
        <Select value={sort} onValueChange={setSort}>
          <SelectTrigger className="w-[160px]">
            <SelectValue placeholder="Sort by" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="downloads">Most Downloads</SelectItem>
            <SelectItem value="recent">Recently Updated</SelectItem>
            <SelectItem value="name">Name</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Featured & Trending */}
      {showFeaturedSection && (
        <>
          {featured && featured.length > 0 && (
            <div>
              <h2 className="text-xl font-semibold mb-3">Featured</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {featured.slice(0, 3).map((plugin: RegistryPluginSummary) => (
                  <PluginCard key={plugin.id} plugin={plugin} />
                ))}
              </div>
            </div>
          )}

          {trending && trending.length > 0 && (
            <div>
              <h2 className="text-xl font-semibold mb-3">Trending</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {trending.slice(0, 3).map((plugin: RegistryPluginSummary) => (
                  <PluginCard key={plugin.id} plugin={plugin} />
                ))}
              </div>
            </div>
          )}
        </>
      )}

      {/* All Plugins */}
      <div>
        {showFeaturedSection && (
          <h2 className="text-xl font-semibold mb-3">All Plugins</h2>
        )}

        {plugins.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Icons.Package className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-semibold mb-2">No plugins found</h3>
              <p className="text-muted-foreground text-center">
                Try adjusting your search or filters
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {plugins.map((plugin: RegistryPluginSummary) => (
              <PluginCard key={plugin.id} plugin={plugin} />
            ))}
          </div>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            <Icons.ArrowLeft className="h-4 w-4 mr-1" />
            Previous
          </Button>
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
            <Icons.ArrowRight className="h-4 w-4 ml-1" />
          </Button>
        </div>
      )}
    </div>
  );
}
