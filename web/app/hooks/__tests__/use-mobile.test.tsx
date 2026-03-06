import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useIsMobile } from "../use-mobile";

describe("useIsMobile", () => {
  let listeners: Array<(e: unknown) => void>;
  let mockMql: {
    addEventListener: ReturnType<typeof vi.fn>;
    removeEventListener: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    listeners = [];
    mockMql = {
      addEventListener: vi.fn((_event: string, cb: (e: unknown) => void) => {
        listeners.push(cb);
      }),
      removeEventListener: vi.fn(),
    };

    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => mockMql),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns false for desktop width (>= 768px)", () => {
    Object.defineProperty(window, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);
  });

  it("returns true for mobile width (< 768px)", () => {
    Object.defineProperty(window, "innerWidth", {
      value: 375,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(true);
  });

  it("returns false at exact breakpoint (768px)", () => {
    Object.defineProperty(window, "innerWidth", {
      value: 768,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);
  });

  it("responds to matchMedia change events", () => {
    Object.defineProperty(window, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);

    // Simulate resize to mobile
    Object.defineProperty(window, "innerWidth", {
      value: 375,
      writable: true,
      configurable: true,
    });

    act(() => {
      listeners.forEach((cb) => cb({}));
    });

    expect(result.current).toBe(true);
  });

  it("cleans up event listener on unmount", () => {
    Object.defineProperty(window, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { unmount } = renderHook(() => useIsMobile());
    unmount();

    expect(mockMql.removeEventListener).toHaveBeenCalledWith(
      "change",
      expect.any(Function),
    );
  });

  it("creates matchMedia with correct query", () => {
    Object.defineProperty(window, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    renderHook(() => useIsMobile());

    expect(window.matchMedia).toHaveBeenCalledWith("(max-width: 767px)");
  });
});
