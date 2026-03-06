import { TasksWaterfall } from "~/components/projects/run-detail/tasks-waterfall";
import { ShowcaseFrame } from "./showcase-frame";
import { SHOWCASE_TASK_RESULTS } from "./showcase-data";

export default function ShowcaseWaterfall() {
  return (
    <ShowcaseFrame
      label="Execution"
      title="Watch parallel execution unfold"
      description="Track task timing, cache hits, and parallel overlap in a compact waterfall view."
      gradient="bottom"
      maxHeight="28rem"
    >
      <TasksWaterfall tasks={SHOWCASE_TASK_RESULTS} />
    </ShowcaseFrame>
  );
}
