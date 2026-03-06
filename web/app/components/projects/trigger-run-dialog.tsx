import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
} from "~/components/ui/form";
import { Icons } from "~/components/icons";
import type { TriggerRunRequest } from "~/lib/api";

const triggerRunSchema = z.object({
  targets: z.string().optional(),
  git_branch: z.string().optional(),
  description: z.string().optional(),
  force: z.boolean().optional(),
});

type TriggerRunFormValues = z.infer<typeof triggerRunSchema>;

type TriggerRunDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (request: TriggerRunRequest) => void;
  isPending: boolean;
  errorMessage?: string;
  defaultBranch?: string;
};

export function TriggerRunDialog({
  open,
  onOpenChange,
  onSubmit,
  isPending,
  errorMessage,
  defaultBranch,
}: TriggerRunDialogProps) {
  const form = useForm<TriggerRunFormValues>({
    resolver: zodResolver(triggerRunSchema),
    defaultValues: {
      targets: "",
      git_branch: "",
      description: "",
      force: false,
    },
  });

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      form.reset();
    }
    onOpenChange(nextOpen);
  };

  const handleSubmit = (values: TriggerRunFormValues) => {
    const request: TriggerRunRequest = {};

    const targets = values.targets?.trim();
    if (targets) {
      request.targets = targets
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
    }
    if (values.git_branch?.trim()) {
      request.git_branch = values.git_branch.trim();
    }
    if (values.description?.trim()) {
      request.description = values.description.trim();
    }
    if (values.force) {
      request.force = true;
    }

    onSubmit(request);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <Icons.Play className="mr-2 h-4 w-4" />
          Trigger Run
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Trigger Workflow Run</DialogTitle>
          <DialogDescription>
            Start a new server-side workflow run for this project.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(handleSubmit)}
            className="space-y-4 py-4"
          >
            <FormField
              control={form.control}
              name="targets"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Targets (optional)</FormLabel>
                  <FormControl>
                    <Input placeholder="build, test, deploy" {...field} />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="git_branch"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Git Branch (optional)</FormLabel>
                  <FormControl>
                    <Input placeholder={defaultBranch || "main"} {...field} />
                  </FormControl>
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
                    <Textarea
                      placeholder="e.g., Retrying after infra fix"
                      rows={2}
                      {...field}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="force"
              render={({ field }) => (
                <FormItem className="flex items-center space-x-2 space-y-0">
                  <FormControl>
                    <Checkbox
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormLabel className="text-sm font-normal">
                    Force run (ignore cache)
                  </FormLabel>
                </FormItem>
              )}
            />
            {errorMessage ? (
              <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
                {errorMessage}
              </div>
            ) : null}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => handleOpenChange(false)}
                disabled={isPending}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={isPending}>
                {isPending ? (
                  <>
                    <Icons.Loader className="mr-2 h-4 w-4 animate-spin" />
                    Triggering...
                  </>
                ) : (
                  "Trigger Run"
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
