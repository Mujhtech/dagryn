import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTeams } from "~/hooks/queries";
import { useCreateProject } from "~/hooks/mutations";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
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

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

const createProjectSchema = z.object({
  name: z.string().min(1, "Project name is required"),
  slug: z
    .string()
    .min(1, "Slug is required")
    .regex(
      /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
      "Slug must be lowercase alphanumeric with hyphens",
    ),
  description: z.string().optional(),
  visibility: z.enum(["private", "public"]),
  team_id: z.string().optional(),
});

type CreateProjectFormValues = z.infer<typeof createProjectSchema>;

export function CreateProjectDialog() {
  const navigate = useNavigate();
  const createProjectMutation = useCreateProject();
  const { data: teamsData } = useTeams();
  const teams = teamsData?.data ?? [];

  const [isOpen, setIsOpen] = useState(false);
  const [slugEdited, setSlugEdited] = useState(false);

  const form = useForm<CreateProjectFormValues>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: {
      name: "",
      slug: "",
      description: "",
      visibility: "private",
      team_id: "none",
    },
  });

  const handleOpenChange = (open: boolean) => {
    setIsOpen(open);
    if (!open) {
      form.reset();
      setSlugEdited(false);
      createProjectMutation.reset();
    }
  };

  const onSubmit = (values: CreateProjectFormValues) => {
    createProjectMutation.mutate(
      {
        name: values.name,
        slug: values.slug,
        description: values.description || undefined,
        visibility: values.visibility,
        team_id:
          values.team_id && values.team_id !== "none"
            ? values.team_id
            : undefined,
      },
      {
        onSuccess: (project) => {
          handleOpenChange(false);
          navigate({
            to: "/projects/$projectId",
            params: { projectId: project.id },
          });
        },
      },
    );
  };

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <Icons.Plus className="mr-2 h-4 w-4" />
          New Project
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-106.25">
        <DialogHeader>
          <DialogTitle>Create Project</DialogTitle>
          <DialogDescription>
            Create a new workflow project. You can configure workflows after
            creation.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="grid gap-4 py-4"
          >
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Name</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="My Project"
                      disabled={createProjectMutation.isPending}
                      {...field}
                      onChange={(e) => {
                        field.onChange(e);
                        if (!slugEdited) {
                          form.setValue("slug", slugify(e.target.value));
                        }
                      }}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="slug"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Slug</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="my-project"
                      disabled={createProjectMutation.isPending}
                      className="font-mono"
                      {...field}
                      onChange={(e) => {
                        field.onChange(e);
                        setSlugEdited(true);
                      }}
                    />
                  </FormControl>
                  <FormDescription>
                    URL-friendly identifier for your project
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Description (optional)</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="A brief description of your project"
                      disabled={createProjectMutation.isPending}
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
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      variant={
                        field.value === "private" ? "default" : "outline"
                      }
                      size="sm"
                      onClick={() => field.onChange("private")}
                      disabled={createProjectMutation.isPending}
                    >
                      Private
                    </Button>
                    <Button
                      type="button"
                      variant={
                        field.value === "public" ? "default" : "outline"
                      }
                      size="sm"
                      onClick={() => field.onChange("public")}
                      disabled={createProjectMutation.isPending}
                    >
                      Public
                    </Button>
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            {teams.length > 0 && (
              <FormField
                control={form.control}
                name="team_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Team (optional)</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={createProjectMutation.isPending}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="No team" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="none">No team</SelectItem>
                        {teams.map((team) => (
                          <SelectItem key={team.id} value={team.id}>
                            {team.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      Assign this project to a team
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            {createProjectMutation.error && (
              <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                {createProjectMutation.error.message}
              </div>
            )}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => handleOpenChange(false)}
                disabled={createProjectMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={createProjectMutation.isPending}
              >
                {createProjectMutation.isPending ? (
                  <>
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  "Create Project"
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
