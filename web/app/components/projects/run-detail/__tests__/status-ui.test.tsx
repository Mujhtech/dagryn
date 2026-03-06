import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  formatDuration,
  formatBytes,
  RunStatusIcon,
  TaskStatusIcon,
  StatusBadge,
} from "../status-ui";

describe("formatDuration", () => {
  it("formats sub-second durations", () => {
    expect(formatDuration(0)).toBe("0ms");
    expect(formatDuration(1)).toBe("1ms");
    expect(formatDuration(500)).toBe("500ms");
    expect(formatDuration(999)).toBe("999ms");
  });

  it("formats seconds", () => {
    expect(formatDuration(1000)).toBe("1.0s");
    expect(formatDuration(2500)).toBe("2.5s");
    expect(formatDuration(59999)).toBe("60.0s");
  });

  it("formats minutes and seconds", () => {
    expect(formatDuration(60000)).toBe("1m 0s");
    expect(formatDuration(90000)).toBe("1m 30s");
    expect(formatDuration(330000)).toBe("5m 30s");
    expect(formatDuration(3600000)).toBe("60m 0s");
  });
});

describe("formatBytes", () => {
  it("formats 0 bytes", () => {
    expect(formatBytes(0)).toBe("0 B");
  });

  it("formats bytes", () => {
    expect(formatBytes(1)).toBe("1 B");
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1023)).toBe("1023 B");
  });

  it("formats kilobytes", () => {
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(10240)).toBe("10 KB");
  });

  it("formats megabytes", () => {
    expect(formatBytes(1048576)).toBe("1.0 MB");
    expect(formatBytes(5242880)).toBe("5.0 MB");
    expect(formatBytes(10485760)).toBe("10 MB");
  });

  it("formats gigabytes", () => {
    expect(formatBytes(1073741824)).toBe("1.0 GB");
  });

  it("formats terabytes", () => {
    expect(formatBytes(1099511627776)).toBe("1.0 TB");
  });
});

describe("RunStatusIcon", () => {
  it("renders green icon for success", () => {
    const { container } = render(<RunStatusIcon status="success" />);
    const svg = container.querySelector("svg");
    expect(svg).toBeTruthy();
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-green-500",
    );
  });

  it("renders red icon for failed", () => {
    const { container } = render(<RunStatusIcon status="failed" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-red-500",
    );
  });

  it("renders blue animated icon for running", () => {
    const { container } = render(<RunStatusIcon status="running" />);
    const svg = container.querySelector("svg");
    const classes = svg?.className.baseVal || svg?.getAttribute("class") || "";
    expect(classes).toContain("text-blue-500");
    expect(classes).toContain("animate-spin");
  });

  it("renders yellow icon for pending", () => {
    const { container } = render(<RunStatusIcon status="pending" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-yellow-500",
    );
  });

  it("renders gray icon for cancelled", () => {
    const { container } = render(<RunStatusIcon status="cancelled" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-gray-500",
    );
  });

  it("renders yellow wifi-off icon for stale", () => {
    const { container } = render(<RunStatusIcon status="stale" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-yellow-500",
    );
  });

  it("renders gray icon for unknown status", () => {
    const { container } = render(<RunStatusIcon status="whatever" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-gray-400",
    );
  });

  it("forwards custom className", () => {
    const { container } = render(
      <RunStatusIcon status="success" className="h-8 w-8" />,
    );
    const svg = container.querySelector("svg");
    const classes = svg?.className.baseVal || svg?.getAttribute("class") || "";
    expect(classes).toContain("h-8");
    expect(classes).toContain("w-8");
  });
});

describe("TaskStatusIcon", () => {
  it("renders green icon for success", () => {
    const { container } = render(<TaskStatusIcon status="success" />);
    const svg = container.querySelector("svg");
    const classes = svg?.className.baseVal || svg?.getAttribute("class") || "";
    expect(classes).toContain("text-green-500");
    expect(classes).toContain("h-5");
    expect(classes).toContain("w-5");
  });

  it("renders red icon for failed", () => {
    const { container } = render(<TaskStatusIcon status="failed" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-red-500",
    );
  });

  it("renders blue spinning icon for running", () => {
    const { container } = render(<TaskStatusIcon status="running" />);
    const svg = container.querySelector("svg");
    const classes = svg?.className.baseVal || svg?.getAttribute("class") || "";
    expect(classes).toContain("text-blue-500");
    expect(classes).toContain("animate-spin");
  });

  it("renders purple icon for cached", () => {
    const { container } = render(<TaskStatusIcon status="cached" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-purple-500",
    );
  });

  it("renders yellow icon for pending", () => {
    const { container } = render(<TaskStatusIcon status="pending" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-yellow-500",
    );
  });

  it("renders gray icon for unknown status", () => {
    const { container } = render(<TaskStatusIcon status="unknown" />);
    const svg = container.querySelector("svg");
    expect(svg?.className.baseVal || svg?.getAttribute("class")).toContain(
      "text-gray-400",
    );
  });
});

describe("StatusBadge", () => {
  it("renders 'Connection Lost' label for stale status", () => {
    render(<StatusBadge status="stale" />);
    expect(screen.getByText("Connection Lost")).toBeTruthy();
  });

  it("capitalizes unknown status names", () => {
    render(<StatusBadge status="pending" />);
    expect(screen.getByText("Pending")).toBeTruthy();
  });

  it("capitalizes first letter of arbitrary status", () => {
    render(<StatusBadge status="queued" />);
    expect(screen.getByText("Queued")).toBeTruthy();
  });

  it("renders success status", () => {
    render(<StatusBadge status="success" />);
    expect(screen.getByText("Success")).toBeTruthy();
  });

  it("renders failed status", () => {
    render(<StatusBadge status="failed" />);
    expect(screen.getByText("Failed")).toBeTruthy();
  });

  it("renders running status", () => {
    render(<StatusBadge status="running" />);
    expect(screen.getByText("Running")).toBeTruthy();
  });

  it("renders cancelled status", () => {
    render(<StatusBadge status="cancelled" />);
    expect(screen.getByText("Cancelled")).toBeTruthy();
  });
});
