import { useEffect, useRef, useState } from "react";

const script = [
  {
    command: "dagryn init --remote",
    output: [
      "Detected Go project (found go.mod)",
      "Created /user/repository/dagryn.toml",
      "Detected existing GitHub Actions workflows in .github/workflows:",
      "- ci.yml",
      "Setting up remote project...",
      "Creating new project: dagryn-ci",
      "Detected git remote: https://github.com/Mujhtech/dagryn",
      "Creating project on server...",
      "Project linked successfully!",
      "  Name: dev-action-ci",
      "  ID:   7ac8e4ee-4b85-47bf-8e61-f7c682acaf14",
      "You can now use 'dagryn run --sync' without specifying --project.",
      "Next steps:",
      "  1. Review and customize dagryn.toml for your project",
      "  2. Run 'dagryn run <task>' to execute a task",
      "  3. Run 'dagryn graph' to visualize the task DAG",
      "  4. Run 'dagryn run' to execute the default workflow",
    ],
  },
  {
    command: "dagryn run --sync",
    output: [
      "Using linked project: dev-action-ci (7ac8e4ee-4b85-47bf-8e61-f7c682acaf14)",
      "Remote sync enabled (run ID: 69e26297-74dc-4f3d-9d92-a1753d80fba8)",
      "Cloud cache enabled (project: dev-action-ci)",
      "  ↓ local:./plugins/setup-node",
      "    ✓ Installed setup-node 1.0.0",
      "  ↓ local:./plugins/setup-go",
      "    ✓ Installed setup-go 1.0.0",
      "  ↓ github:golangci/golangci-lint@v2.8.0",
      "    ✓ golangci-lint [CACHED]",
      "● web-install",
      "✓ web-install  [CACHE MISS] 2.10s",
      "● web-lint",
      "● web-build",
      "✓ web-lint     [CACHE MISS] 6.52s",
      "✓ web-build    [CACHE MISS] 7.70s",
      "● web-test",
      "● go-build",
      "✓ web-test     [CACHE MISS] 1.82s",
      "✓ go-build     [CACHE MISS] 11.32s",
      "● go-test",
      "● go-lint",
      "● go-fmt",
      "✗ go-fmt       880ms",
      "⊘ go-lint      525ms",
      "⊘ go-test      889ms",
      "✗ 1/8 tasks failed in  68.01s",
    ],
  },
  {
    command: "dagryn run go-test",
    output: [
      "Using linked project: dev-action-ci (7ac8e4ee-4b85-47bf-8e61-f7c682acaf14)",
      "Remote sync enabled (run ID: 69e26297-74dc-4f3d-9d92-a1753d80fba8)",
      "Cloud cache enabled (project: dev-action-ci)",
      "  ↓ local:./plugins/setup-node",
      "    ✓ Installed setup-node 1.0.0",
      "  ↓ local:./plugins/setup-go",
      "    ✓ Installed setup-go 1.0.0",
      "  ↓ github:golangci/golangci-lint@v2.8.0",
      "    ✓ golangci-lint [CACHED]",
      "✓ web-install  [CACHE HIT] 0s",
      "✓ web-build    [CACHE HIT] 0s",
      "✓ go-build     [CACHE HIT] 0s",
      "✓ go-test      [CACHE MISS] 15.93s",
      "✓ 4 tasks completed in  16.41s (3 cached)",
    ],
  },
];

const promptPrefix = "my-repository % ";

