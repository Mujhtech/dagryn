// SSE client for real-time updates
import { EventSourcePolyfill } from "event-source-polyfill";
// import { isEqual, isEmpty } from "lodash-es";

export interface SSEEvent {
  id: string;
  type: string;
  timestamp: string;
  data: unknown;
}

export interface RunEventData {
  run_id: string;
  project_id: string;
  status: string;
  error_message?: string;
}

export interface TaskEventData {
  run_id: string;
  task_name: string;
  status: string;
  exit_code?: number;
  duration_ms?: number;
  cache_hit?: boolean;
  cache_key?: string;
}

export interface LogEventData {
  run_id: string;
  task_name?: string;
  stream: "stdout" | "stderr";
  line: string;
  line_num: number;
}

type EventCallback<T> = (data: T) => void;

export class RunStreamClient {
  private eventSource: EventSource | null = null;
  private callbacks = {
    runStarted: [] as EventCallback<RunEventData>[],
    runCompleted: [] as EventCallback<RunEventData>[],
    runFailed: [] as EventCallback<RunEventData>[],
    runCancelled: [] as EventCallback<RunEventData>[],
    taskStarted: [] as EventCallback<TaskEventData>[],
    taskCompleted: [] as EventCallback<TaskEventData>[],
    taskFailed: [] as EventCallback<TaskEventData>[],
    taskCached: [] as EventCallback<TaskEventData>[],
    log: [] as EventCallback<LogEventData>[],
    connected: [] as EventCallback<{ client_id: string }>[],
    error: [] as EventCallback<Event>[],
  };

  connect(projectId: string, runId: string, token?: string) {
    console.log("Connecting to SSE", projectId, runId, token);
    const url = `/api/v1/projects/${projectId}/runs/${runId}/events`;

    // Note: EventSource doesn't support custom headers, so we'd need to use
    // query params or cookies for auth in production

    const accessToken = localStorage.getItem("access_token");

    const options: {
      heartbeatTimeout: number;
      headers?: { [key: string]: string };
    } = {
      heartbeatTimeout: 999999999,
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    };

    const eventSource = new EventSourcePolyfill(url.toString(), options);
    this.eventSource = eventSource;

    this.eventSource.addEventListener("connected", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.connected.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("run.started", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.runStarted.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("run.completed", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.runCompleted.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("run.failed", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.runFailed.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("run.cancelled", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.runCancelled.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("task.started", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.taskStarted.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("task.completed", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.taskCompleted.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("task.failed", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.taskFailed.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("task.cached", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.taskCached.forEach((cb) => cb(data.data));
    });

    this.eventSource.addEventListener("log", (event) => {
      const data = JSON.parse(event.data);
      this.callbacks.log.forEach((cb) => cb(data.data));
    });

    this.eventSource.onerror = (event) => {
      this.callbacks.error.forEach((cb) => cb(event));
    };
  }

  disconnect() {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
  }

  onRunStarted(callback: EventCallback<RunEventData>) {
    this.callbacks.runStarted.push(callback);
    return () => {
      this.callbacks.runStarted = this.callbacks.runStarted.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onRunCompleted(callback: EventCallback<RunEventData>) {
    this.callbacks.runCompleted.push(callback);
    return () => {
      this.callbacks.runCompleted = this.callbacks.runCompleted.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onRunFailed(callback: EventCallback<RunEventData>) {
    this.callbacks.runFailed.push(callback);
    return () => {
      this.callbacks.runFailed = this.callbacks.runFailed.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onRunCancelled(callback: EventCallback<RunEventData>) {
    this.callbacks.runCancelled.push(callback);
    return () => {
      this.callbacks.runCancelled = this.callbacks.runCancelled.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onTaskStarted(callback: EventCallback<TaskEventData>) {
    this.callbacks.taskStarted.push(callback);
    return () => {
      this.callbacks.taskStarted = this.callbacks.taskStarted.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onTaskCompleted(callback: EventCallback<TaskEventData>) {
    this.callbacks.taskCompleted.push(callback);
    return () => {
      this.callbacks.taskCompleted = this.callbacks.taskCompleted.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onTaskFailed(callback: EventCallback<TaskEventData>) {
    this.callbacks.taskFailed.push(callback);
    return () => {
      this.callbacks.taskFailed = this.callbacks.taskFailed.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onTaskCached(callback: EventCallback<TaskEventData>) {
    this.callbacks.taskCached.push(callback);
    return () => {
      this.callbacks.taskCached = this.callbacks.taskCached.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onLog(callback: EventCallback<LogEventData>) {
    this.callbacks.log.push(callback);
    return () => {
      this.callbacks.log = this.callbacks.log.filter((cb) => cb !== callback);
    };
  }

  onConnected(callback: EventCallback<{ client_id: string }>) {
    this.callbacks.connected.push(callback);
    return () => {
      this.callbacks.connected = this.callbacks.connected.filter(
        (cb) => cb !== callback,
      );
    };
  }

  onError(callback: EventCallback<Event>) {
    this.callbacks.error.push(callback);
    return () => {
      this.callbacks.error = this.callbacks.error.filter(
        (cb) => cb !== callback,
      );
    };
  }
}
