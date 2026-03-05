import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useSSOConnection } from "~/hooks/queries";
import {
  useCreateSSOConnection,
  useUpdateSSOConnection,
  useDeleteSSOConnection,
  useTestSSOConnection,
  useToggleSSOEnforcement,
  useGenerateSCIMToken,
  useRotateSCIMToken,
} from "~/hooks/mutations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { Label } from "~/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Switch } from "~/components/ui/switch";
import { Badge } from "~/components/ui/badge";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";
import { generateMetadata } from "~/lib/metadata";

export const Route = createFileRoute(
  "/_dashboard_layout/teams/$teamId/sso",
)({
  component: SSOSettingsPage,
  head: () => {
    return generateMetadata({ title: "SSO Settings" });
  },
});

const ssoFormSchema = z.object({
  idp_metadata_url: z.url("Must be a valid URL").or(z.literal("")),
  idp_entity_id: z.url("Must be a valid URL").or(z.literal("")),
  idp_sso_url: z.url("Must be a valid URL").or(z.literal("")),
  certificate: z.string(),
});

const createSSOSchema = ssoFormSchema.superRefine((data, ctx) => {
  const hasMetadataUrl = data.idp_metadata_url !== "";
  if (!hasMetadataUrl) {
    if (!data.idp_entity_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Required when metadata URL is not provided",
        path: ["idp_entity_id"],
      });
    }
    if (!data.idp_sso_url) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Required when metadata URL is not provided",
        path: ["idp_sso_url"],
      });
    }
    if (!data.certificate) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Required when metadata URL is not provided",
        path: ["certificate"],
      });
    }
  }
});

type SSOFormValues = z.infer<typeof ssoFormSchema>;

