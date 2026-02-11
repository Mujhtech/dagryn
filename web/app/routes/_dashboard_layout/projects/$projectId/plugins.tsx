import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Package, Search, ExternalLink } from "lucide-react";
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
import type { PluginInfo } from "~/lib/api";
import { useProjectPlugins } from "~/hooks/queries";
import { Icons } from "~/components/icons";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/plugins",
)({
  component: ProjectPluginsPage,
});

function ProjectPluginsPage() {
  const { projectId } = Route.useParams();
  const [search, setSearch] = useState("");

  // Fetch project plugins
  const { data, isLoading } = useProjectPlugins(projectId);

  const plugins: PluginInfo[] = data?.plugins || [];

  const filteredPlugins = plugins.filter(
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
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Plugins</h1>
          <p className="text-muted-foreground mt-1">
            Manage plugins for your project
          </p>
        </div>
        <Link to="/plugins/browse">
          <Button>
            <Package className="mr-2 h-4 w-4" />
            Browse Official Plugins
          </Button>
        </Link>
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

      {filteredPlugins.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Package className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No plugins installed</h3>
            <p className="text-muted-foreground text-center mb-4">
              Get started by browsing official plugins
            </p>
            <Link to="/plugins/browse">
              <Button>Browse Plugins</Button>
            </Link>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredPlugins.map((plugin: PluginInfo) => (
            <Card
              key={plugin.name}
              className="hover:shadow-lg transition-shadow"
            >
              <CardHeader>
                <div className="flex justify-between items-start">
                  <CardTitle className="text-lg">{plugin.name}</CardTitle>
                  {plugin.installed && (
                    <Badge variant="default">Installed</Badge>
                  )}
                </div>
                <CardDescription className="line-clamp-2">
                  {plugin.description}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2 mb-4">
                  <Badge variant="outline">{plugin.type}</Badge>
                  <Badge variant="outline">{plugin.version}</Badge>
                  {plugin.author && (
                    <Badge variant="outline">{plugin.author}</Badge>
                  )}
                </div>
                <Button variant="ghost" size="sm" className="w-full" asChild>
                  <Link
                    to="/plugins/$pluginName"
                    params={{ pluginName: plugin.name }}
                  >
                    View Details
                    <ExternalLink className="ml-2 h-3 w-3" />
                  </Link>
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
