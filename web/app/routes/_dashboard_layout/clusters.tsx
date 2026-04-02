import { createFileRoute, Link, useSearch } from "@tanstack/react-router";
import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import {
  useClusters,
  useTeams,
  useWorkersForAccessibleScopes,
} from "~/hooks/queries";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";
import { generateMetadata } from "~/lib/metadata";
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

export const Route = createFileRoute("/_dashboard_layout/clusters")({
  component: ClustersPage,
  head: () => generateMetadata({ title: "Clusters" }),
  validateSearch: (search: Record<string, unknown>) => ({
    teamId: typeof search.teamId === "string" ? search.teamId : undefined,
  }),
});

export function ClustersPage() {
  const search = useSearch({ from: "/_dashboard_layout/clusters" });
  const teamId = typeof search.teamId === "string" ? search.teamId : undefined;
  const { data: teamsResp } = useTeams();
  const personalClusters = useClusters(teamId);
  const teamIds = (teamsResp?.data ?? []).map((t) => t.id);
  const teamClusterQueries = useQueries({
    queries: teamId
      ? []
      : teamIds.map((id) => ({
          queryKey: queryKeys.clusters(id),
          queryFn: async () => {
            const { data } = await api.listClusters(id);
            return data;
          },
        })),
  });
  const { data: workers } = useWorkersForAccessibleScopes(teamIds);

  const clusters = useMemo(() => {
    const all = [...(personalClusters.data ?? [])];
    teamClusterQueries.forEach((q) => {
      if (q.data) all.push(...q.data);
    });
    return all;
  }, [personalClusters.data, teamClusterQueries]);

  const workerCounts = useMemo(() => {
    const map = new Map<string, number>();
    (workers ?? []).forEach((w) => {
      if (!w.cluster_id) return;
      map.set(w.cluster_id, (map.get(w.cluster_id) ?? 0) + 1);
    });
    return map;
  }, [workers]);

  const loading =
    personalClusters.isLoading || teamClusterQueries.some((q) => q.isLoading);

  return (
    <div className="space-y-6 px-6 py-3">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Clusters</h1>
        <p className="text-muted-foreground">
          Monitor execution clusters, worker pools, and scope boundaries.
        </p>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-48">
          <Icons.Loader className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>All Clusters</CardTitle>
            <CardDescription>
              Team and personal clusters available for task routing.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Scope</TableHead>
                  <TableHead>Workers</TableHead>
                  <TableHead>Default</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {clusters.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell>
                      <Link
                        to="/clusters/$clusterId"
                        params={{ clusterId: c.id }}
                        search={{ teamId }}
                        className="font-medium hover:underline"
                      >
                        {c.name}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{c.scope_type}</Badge>
                    </TableCell>
                    <TableCell>{workerCounts.get(c.id) ?? 0}</TableCell>
                    <TableCell>
                      {c.system_default ? (
                        <Badge>default</Badge>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {new Date(c.updated_at).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))}
                {clusters.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-muted-foreground">
                      No clusters available.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
