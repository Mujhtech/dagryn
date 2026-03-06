import { describe, it, expect } from "vitest";
import { cn, stripAnsi } from "../utils";

describe("cn utility", () => {
  it("should merge class names", () => {
    expect(cn("foo", "bar")).toBe("foo bar");
  });

  it("should handle conditional classes", () => {
    expect(cn("foo", false && "bar", "baz")).toBe("foo baz");
    expect(cn("foo", true && "bar", "baz")).toBe("foo bar baz");
  });

  it("should handle undefined and null values", () => {
    expect(cn("foo", undefined, null, "bar")).toBe("foo bar");
  });

  it("should merge tailwind classes correctly", () => {
    expect(cn("px-2 py-1", "px-4")).toBe("py-1 px-4");
    expect(cn("text-red-500", "text-blue-500")).toBe("text-blue-500");
  });

  it("should handle array inputs", () => {
    expect(cn(["foo", "bar"])).toBe("foo bar");
  });

  it("should handle object inputs", () => {
    expect(cn({ foo: true, bar: false, baz: true })).toBe("foo baz");
  });

  it("should handle empty inputs", () => {
    expect(cn()).toBe("");
    expect(cn("")).toBe("");
  });
});

describe("stripAnsi", () => {
  it("should strip SGR color codes", () => {
    expect(stripAnsi("\x1b[31mred text\x1b[0m")).toBe("red text");
    expect(stripAnsi("\x1b[32mgreen\x1b[39m")).toBe("green");
  });

  it("should strip bold, underline, and other style codes", () => {
    expect(stripAnsi("\x1b[1mbold\x1b[22m")).toBe("bold");
    expect(stripAnsi("\x1b[4munderline\x1b[24m")).toBe("underline");
    expect(stripAnsi("\x1b[3mitalic\x1b[23m")).toBe("italic");
  });

  it("should strip multi-param SGR sequences", () => {
    expect(stripAnsi("\x1b[1;31;42mbold red on green\x1b[0m")).toBe(
      "bold red on green",
    );
  });

  it("should strip OSC sequences (BEL terminated)", () => {
    expect(stripAnsi("\x1b]0;Window Title\x07some text")).toBe("some text");
  });

  it("should strip OSC sequences (ST terminated)", () => {
    expect(stripAnsi("\x1b]0;Window Title\x1b\\some text")).toBe("some text");
  });

  it("should strip cursor movement sequences", () => {
    expect(stripAnsi("\x1b[2Amove up")).toBe("move up");
    expect(stripAnsi("\x1b[10Bdown")).toBe("down");
    expect(stripAnsi("\x1b[Hhome")).toBe("home");
  });

  it("should return empty string for empty input", () => {
    expect(stripAnsi("")).toBe("");
  });

  it("should pass through strings without ANSI codes unchanged", () => {
    const plain = "Hello, world! 123 @#$%";
    expect(stripAnsi(plain)).toBe(plain);
  });

  it("should handle multiple ANSI sequences in one string", () => {
    expect(
      stripAnsi("\x1b[31mred\x1b[0m and \x1b[32mgreen\x1b[0m"),
    ).toBe("red and green");
  });
});
