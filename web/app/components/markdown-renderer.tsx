import { Streamdown } from "streamdown";
import "streamdown/styles.css";
import { code } from "@streamdown/code";
import { mermaid } from "@streamdown/mermaid";
import { math } from "@streamdown/math";
import { cjk } from "@streamdown/cjk";

export function MarkdownRenderer({ content }: { content: string }) {
  return (
    <Streamdown
      className="custom-streamdown"
      animated
      plugins={{ code, mermaid, math, cjk }}
      shikiTheme={["catppuccin-latte", "catppuccin-mocha"]}
    >
      {content}
    </Streamdown>
  );
}
