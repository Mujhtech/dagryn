import { createFileRoute, Link, useSearch } from "@tanstack/react-router";
import { useMemo } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  useClusters,
  useCapabilities,
  useTeams,
  useWorkersForAccessibleScopes,
  useWorkerTokens,
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
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { useState } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";

const createWorkerTokenSchema = z.object({
  name: z.string().trim().min(1, "Token name is required"),
  expiry: z.enum(["7d", "30d", "90d", "1y", "no"]),
});

type CreateWorkerTokenForm = z.infer<typeof createWorkerTokenSchema>;

export const Route = createFileRoute("/_dashboard_layout/clusters/")({
  component: ClustersPage,
  head: () => generateMetadata({ title: "Clusters" }),
  validateSearch: (search: Record<string, unknown>) => ({
    teamId: typeof search.teamId === "string" ? search.teamId : undefined,
  }),
});

export function ClustersPage() {
  const search = useSearch({ from: "/_dashboard_layout/clusters/" });
  const teamId = typeof search.teamId === "string" ? search.teamId : undefined;
  const queryClient = useQueryClient();
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const { data: capabilities } = useCapabilities();

  const form = useForm<CreateWorkerTokenForm>({
    resolver: zodResolver(createWorkerTokenSchema),
    defaultValues: {
      name: "my-agent",
      expiry: "90d",
    },
  });

  const resetCreateTokenForm = () => {
    form.reset({ name: "my-agent", expiry: "90d" });
  };
  const { data: teamsResp } = useTeams();
  const personalClusters = useClusters(teamId);
  const teamIds = (teamsResp?.data ?? []).map((t) => t.id);
  const { data: workers } = useWorkersForAccessibleScopes(teamIds);
  const { data: workerTokens, isLoading: tokensLoading } = useWorkerTokens();

  const grpcServer = useMemo(() => {
    if (capabilities?.grpc_public_address) {
      return capabilities.grpc_public_address;
    }
    if (typeof window === "undefined") {
      return "localhost:9001";
    }
    return `${window.location.hostname}:443`;
  }, [capabilities?.grpc_public_address]);

  const createToken = useMutation({
    mutationFn: async (values: CreateWorkerTokenForm) => {
      const payload: {
        name: string;
        expires_in?: string;
        team_id?: string;
      } = { name: values.name.trim() || "my-agent" };
      if (values.expiry !== "no") payload.expires_in = values.expiry;
      if (teamId) payload.team_id = teamId;
      const { data } = await api.createWorkerToken(payload);
      return data;
    },
    onSuccess: async (data) => {
      setCreatedToken(data.key);
      setCreateDialogOpen(false);
      resetCreateTokenForm();
      await queryClient.invalidateQueries({ queryKey: queryKeys.workerTokens });
    },
  });

  const revokeToken = useMutation({
    mutationFn: async (id: string) => {
      await api.revokeWorkerToken(id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.workerTokens });
    },
  });

  const clusters = useMemo(() => {
    const all = [...(personalClusters.data ?? [])];
    // teamClusterQueries.forEach((q) => {
    //   if (q.data) all.push(...q.data);
    // });
    return all;
  }, [personalClusters.data]);

  const workerCounts = useMemo(() => {
    const map = new Map<string, number>();
    (workers ?? []).forEach((w) => {
      if (!w.cluster_id) return;
      map.set(w.cluster_id, (map.get(w.cluster_id) ?? 0) + 1);
    });
    return map;
  }, [workers]);

  const loading = personalClusters.isLoading;

  return (
    <div className="space-y-6 px-6 py-3">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Clusters</h1>
        <p className="text-muted-foreground">
          Monitor execution clusters, worker pools, and scope boundaries.
        </p>
      </div>

      <Tabs defaultValue="clusters" className="space-y-4">
        <TabsList>
          <TabsTrigger value="clusters">Clusters</TabsTrigger>
          <TabsTrigger value="tokens">Worker Tokens</TabsTrigger>
        </TabsList>

        <TabsContent value="clusters">
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
                        <TableCell
                          colSpan={5}
                          className="text-muted-foreground"
                        >
                          No clusters available.
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="tokens">
          <Card>
            <CardHeader className="flex flex-row items-start justify-between">
              <div>
                <CardTitle>Worker Tokens</CardTitle>
                <CardDescription>
                  Create scoped tokens for agents. Token value is shown once.
                </CardDescription>
              </div>
              <Button onClick={() => setCreateDialogOpen(true)}>
                Create Token
              </Button>
            </CardHeader>
            <CardContent className="space-y-4">
              {tokensLoading ? (
                <div className="text-sm text-muted-foreground">
                  Loading tokens...
                </div>
              ) : (workerTokens?.length ?? 0) === 0 ? (
                <div className="text-sm text-muted-foreground">
                  No worker tokens yet.
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Prefix</TableHead>
                      <TableHead>Scope</TableHead>
                      <TableHead>Last Used</TableHead>
                      <TableHead>Expires</TableHead>
                      <TableHead>Action</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(workerTokens ?? []).map((token) => (
                      <TableRow key={token.id}>
                        <TableCell>{token.name}</TableCell>
                        <TableCell className="font-mono text-xs">
                          {token.key_prefix}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline">{token.scope_type}</Badge>
                        </TableCell>
                        <TableCell>
                          {token.last_used_at
                            ? new Date(token.last_used_at).toLocaleString()
                            : "never"}
                        </TableCell>
                        <TableCell>
                          {token.expires_at
                            ? new Date(token.expires_at).toLocaleString()
                            : "never"}
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => revokeToken.mutate(token.id)}
                            disabled={revokeToken.isPending}
                          >
                            Revoke
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <Dialog
        open={createDialogOpen}
        onOpenChange={(open) => {
          setCreateDialogOpen(open);
          if (!open) {
            createToken.reset();
            resetCreateTokenForm();
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Worker Token</DialogTitle>
            <DialogDescription>
              This token can be used by `dagryn agent start` and will be shown
              once.
            </DialogDescription>
          </DialogHeader>
          <Form {...form}>
            <form
              className="space-y-4"
              onSubmit={form.handleSubmit((values) =>
                createToken.mutate(values),
              )}
            >
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Token Name</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="my-agent"
                        {...field}
                        disabled={createToken.isPending}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="expiry"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Expiration</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={createToken.isPending}
                    >
                      <FormControl>
                        <SelectTrigger className="w-full">
                          <SelectValue placeholder="Select expiry" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="7d">7 days</SelectItem>
                        <SelectItem value="30d">30 days</SelectItem>
                        <SelectItem value="90d">90 days</SelectItem>
                        <SelectItem value="1y">1 year</SelectItem>
                        <SelectItem value="no">No expiration</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {createToken.error ? (
                <p className="text-sm text-destructive">
                  Failed to create worker token.
                </p>
              ) : null}

              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline" type="button">
                    Cancel
                  </Button>
                </DialogClose>
                <Button type="submit" disabled={createToken.isPending}>
                  {createToken.isPending ? "Creating..." : "Create Token"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!createdToken}
        onOpenChange={(open) => !open && setCreatedToken(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Worker Token Created</DialogTitle>
            <DialogDescription>
              Copy this token now. You will not be able to see it again.
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-md border bg-muted p-3 font-mono text-xs break-all">
            {createdToken}
          </div>
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">
              Start an agent with this token:
            </p>
            <div className="rounded-md border bg-muted p-3 font-mono text-xs break-all">
              {`dagryn agent start --server ${grpcServer} --token ${createdToken ?? "<token>"}`}
            </div>
            <p className="text-xs text-muted-foreground">
              For team workers, create token while filtering to that team. For personal workers,
              create token without team filter.
            </p>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={async () => {
                if (createdToken) {
                  await navigator.clipboard.writeText(createdToken);
                }
              }}
            >
              Copy
            </Button>
            <Button
              variant="outline"
              onClick={async () => {
                if (createdToken) {
                  await navigator.clipboard.writeText(
                    `dagryn agent start --server ${grpcServer} --token ${createdToken}`,
                  );
                }
              }}
            >
              Copy Command
            </Button>
            <Button onClick={() => setCreatedToken(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
