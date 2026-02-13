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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
  DialogClose,
} from "~/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "~/components/ui/alert-dialog";
import type { PluginInfo, RegistryPluginSummary } from "~/lib/api";
import { useProjectPlugins, useRegistryPlugins } from "~/hooks/queries";
import { useInstallPlugin } from "~/hooks/mutations/use-install-plugin";
import { useUninstallPlugin } from "~/hooks/mutations/use-uninstall-plugin";
import { Icons } from "~/components/icons";

export const Route = createFileRoute(
  "/_dashboard_layout/projects/$projectId/plugins",
)({
  component: ProjectPluginsPage,
});

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function InstallPluginDialog({ projectId }: { projectId: string }) {
  const [search, setSearch] = useState("");
  const [open, setOpen] = useState(false);
  const installMutation = useInstallPlugin(projectId);

  const { data: searchResults, isLoading } = useRegistryPlugins({
    q: search || undefined,
    per_page: 6,
  });

  const plugins: RegistryPluginSummary[] = searchResults?.plugins || [];

  const handleInstall = async (plugin: RegistryPluginSummary) => {
    try {
      await installMutation.mutateAsync(
        `registry:${plugin.publisher_name}/${plugin.name}@${plugin.latest_version}`,
      );
      setOpen(false);
    } catch {
      // Error handled by mutation state
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Icons.Plus className="mr-2 h-4 w-4" />
          Install Plugin
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Install Plugin</DialogTitle>
          <DialogDescription>
            Search the registry and install a plugin
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="relative">
            <Icons.Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search plugins..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10"
            />
          </div>

          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : plugins.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              No plugins found
            </div>
          ) : (
            <div className="space-y-2 max-h-[300px] overflow-y-auto">
              {plugins.map((plugin) => (
                <div
                  key={plugin.id}
                  className="flex items-center justify-between p-3 rounded-md border hover:bg-muted/50 transition-colors"
                >
                  <div className="flex-1 min-w-0 mr-3">
                    <div className="flex items-center gap-2">
                      <span className="font-medium truncate">
                        {plugin.publisher_name}/{plugin.name}
                      </span>
                      {plugin.publisher_verified && (
                        <Icons.CheckCircle className="h-3 w-3 text-primary shrink-0" />
                      )}
                    </div>
                    <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
                      <Badge variant="outline" className="text-xs">
                        {plugin.type}
                      </Badge>
                      <span>v{plugin.latest_version}</span>
                      <span className="flex items-center gap-1">
                        <Icons.Download className="h-3 w-3" />
                        {formatNumber(plugin.total_downloads)}
                      </span>
                    </div>
                  </div>
                  <Button
                    size="sm"
                    disabled={installMutation.isPending}
                    onClick={() => handleInstall(plugin)}
                  >
                    {installMutation.isPending ? (
                      <Icons.Loader className="h-3 w-3 animate-spin" />
                    ) : (
                      "Install"
                    )}
                  </Button>
                </div>
              ))}
            </div>
          )}

          {installMutation.error && (
            <p className="text-sm text-destructive">
              {installMutation.error instanceof Error
                ? installMutation.error.message
                : "Failed to install plugin"}
            </p>
          )}
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">Close</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ProjectPluginsPage() {
  const { projectId } = Route.useParams();
  const [search, setSearch] = useState("");
  const uninstallMutation = useUninstallPlugin(projectId);

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
    <div className="container mx-auto py-8 space-y-6 px-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Plugins</h1>
          <p className="text-muted-foreground mt-1">
            Manage plugins for your project
          </p>
        </div>
        <div className="flex items-center gap-2">
          <InstallPluginDialog projectId={projectId} />
          <Link to="/plugins/browse">
            <Button variant="outline">
              <Icons.Package className="mr-2 h-4 w-4" />
              Browse Registry
            </Button>
          </Link>
        </div>
      </div>

      <div className="flex items-center space-x-2">
        <div className="relative flex-1 max-w-md">
          <Icons.Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
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
            <Icons.Package className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No plugins installed</h3>
            <p className="text-muted-foreground text-center mb-4">
              Get started by installing plugins from the registry
            </p>
            <InstallPluginDialog projectId={projectId} />
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
                <div className="flex gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="flex-1"
                    asChild
                  >
                    <Link
                      to="/plugins/$pluginName"
                      params={{ pluginName: plugin.name }}
                    >
                      View Details
                    </Link>
                  </Button>
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button variant="outline" size="sm">
                        <Icons.Trash className="h-4 w-4" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>Uninstall Plugin</AlertDialogTitle>
                        <AlertDialogDescription>
                          Are you sure you want to uninstall{" "}
                          <strong>{plugin.name}</strong>? This will remove it
                          from your project configuration.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                          onClick={() =>
                            uninstallMutation.mutate(plugin.name)
                          }
                        >
                          Uninstall
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
