import { describe, it, expect } from "vitest";
import { generateMetadata } from "../metadata";

describe("generateMetadata", () => {
  it("should return title and custom description", async () => {
    const result = await generateMetadata({
      title: "Dashboard",
      description: "Project overview",
    });

    expect(result).toEqual({
      meta: [
        { title: "Dashboard" },
        { name: "description", content: "Project overview" },
      ],
    });
  });

  it("should use default description when none provided", async () => {
    const result = await generateMetadata({ title: "Settings" });

    expect(result.meta[1]).toEqual({
      name: "description",
      content: "Local-first, self-hosted developer workflow orchestrator",
    });
  });

  it("should use default description when description is empty string", async () => {
    const result = await generateMetadata({ title: "Page", description: "" });

    expect(result.meta[1]).toEqual({
      name: "description",
      content: "Local-first, self-hosted developer workflow orchestrator",
    });
  });

  it("should always include title as first meta entry", async () => {
    const result = await generateMetadata({ title: "My Title" });
    expect(result.meta[0]).toEqual({ title: "My Title" });
  });

  it("should return exactly two meta entries", async () => {
    const result = await generateMetadata({
      title: "Test",
      description: "Desc",
    });
    expect(result.meta).toHaveLength(2);
  });
});
