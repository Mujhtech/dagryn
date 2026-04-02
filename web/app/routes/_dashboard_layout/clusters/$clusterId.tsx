import { createFileRoute, Link } from "@tanstack/react-router";
import { useMemo } from "react";
import { useCluster, useWorkers } from "~/hooks/queries/use-cluster-detail";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Badge } from "~/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Icons } from "~/components/icons";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute("/_dashboard_layout/clusters/$clusterId")({
  component: ClusterDetailPage,
  head: () => generateMetadata({ title: "Cluster Detail" }),
});

function ClusterDetailPage() {
  const { clusterId } = Route.useParams();
  const { data: cluster, isLoading: clusterLoading } = useCluster(clusterId);
  const { data: workers, isLoading: workersLoading } = useWorkers(clusterId);

  const online = useMemo(
    () => (workers ?? []).filter((w) => w.status === "online").length,
    [workers],
  );

  if (clusterLoading || workersLoading) {
    return (
      <div className="flex items-center justify-center h-48">
        <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!cluster) {
    return (
      <div className="p-6 text-muted-foreground">Cluster not found.</div>
    );
  }

  return (
    <div className="space-y-6 px-6 py-3">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">{cluster.name}</h1>
        <p className="text-muted-foreground">Cluster worker inventory and health.</p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Total Workers</CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">
            {workers?.length ?? 0}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Online</CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">{online}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Scope</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">{cluster.scope_type}</Badge>
            {cluster.system_default && <Badge className="ml-2">default</Badge>}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Workers</CardTitle>
          <CardDescription>Workers currently associated with this cluster.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Worker</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Environment</TableHead>
                <TableHead>OS/Arch</TableHead>
                <TableHead>Active Tasks</TableHead>
                <TableHead>Last Heartbeat</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(workers ?? []).map((w) => (
                <TableRow key={w.id}>
                  <TableCell>
                    <Link
                      to="/workers/$workerId"
                      params={{ workerId: w.id }}
                      className="font-medium hover:underline"
                    >
                      {w.hostname}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={w.status === "online" ? "default" : "secondary"}
                    >
                      {w.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{w.environment}</TableCell>
                  <TableCell>
                    {w.os}/{w.arch}
                  </TableCell>
                  <TableCell>{w.active_tasks}</TableCell>
                  <TableCell>{new Date(w.last_heartbeat_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
              {(workers ?? []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="text-muted-foreground">
                    No workers registered in this cluster.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