function SSOSettingsPage() {
  const { teamId } = Route.useParams();
  const { data: connection, isLoading, error } = useSSOConnection(teamId);

  const createConnection = useCreateSSOConnection(teamId);
  const updateConnection = useUpdateSSOConnection(teamId);
  const deleteConnection = useDeleteSSOConnection(teamId);
  const testConnection = useTestSSOConnection(teamId);
  const toggleEnforcement = useToggleSSOEnforcement(teamId);
  const generateToken = useGenerateSCIMToken(teamId);
  const rotateToken = useRotateSCIMToken(teamId);

  const [testResult, setTestResult] = useState<{
    success: boolean;
    error?: string;
  } | null>(null);
  const [scimToken, setScimToken] = useState<string | null>(null);

  const hasConnection = !!connection && !error;

  const form = useForm<SSOFormValues>({
    resolver: zodResolver(hasConnection ? ssoFormSchema : createSSOSchema),
    defaultValues: {
      idp_metadata_url: "",
      idp_entity_id: "",
      idp_sso_url: "",
      certificate: "",
    },
  });

  const handleSubmit = async (values: SSOFormValues) => {
    if (hasConnection) {
      await updateConnection.mutateAsync({
        idp_entity_id: values.idp_entity_id || undefined,
        idp_sso_url: values.idp_sso_url || undefined,
        idp_metadata_url: values.idp_metadata_url || undefined,
        certificate: values.certificate || undefined,
      });
    } else {
      await createConnection.mutateAsync({
        idp_entity_id: values.idp_entity_id,
        idp_sso_url: values.idp_sso_url,
        idp_metadata_url: values.idp_metadata_url || undefined,
        certificate: values.certificate,
      });
    }
  };

  const handleTest = async () => {
    const result = await testConnection.mutateAsync();
    setTestResult(result);
  };

  const handleGenerateToken = async () => {
    const result = await generateToken.mutateAsync();
    setScimToken(result.token);
  };

  const handleRotateToken = async () => {
    const result = await rotateToken.mutateAsync();
    setScimToken(result.token);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-muted-foreground">Loading SSO settings...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">SSO Settings</h2>
        <p className="text-muted-foreground">
          Configure SAML 2.0 Single Sign-On and SCIM provisioning for your team.
        </p>
      </div>

      <Tabs defaultValue="saml">
        <TabsList>
          <TabsTrigger value="saml">SAML Configuration</TabsTrigger>
          <TabsTrigger value="scim">SCIM Provisioning</TabsTrigger>
        </TabsList>

        <TabsContent value="saml" className="space-y-4">
          {hasConnection && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  Service Provider Details
                  <Badge variant="secondary">Configured</Badge>
                </CardTitle>
                <CardDescription>
                  Share these details with your identity provider.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div>
                  <Label className="text-xs text-muted-foreground">
                    SP Entity ID
                  </Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 rounded bg-muted px-2 py-1 text-sm">
                      {connection.sp_entity_id}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        navigator.clipboard.writeText(connection.sp_entity_id)
                      }
                    >
                      Copy
                    </Button>
                  </div>
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground">
                    ACS URL
                  </Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 rounded bg-muted px-2 py-1 text-sm">
                      {connection.sp_acs_url}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        navigator.clipboard.writeText(connection.sp_acs_url)
                      }
                    >
                      Copy
                    </Button>
                  </div>
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground">
                    Metadata URL
                  </Label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 rounded bg-muted px-2 py-1 text-sm">
                      {connection.sp_entity_id}/metadata
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        navigator.clipboard.writeText(
                          `${connection.sp_entity_id}/metadata`,
                        )
                      }
                    >
                      Copy
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>
                {hasConnection
                  ? "Update IdP Configuration"
                  : "Configure Identity Provider"}
              </CardTitle>
              <CardDescription>
                Enter your identity provider's SAML settings. You can either
                provide a metadata URL or enter the details manually.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Form {...form}>
                <form
                  onSubmit={form.handleSubmit(handleSubmit)}
                  className="space-y-4"
                >
                  <FormField
                    control={form.control}
                    name="idp_metadata_url"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          IdP Metadata URL{" "}
                          {hasConnection ? "(optional)" : "(recommended)"}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder="https://your-idp.com/metadata"
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          If provided, Entity ID and SSO URL will be
                          auto-detected.
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="idp_entity_id"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>IdP Entity ID</FormLabel>
                        <FormControl>
                          <Input
                            placeholder="https://your-idp.com/entity"
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="idp_sso_url"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>IdP SSO URL</FormLabel>
                        <FormControl>
                          <Input
                            placeholder="https://your-idp.com/sso"
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="certificate"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>IdP Certificate (PEM)</FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder={
                              "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
                            }
                            className="min-h-25 font-mono text-xs"
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex gap-2">
                    <Button
                      type="submit"
                      disabled={
                        createConnection.isPending ||
                        updateConnection.isPending
                      }
                    >
                      {hasConnection ? "Update" : "Configure SSO"}
                    </Button>
                    {hasConnection && (
                      <>
                        <Button
                          type="button"
                          variant="outline"
                          onClick={handleTest}
                          disabled={testConnection.isPending}
                        >
                          {testConnection.isPending
                            ? "Testing..."
                            : "Test Connection"}
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          onClick={() => deleteConnection.mutate()}
                          disabled={deleteConnection.isPending}
                        >
                          Remove SSO
                        </Button>
                      </>
                    )}
                  </div>

                  {testResult && (
                    <div
                      className={`rounded-md p-3 text-sm ${testResult.success ? "bg-green-50 text-green-800 dark:bg-green-950 dark:text-green-200" : "bg-red-50 text-red-800 dark:bg-red-950 dark:text-red-200"}`}
                    >
                      {testResult.success
                        ? "Connection test passed!"
                        : `Connection test failed: ${testResult.error}`}
                    </div>
                  )}
                </form>
              </Form>
            </CardContent>
          </Card>

          {hasConnection && (
            <Card>
              <CardHeader>
                <CardTitle>SSO Enforcement</CardTitle>
                <CardDescription>
                  When enabled, all team members must authenticate via SSO.
                  Non-SAML logins will be blocked.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-3">
                  <Switch
                    checked={connection.enforce_sso}
                    onCheckedChange={(checked) =>
                      toggleEnforcement.mutate(checked)
                    }
                    disabled={toggleEnforcement.isPending}
                  />
                  <Label>
                    {connection.enforce_sso
                      ? "SSO is required for all team members"
                      : "SSO is optional"}
                  </Label>
                </div>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="scim" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>SCIM 2.0 Provisioning</CardTitle>
              <CardDescription>
                Automatically sync users and groups from your identity provider.
                {!hasConnection &&
                  " Configure SAML first to enable SCIM provisioning."}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {hasConnection ? (
                <>
                  <div className="flex items-center gap-3">
                    <Badge
                      variant={
                        connection.scim_enabled ? "default" : "secondary"
                      }
                    >
                      {connection.scim_enabled ? "Enabled" : "Disabled"}
                    </Badge>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">
                      SCIM Endpoint
                    </Label>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 rounded bg-muted px-2 py-1 text-sm">
                        {connection.sp_entity_id.replace(
                          "/api/v1/sso/",
                          "/api/v1/scim/",
                        )}
                      </code>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          navigator.clipboard.writeText(
                            connection.sp_entity_id.replace(
                              "/api/v1/sso/",
                              "/api/v1/scim/",
                            ),
                          )
                        }
                      >
                        Copy
                      </Button>
                    </div>
                  </div>

                  <div className="flex gap-2">
                    <Button onClick={handleGenerateToken}>
                      {connection.scim_enabled
                        ? "Regenerate Token"
                        : "Generate SCIM Token"}
                    </Button>
                    {connection.scim_enabled && (
                      <Button variant="outline" onClick={handleRotateToken}>
                        Rotate Token
                      </Button>
                    )}
                  </div>

                  {scimToken && (
                    <div className="rounded-md border bg-muted p-3">
                      <p className="mb-2 text-sm font-medium text-amber-600 dark:text-amber-400">
                        Copy this token now. It will not be shown again.
                      </p>
                      <div className="flex items-center gap-2">
                        <code className="flex-1 break-all text-sm">
                          {scimToken}
                        </code>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() =>
                            navigator.clipboard.writeText(scimToken)
                          }
                        >
                          Copy
                        </Button>
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <p className="text-sm text-muted-foreground">
                  Please configure SAML SSO first before enabling SCIM
                  provisioning.
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
