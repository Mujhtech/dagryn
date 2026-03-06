import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Matches all ANSI escape sequences (SGR colors/styles, cursor movement, etc.)
// eslint-disable-next-line no-control-regex
const ANSI_RE = /\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?(?:\x07|\x1b\\)/g

/** Strip ANSI escape codes from a string. */
export function stripAnsi(str: string): string {
  return str.replace(ANSI_RE, "")
}
