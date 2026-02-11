import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Package, Search, ArrowLeft } from "lucide-react";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import type { PluginInfo } from "~/lib/api";
import { useOfficialPlugins } from "~/hooks/queries";
import { Icons } from "~/components/icons";

export const Route = createFileRoute("/_dashboard_layout/plugins/browse")({
  component: BrowsePluginsPage,
});

function BrowsePluginsPage() {
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");

  // Fetch official plugins
  const { data, isLoading } = useOfficialPlugins();

  const plugins: PluginInfo[] = data?.plugins || [];

  // Categorize plugins
  const categories = {
    all: plugins,
    setup: plugins.filter((p: PluginInfo) => p.name.startsWith("setup-")),
    linters: plugins.filter((p: PluginInfo) =>
      ["eslint", "prettier", "golangci-lint"].includes(p.name),
    ),
    testing: plugins.filter((p: PluginInfo) =>
      ["pytest", "jest"].includes(p.name),
    ),
    utilities: plugins.filter((p: PluginInfo) =>
      ["docker-build", "slack-notify", "cache-s3"].includes(p.name),
    ),
  };

  const filteredPlugins = categories[
    category as keyof typeof categories
  ].filter(
    (p: PluginInfo) =>
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      p.description?.toLowerCase().includes(search.toLowerCase()),
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="container mx-auto py-8 space-y-6">
      <div className="flex items-center space-x-4">
        <Link to="/">
          <Button variant="ghost" size="icon">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <div>
          <h1 className="text-3xl font-bold tracking-tight">
            Official Dagryn Plugins
          </h1>
          <p className="text-muted-foreground mt-1">
            Browse and install official plugins for your projects
          </p>
        </div>
      </div>

      <div className="flex items-center space-x-2">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search plugins..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-10"
          />
        </div>
      </div>

      <Tabs value={category} onValueChange={setCategory}>
        <TabsList>
          <TabsTrigger value="all">All ({categories.all.length})</TabsTrigger>
          <TabsTrigger value="setup">
            Setup ({categories.setup.length})
          </TabsTrigger>
          <TabsTrigger value="linters">
            Linters ({categories.linters.length})
          </TabsTrigger>
          <TabsTrigger value="testing">
            Testing ({categories.testing.length})
          </TabsTrigger>
          <TabsTrigger value="utilities">
            Utilities ({categories.utilities.length})
          </TabsTrigger>
        </TabsList>

        <TabsContent value={category} className="mt-6">
          {filteredPlugins.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-12">
                <Package className="h-12 w-12 text-muted-foreground mb-4" />
                <h3 className="text-lg font-semibold mb-2">No plugins found</h3>
                <p className="text-muted-foreground text-center">
                  Try adjusting your search or category filter
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {filteredPlugins.map((plugin: PluginInfo) => (
                <Card
                  key={plugin.name}
                  className="hover:shadow-lg transition-shadow cursor-pointer"
                >
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center justify-between">
                      <span>{plugin.name}</span>
                      <Package className="h-5 w-5 text-muted-foreground" />
                    </CardTitle>
                    <CardDescription className="line-clamp-2">
                      {plugin.description}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-wrap gap-2 mb-4">
                      <Badge variant="outline">{plugin.type}</Badge>
                      <Badge variant="outline">v{plugin.version}</Badge>
                      {plugin.author && (
                        <Badge variant="secondary">{plugin.author}</Badge>
                      )}
                    </div>

                    {plugin.inputs && Object.keys(plugin.inputs).length > 0 && (
                      <div className="text-sm text-muted-foreground mb-2">
                        {Object.keys(plugin.inputs).length} input
                        {Object.keys(plugin.inputs).length !== 1 ? "s" : ""}
                      </div>
                    )}

                    <Link
                      to="/plugins/$pluginName"
                      params={{ pluginName: plugin.name }}
                    >
                      <Button variant="default" size="sm" className="w-full">
                        View Details
                      </Button>
                    </Link>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
