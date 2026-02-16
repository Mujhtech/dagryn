import { useEffect } from "react";
import type { RunStatus } from "~/lib/api";

const ORIGINAL_FAVICON = "/favicon.svg";

const STATUS_COLORS: Record<string, string> = {
  running: "#eab308",
  pending: "#eab308",
  success: "#22c55e",
  failed: "#ef4444",
  cancelled: "#6b7280",
};

function statusIcon(status: RunStatus): string {
  switch (status) {
    case "success":
      // Checkmark
      return `<path d="M44 50 l5 5 l9-9" stroke="white" stroke-width="3" fill="none" stroke-linecap="round" stroke-linejoin="round"/>`;
    case "failed":
      // X mark
      return `<path d="M45 45 l8 8 M53 45 l-8 8" stroke="white" stroke-width="3" fill="none" stroke-linecap="round"/>`;
    case "running":
      // Spinning dot
      return `<circle cx="49" cy="49" r="4" fill="white"><animate attributeName="r" values="3;5;3" dur="1.5s" repeatCount="indefinite"/></circle>`;
    case "pending":
      // Clock-like dots
      return `<circle cx="49" cy="44" r="2" fill="white"/><circle cx="49" cy="54" r="2" fill="white"/><circle cx="44" cy="49" r="2" fill="white"/><circle cx="54" cy="49" r="2" fill="white"/>`;
    case "cancelled":
      // Dash
      return `<path d="M44 49 h10" stroke="white" stroke-width="3" stroke-linecap="round"/>`;
    default:
      return `<circle cx="49" cy="49" r="4" fill="white"/>`;
  }
}

function buildFaviconSvg(status: RunStatus): string {
  const color = STATUS_COLORS[status] ?? "#6b7280";

  return `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
<rect width="64" height="64" rx="8" fill="black"/>
<path d="M4.6 32.1 18.3 8.3 45.7 8.3 59.4 32 45.7 55.7 18.3 55.7Z" fill="white"/>
<circle cx="49" cy="49" r="12" fill="${color}"/>
${statusIcon(status)}
</svg>`;
}

function setFavicon(href: string) {
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    document.head.appendChild(link);
  }
  link.href = href;
}

export function useFavicon(status: RunStatus | null) {
  useEffect(() => {
    if (!status) {
      setFavicon(ORIGINAL_FAVICON);
      return;
    }

    const svg = buildFaviconSvg(status);
    const dataUrl = `data:image/svg+xml,${encodeURIComponent(svg)}`;
    setFavicon(dataUrl);

    return () => {
      setFavicon(ORIGINAL_FAVICON);
    };
  }, [status]);
}