export function TerminalPreview() {
  const [history, setHistory] = useState<
    Array<{ type: "prompt" | "output" | "success" | "ready"; text: string }>
  >([]);
  const [runVersion, setRunVersion] = useState(0);
  const [activeCommand, setActiveCommand] = useState("");
  const [isComplete, setIsComplete] = useState(false);
  const terminalBodyRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    setHistory([]);
    setActiveCommand("");
    setIsComplete(false);

    const timers: Array<ReturnType<typeof setTimeout>> = [];
    const typingSpeed = 36;
    const lineDelay = 320;
    let timeOffset = 0;

    script.forEach((step, stepIndex) => {
      for (let i = 1; i <= step.command.length; i += 1) {
        timeOffset += typingSpeed;
        timers.push(
          setTimeout(() => {
            setActiveCommand(step.command.slice(0, i));
          }, timeOffset),
        );
      }

      timeOffset += 120;
      timers.push(
        setTimeout(() => {
          setHistory((previous) => [
            ...previous,
            { type: "prompt", text: `${promptPrefix}${step.command}` },
          ]);
          setActiveCommand("");
        }, timeOffset),
      );

      step.output.forEach((line) => {
        timeOffset += lineDelay;
        timers.push(
          setTimeout(() => {
            setHistory((previous) => [
              ...previous,
              {
                type: line.startsWith("✓") ? "success" : "output",
                text: line,
              },
            ]);
          }, timeOffset),
        );
      });

      if (stepIndex === script.length - 1) {
        timeOffset += lineDelay;
        timers.push(
          setTimeout(() => {
            setHistory((previous) => [
              ...previous,
              { type: "ready", text: "Ready." },
            ]);
          }, timeOffset),
        );
      }

      timeOffset += 280;
    });

    timers.push(setTimeout(() => setIsComplete(true), timeOffset + 80));

    return () => {
      timers.forEach((timer) => clearTimeout(timer));
    };
  }, [runVersion]);

  useEffect(() => {
    const terminalBody = terminalBodyRef.current;
    if (!terminalBody) return;

    terminalBody.scrollTo({
      top: terminalBody.scrollHeight,
      behavior: "smooth",
    });
  }, [history.length, runVersion, isComplete]);

  return (
    <div
      style={{
        maskImage:
          "linear-gradient(black 50%, rgba(0, 0, 0, 0.5) 75%, transparent 100%)",
      }}
    >
      <div className="landing-terminal border border-border/70 bg-card/30 h-full max-h-175 flex flex-col">
        <div className="landing-terminal-head border-b border-border/70 px-4 py-3 md:px-6">
          <div className="flex items-center gap-2">
            <span className="landing-terminal-dot" />
            <span className="landing-terminal-dot" />
            <span className="landing-terminal-dot" />
            <p className="ml-3 text-sm text-muted-foreground md:text-base">
              Terminal
            </p>
          </div>
          <button
            className="landing-rerun-button"
            onClick={() => setRunVersion((version) => version + 1)}
            type="button"
          >
            Re-run
          </button>
        </div>

        <div
          ref={terminalBodyRef}
          className="landing-terminal-body flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-border/70 scrollbar-track-transparent px-4 py-6 font-mono md:px-6 md:py-8"
        >
          <div className="space-y-3 text-sm text-muted-foreground">
            {history.map((line, index) => (
              <p
                key={`${line.text}-${index}`}
                className="landing-terminal-line"
              >
                {line.type === "prompt" && (
                  <>
                    <span className="text-muted-foreground">
                      my-repository{" "}
                    </span>
                    <span className="landing-terminal-accent">%</span>{" "}
                    <span className="text-white">
                      {line.text.replace(promptPrefix, "")}
                    </span>
                  </>
                )}
                {line.type === "success" && (
                  <>
                    <span className="landing-terminal-accent">✓</span>{" "}
                    <span>{line.text.replace(/^✓\s*/, "")}</span>
                  </>
                )}
                {line.type === "output" && line.text}
                {line.type === "ready" && (
                  <span className="landing-terminal-ready">{line.text}</span>
                )}
              </p>
            ))}
          </div>

          <p className="mt-8 text-sm text-muted-foreground">
            my-repository <span className="landing-terminal-accent">%</span>{" "}
            <span>{activeCommand}</span>
            <span className="landing-terminal-cursor">|</span>
          </p>

          {isComplete && (
            <p className="mt-2 text-xs text-muted-foreground">
              Workflow ready for rerun.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
