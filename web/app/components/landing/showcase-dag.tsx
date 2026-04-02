import { WorkflowDag } from "~/components/workflow-dag";
import { ShowcaseFrame } from "./showcase-frame";
import { SHOWCASE_WORKFLOW, SHOWCASE_TASK_STATUSES } from "./showcase-data";

export default function ShowcaseDag() {
  return (
    <ShowcaseFrame
      label="Pipeline"
      title="See your pipeline as a graph"
      description="Visualize task dependencies, parallel groups, and conditional branches at a glance."
      gradient="bottom"
      maxHeight="32rem"
    >
      <div className="min-h-112 w-full">
        <WorkflowDag
          workflow={SHOWCASE_WORKFLOW}
          taskStatuses={SHOWCASE_TASK_STATUSES}
          className="w-full"
        />
      </div>
    </ShowcaseFrame>
  );
}
