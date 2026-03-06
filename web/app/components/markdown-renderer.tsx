import { Streamdown } from "streamdown";
import "streamdown/styles.css";
import { mermaid } from "@streamdown/mermaid";
import { math } from "@streamdown/math";
import { cjk } from "@streamdown/cjk";

export function MarkdownRenderer({ content }: { content: string }) {
  return (
    <Streamdown
      className="custom-streamdown"
      animated
      plugins={{ mermaid, math, cjk }}
    >
      {content}
    </Streamdown>
  );
}
