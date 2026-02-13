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
import { Label } from "~/components/ui/label";
import { Icons } from "~/components/icons";

type TriggerRunDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  triggerTargets: string;
  setTriggerTargets: (targets: string) => void;
  triggerBranch: string;
  setTriggerBranch: (branch: string) => void;
  triggerForce: boolean;
  setTriggerForce: (force: boolean) => void;
  onTriggerRun: () => void;
  isPending: boolean;
  errorMessage?: string;
};

export function TriggerRunDialog({
  open,
  onOpenChange,
  triggerTargets,
  setTriggerTargets,
  triggerBranch,
  setTriggerBranch,
  triggerForce,
  setTriggerForce,
  onTriggerRun,
  isPending,
  errorMessage,
}: TriggerRunDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
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
            Start a new workflow run for this project.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="targets">Targets (optional)</Label>
            <Input
              id="targets"
              value={triggerTargets}
              onChange={(e) => setTriggerTargets(e.target.value)}
              placeholder="build, test, deploy"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="branch">Git Branch (optional)</Label>
            <Input
              id="branch"
              value={triggerBranch}
              onChange={(e) => setTriggerBranch(e.target.value)}
              placeholder="main"
            />
          </div>
          <div className="flex items-center space-x-2">
            <Checkbox
              id="force"
              checked={triggerForce}
              onCheckedChange={(checked) => setTriggerForce(checked === true)}
            />
            <Label htmlFor="force" className="text-sm font-normal">
              Force run (ignore cache)
            </Label>
          </div>
          {errorMessage ? (
            <div className="rounded-none bg-destructive/10 p-3 text-sm text-destructive">
              {errorMessage}
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isPending}
          >
            Cancel
          </Button>
          <Button onClick={onTriggerRun} disabled={isPending}>
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
      </DialogContent>
    </Dialog>
  );
}
