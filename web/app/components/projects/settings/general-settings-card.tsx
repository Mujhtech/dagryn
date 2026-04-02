import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { FolderCog } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";
import { Icons } from "~/components/icons";
import type { Project } from "~/lib/api";

const generalSettingsSchema = z.object({
  name: z.string().min(1, "Project name is required"),
  description: z.string().optional(),
  visibility: z.enum(["private", "public"]),
});

export type GeneralSettingsFormValues = z.infer<typeof generalSettingsSchema>;

type GeneralSettingsCardProps = {
  project: Project;
  onSave: (values: GeneralSettingsFormValues) => void;
  isSaving: boolean;
  saveError?: string;
  saveSuccess: boolean;
};

export function GeneralSettingsCard({
  project,
  onSave,
  isSaving,
  saveError,
  saveSuccess,
}: GeneralSettingsCardProps) {
  const form = useForm<GeneralSettingsFormValues>({
    resolver: zodResolver(generalSettingsSchema),
    defaultValues: {
      name: project.name || "",
      description: project.description || "",
      visibility: (project.visibility as "public" | "private") || "private",
    },
  });

  useEffect(() => {
    form.reset({
      name: project.name || "",
      description: project.description || "",
      visibility: (project.visibility as "public" | "private") || "private",
    });
  }, [project, form]);

  const handleSubmit = (values: GeneralSettingsFormValues) => {
    onSave(values);
  };

  const visibility = form.watch("visibility");

  return (
    <Card className="py-6">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <FolderCog className="h-5 w-5" />
          General
        </CardTitle>
        <CardDescription>Update your project&apos;s basic information.</CardDescription>
      </CardHeader>
      <Form {...form}>
        <form onSubmit={form.handleSubmit(handleSubmit)}>
          <CardContent className="space-y-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Project Name</FormLabel>
                  <FormControl>
                    <Input placeholder="My Project" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Description</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder="A brief description of your project"
                      className="resize-none"
                      rows={3}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="visibility"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Visibility</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Select visibility" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="private">
                        Private - Only you and team members can see this project
                      </SelectItem>
                      <SelectItem value="public">
                        Public - Anyone can see this project
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {visibility === "public"
                      ? "Public projects are visible to everyone."
                      : "Private projects are only visible to you and your team."}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {saveError ? (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {saveError}
              </div>
            ) : null}

            {saveSuccess ? (
              <div className="rounded-md bg-green-500/10 p-3 text-sm text-green-600 dark:text-green-400">
                Project settings updated successfully!
              </div>
            ) : null}
          </CardContent>
          <CardFooter>
            <Button type="submit" disabled={isSaving}>
              {isSaving ? (
                <>
                  <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Icons.FloppyDisk className="mr-2 h-4 w-4" />
                  Save Changes
                </>
              )}
            </Button>
          </CardFooter>
        </form>
      </Form>
    </Card>
  );
}
