import { useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { ProjectEnvVar, SetProjectEnvVarInput } from "~/lib/api";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { Checkbox } from "~/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Separator } from "~/components/ui/separator";
import { Badge } from "~/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Icons } from "~/components/icons";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";

const envSchema = z.object({
  key: z.string().min(1, "Key is required"),
  value: z.string().min(1, "Value is required"),
  environment: z.string().optional(),
  branch: z.string().optional(),
  required: z.boolean(),
  secret: z.boolean(),
  description: z.string().optional(),
});

type EnvFormValues = z.infer<typeof envSchema>;

type EnvVarsCardProps = {
  envVars?: ProjectEnvVar[];
  loading: boolean;
  error?: string;
  createPending: boolean;
  deletePending: boolean;
  rotatePending: boolean;
  seedPending: boolean;
  onCreate: (payload: SetProjectEnvVarInput) => void;
  onDelete: (id: string) => void;
  onRotate: (payload: { envVarId: string; value?: string }) => void;
  onUpdate: (payload: { envVarId: string; description?: string; required?: boolean; enabled?: boolean }) => void;
  onSeed: (content: string, options: { environment?: string; branch?: string; secret: boolean; required: boolean }) => void;
};

const ENV_OPTIONS = ["dev", "staging", "prod"] as const;

export function EnvVarsCard({
  envVars,
  loading,
  error,
  createPending,
  deletePending,
  rotatePending,
  seedPending,
  onCreate,
  onDelete,
  onRotate,
  onUpdate,
  onSeed,
}: EnvVarsCardProps) {
  const [seedContent, setSeedContent] = useState("");
  const [seedEnv, setSeedEnv] = useState("dev");
  const [seedBranch, setSeedBranch] = useState("");
  const [seedSecret, setSeedSecret] = useState(true);
  const [seedRequired, setSeedRequired] = useState(false);
  const [rotateTarget, setRotateTarget] = useState<ProjectEnvVar | null>(null);
  const [rotateValue, setRotateValue] = useState("");

  const form = useForm<EnvFormValues>({
    resolver: zodResolver(envSchema),
    defaultValues: {
      key: "",
      value: "",
      environment: "dev",
      branch: "",
      required: false,
      secret: true,
      description: "",
    },
  });

  const grouped = useMemo(() => {
    const list = envVars ?? [];
    return [...list].sort((a, b) => a.key.localeCompare(b.key));
  }, [envVars]);

  const onSubmit = (values: EnvFormValues) => {
    onCreate({
      key: values.key.trim(),
      value: values.value,
      environment: values.environment?.trim() || undefined,
      branch: values.branch?.trim() || undefined,
      required: values.required,
      secret: values.secret,
      description: values.description?.trim() || undefined,
    });

    form.setValue("key", "");
    form.setValue("value", "");
    form.setValue("description", "");
  };

  const handleSeed = () => {
    if (!seedContent.trim()) return;
    onSeed(seedContent, {
      environment: seedEnv.trim() || undefined,
      branch: seedBranch.trim() || undefined,
      secret: seedSecret,
      required: seedRequired,
    });
  };

  const openRotateDialog = (item: ProjectEnvVar) => {
    setRotateTarget(item);
    setRotateValue("");
  };

  const closeRotateDialog = () => {
    setRotateTarget(null);
    setRotateValue("");
  };

  const submitRotate = () => {
    if (!rotateTarget) return;
    const value = rotateValue.trim();
    if (!value) return;
    onRotate({ envVarId: rotateTarget.id, value });
    closeRotateDialog();
  };

  return (
    <Card className="py-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Icons.Lock className="h-5 w-5" />
          Environment Variables
        </CardTitle>
        <CardDescription>
          Manage project environment variables and secrets.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        {error ? (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
        ) : null}

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <FormField
                control={form.control}
                name="key"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Key</FormLabel>
                    <FormControl>
                      <Input placeholder="DATABASE_URL" disabled={createPending} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="value"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Value</FormLabel>
                    <FormControl>
                      <Input type="password" placeholder="••••••••" disabled={createPending} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="environment"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Environment</FormLabel>
                    <Select value={field.value || "dev"} onValueChange={field.onChange} disabled={createPending}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="Select environment" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {ENV_OPTIONS.map((env) => (
                          <SelectItem key={env} value={env}>
                            {env}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="branch"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Branch (optional)</FormLabel>
                    <FormControl>
                      <Input placeholder="feature/my-branch" disabled={createPending} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem className="md:col-span-2">
                    <FormLabel>Description</FormLabel>
                    <FormControl>
                      <Input placeholder="Optional context" disabled={createPending} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className="flex flex-wrap items-center gap-4">
              <FormField
                control={form.control}
                name="secret"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center space-x-2 space-y-0">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(checked) => field.onChange(Boolean(checked))} />
                    </FormControl>
                    <FormLabel className="text-sm">Secret</FormLabel>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="required"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center space-x-2 space-y-0">
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(checked) => field.onChange(Boolean(checked))} />
                    </FormControl>
                    <FormLabel className="text-sm">Required at runtime</FormLabel>
                  </FormItem>
                )}
              />

              <Button type="submit" disabled={createPending}>
                {createPending ? (
                  <>
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    Saving...
                  </>
                ) : (
                  "Save variable"
                )}
              </Button>
            </div>
          </form>
        </Form>

        <Separator />

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium">Bulk seed (.env style)</p>
            {seedPending ? <Icons.Loader className="h-4 w-4 animate-spin text-muted-foreground" /> : null}
          </div>

          <Textarea
            value={seedContent}
            onChange={(e) => setSeedContent(e.target.value)}
            placeholder={"DATABASE_URL=...\nREDIS_URL=...\nAPI_KEY=..."}
            className="min-h-32 font-mono"
          />

          <div className="grid gap-3 md:grid-cols-2">
            <Select value={seedEnv} onValueChange={setSeedEnv}>
              <SelectTrigger>
                <SelectValue placeholder="environment" />
              </SelectTrigger>
              <SelectContent>
                {ENV_OPTIONS.map((env) => (
                  <SelectItem key={env} value={env}>
                    {env}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input value={seedBranch} onChange={(e) => setSeedBranch(e.target.value)} placeholder="branch (optional)" />
          </div>

          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={seedSecret} onCheckedChange={(v) => setSeedSecret(Boolean(v))} />
              secret
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={seedRequired} onCheckedChange={(v) => setSeedRequired(Boolean(v))} />
              required
            </label>
            <Button type="button" variant="outline" onClick={handleSeed} disabled={!seedContent.trim() || seedPending}>
              Seed variables
            </Button>
          </div>
        </div>

        <Separator />

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium">Existing variables</p>
            {loading ? <Icons.Loader className="h-4 w-4 animate-spin text-muted-foreground" /> : null}
          </div>

          {!loading && grouped.length === 0 ? (
            <p className="text-sm text-muted-foreground">No variables yet.</p>
          ) : null}

          {grouped.length > 0 ? (
            <div className="space-y-2">
              {grouped.map((item) => (
                <div key={item.id} className="flex items-start justify-between gap-3 rounded-md border bg-card px-3 py-2">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{item.key}</span>
                      <Badge variant="outline" className="text-[10px]">{item.value_type}</Badge>
                      {item.required ? <Badge variant="outline" className="text-[10px]">required</Badge> : null}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      scope: {item.environment || "default"}
                      {item.branch ? `/${item.branch}` : ""}
                    </p>
                    {item.description ? <p className="text-xs text-muted-foreground">{item.description}</p> : null}
                  </div>
                  <div className="flex gap-2">
                    {item.value_type === "secret" ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={rotatePending}
                        onClick={() => openRotateDialog(item)}
                      >
                        Rotate
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        const description = window.prompt("Description", item.description || "");
                        if (description === null) return;
                        const required = window.confirm("Mark as required?");
                        const enabled = window.confirm("Keep enabled?");
                        onUpdate({ envVarId: item.id, description, required, enabled });
                      }}
                    >
                      Edit
                    </Button>
                    <Button type="button" variant="outline" size="sm" disabled={deletePending} onClick={() => onDelete(item.id)}>
                      Delete
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </CardContent>

      <Dialog open={!!rotateTarget} onOpenChange={(open) => !open && closeRotateDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rotate Secret</DialogTitle>
            <DialogDescription>
              Set a new value for <span className="font-mono">{rotateTarget?.key}</span>.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <label className="text-sm font-medium">New secret value</label>
            <Input
              type="password"
              value={rotateValue}
              onChange={(e) => setRotateValue(e.target.value)}
              placeholder="••••••••"
              autoFocus
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeRotateDialog}>
              Cancel
            </Button>
            <Button type="button" onClick={submitRotate} disabled={!rotateValue.trim() || rotatePending}>
              {rotatePending ? "Rotating..." : "Rotate"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
