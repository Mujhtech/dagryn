import { createFileRoute, Link } from "@tanstack/react-router";
import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import { Separator } from "~/components/ui/separator";
import { Icons } from "~/components/icons";

const WorkflowConverter = lazy(
  () => import("~/components/workflow-converter"),
);

export const Route = createFileRoute("/")({
  component: IndexPage,
});

function IndexPage() {
  const cloudPlans: PricingPlan[] = [
    {
      name: "Free",
      summary: "For individuals",
      price: "$0.00",
      period: "/monthly",
      ctaLabel: "Get Started",
      ctaVariant: "outline",
      ctaLink: "/login",
      features: [
        { label: "3 projects" },
        { label: "1 team members" },
        { label: "1 GB storage" },
        { label: "5 GB bandwidth" },
        { label: "2 concurrent runs" },
        { label: "7-day cache TTL" },
        { label: "3-day artifact retention" },
        { label: "7-day log retention" },
        { label: "Container execution", included: false },
        { label: "Priority queue", included: false },
        { label: "SSO / SAML", included: false },
        { label: "Audit logs", included: false },
        { label: "AI analysis" },
        { label: "AI suggestions", included: false },
        { label: "10 AI analyses/mo" },
      ],
    },
    {
      name: "Pro",
      summary:
        "For professional developers who need more power and longer retention.",
      price: "$15.00",
      period: "/monthly",
      ctaLabel: "Get Started",
      ctaLink: "/login?plan=pro",
      features: [
        { label: "25 projects" },
        { label: "1 team members" },
        { label: "10 GB storage" },
        { label: "50 GB bandwidth" },
        { label: "10 concurrent runs" },
        { label: "30-day cache TTL" },
        { label: "30-day artifact retention" },
        { label: "30-day log retention" },
        { label: "Container execution" },
        { label: "Priority queue", included: false },
        { label: "SSO / SAML", included: false },
        { label: "Audit logs", included: false },
        { label: "AI analysis" },
        { label: "AI suggestions" },
        { label: "100 AI analyses/mo" },
      ],
    },
    {
      name: "Team",
      summary:
        "For teams that need shared caches, higher limits, and collaboration features.",
      price: "$30.00",
      period: "/monthly",
      perSeat: "per seat",
      ctaLabel: "Get Started",
      ctaVariant: "outline",

      features: [
        { label: "Unlimited projects" },
        { label: "50 team members" },
        { label: "50 GB storage" },
        { label: "250 GB bandwidth" },
        { label: "50 concurrent runs" },
        { label: "90-day cache TTL" },
        { label: "90-day artifact retention" },
        { label: "90-day log retention" },
        { label: "Container execution" },
        { label: "Priority queue" },
        { label: "SSO / SAML", included: false },
        { label: "Audit logs" },
        { label: "AI analysis" },
        { label: "AI suggestions" },
        { label: "500 AI analyses/mo" },
      ],
    },
    {
      name: "Enterprise",
      summary:
        "Custom plans for large organizations with dedicated support and SLA.",
      price: "Custom",
      ctaLabel: "Contact Sales",
      ctaVariant: "outline",
      features: [
        { label: "Unlimited projects" },
        { label: "Unlimited team members" },
        { label: "Unlimited storage" },
        { label: "Unlimited bandwidth" },
        { label: "Unlimited concurrent runs" },
        { label: "Unlimited cache TTL" },
        { label: "Unlimited artifact retention" },
        { label: "Unlimited log retention" },
        { label: "Container execution" },
        { label: "Priority queue" },
        { label: "SSO / SAML" },
        { label: "Audit logs" },
        { label: "AI analysis" },
        { label: "AI suggestions" },
        { label: "Unlimited AI analyses" },
      ],
    },
  ];
  const selfHostPlans: PricingPlan[] = [
    {
      name: "Community",
      summary: "Self-managed deployment for core workflows.",
      price: "$0.00",
      period: "/month",
      ctaLabel: "Free",
      ctaVariant: "outline",
      ctaLink: "https://github.com/mujhtech/dagryn",
      features: [
        { label: "Unlimited Projects" },
        { label: "Team members", included: false },
        { label: "Unlimited Storage" },
        { label: "Unlimited Bandwidth" },
        { label: "3 Concurrent runs" },
        { label: "Unlimited Cache TTL" },
        { label: "Unlimited Artifact retention" },
        { label: "Unlimited Log retention" },
        { label: "Container execution" },
        { label: "Priority queue" },
        { label: "SSO / SAML", included: false },
        { label: "Audit logs", included: false },
        { label: "AI analysis" },
        { label: "AI suggestions" },
        { label: "Unlimited AI analyses/mo" },
      ],
    },
    {
      name: "Enterprise",
      summary: "Commercial self-host with enterprise capabilities.",
      price: "Custom",
      ctaLabel: "Contact Sales",
      ctaLink: "/contact-sales",
      ctaVariant: "outline",
      features: [
        { label: "Projects limit" },
        { label: "Team members limit" },
        { label: "Storage limit" },
        { label: "Bandwidth limit" },
        { label: "Concurrent runs limit" },
        { label: "Cache TTL limit" },
        { label: "Artifact retention limit" },
        { label: "Log retention limit" },
        { label: "Container execution" },
        { label: "Priority queue" },
        { label: "SSO / SAML" },
        { label: "Audit logs" },
        { label: "AI analysis" },
        { label: "AI suggestions" },
        { label: "AI analyses/mo limit" },
      ],
    },
  ];

  const highlights = [
    {
      title: "Deterministic by default",
      description:
        "Run the same workflow locally and in CI with predictable outputs.",
      icon: Icons.Target,
    },
    {
      title: "Local-first speed",
      description:
        "Use smart task caching and parallel execution to reduce wait time.",
      icon: Icons.TrendUp,
    },
    {
      title: "Simple workflow model",
      description:
        "Describe tasks in dagryn.toml and keep orchestration close to code.",
      icon: Icons.Terminal,
    },
  ];

  return (
    <div className="landing-shell relative min-h-screen overflow-hidden px-6 pt-8 md:pt-12">
      <div className="landing-glow landing-glow-a" />
      <div className="landing-glow landing-glow-b" />
      <main className="landing-grid mx-auto max-w-6xl">
        <section className="landing-reveal [--delay:0ms] space-y-8 pt-8 md:pt-14">
          <Badge variant="outline" className="landing-badge w-fit">
            Local-first workflow runtime
          </Badge>
          <div className="space-y-5">
            <h1 className="landing-title max-w-5xl text-5xl font-semibold tracking-tight md:text-7xl">
              Build and ship software with reproducible, graph-based pipelines.
            </h1>
            <p className="max-w-2xl text-base text-muted-foreground md:text-lg">
              Dagryn keeps execution deterministic from local to CI, so teams
              move faster without guessing why builds drift.
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button size="lg" asChild>
              <Link to="/login">Start Building</Link>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <Link to="/projects/new/github">Import from GitHub</Link>
            </Button>
          </div>
        </section>

        <section className="landing-reveal [--delay:70ms]">
          <TerminalPreview />
        </section>

        <section className="landing-reveal grid gap-4 md:grid-cols-3 [--delay:100ms]">
          {highlights.map((item) => (
            <article
              key={item.title}
              className="landing-card border border-border/70 bg-card/35 p-6"
            >
              <div className="space-y-3">
                <item.icon className="h-5 w-5 text-muted-foreground" />
                <h2 className="text-base font-semibold">{item.title}</h2>
              </div>
              <p className="mt-3 text-sm text-muted-foreground">
                {item.description}
              </p>
            </article>
          ))}
        </section>

        <section className="landing-reveal [--delay:140ms]">
          <div className="border border-border/70 bg-card/25 p-6 md:p-8">
            <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
              Pricing
            </p>
            <h2 className="mt-3 text-2xl font-semibold tracking-tight md:text-3xl">
              Choose your deployment model
            </h2>
            <p className="mt-2 max-w-2xl text-sm text-muted-foreground md:text-base">
              Compare available plans across Cloud and Self-host options.
            </p>

            <Tabs defaultValue="cloud" className="mt-6">
              <TabsList className="w-full max-w-sm">
                <TabsTrigger value="cloud">Cloud</TabsTrigger>
                <TabsTrigger value="self-host">Self-host</TabsTrigger>
              </TabsList>

              <TabsContent value="cloud" className="mt-4">
                <PlanGrid plans={cloudPlans} />
              </TabsContent>
              <TabsContent value="self-host" className="mt-4">
                <PlanGrid plans={selfHostPlans} />
              </TabsContent>
            </Tabs>
          </div>
        </section>

        <section className="landing-reveal [--delay:160ms]">
          <div className="border border-border/70 bg-card/25 p-6 md:p-8">
            <div className="mb-6">
              <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
                Workflow
              </p>
              <h2 className="mt-3 text-2xl font-semibold tracking-tight md:text-3xl">
                Convert GitHub Actions workflows
              </h2>
              <p className="mt-2 max-w-2xl text-sm text-muted-foreground md:text-base">
                Convert your existing GitHub Actions workflows to Dagryn tasks.
              </p>
            </div>
            <Suspense
              fallback={
                <div className="flex items-center justify-center h-64">
                  <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
              }
            >
              <WorkflowConverter />
            </Suspense>
          </div>
        </section>

        <section className="landing-reveal [--delay:180ms]">
          <div className="border border-border/70 bg-card/25 p-6 md:p-8">
            <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
              Typical flow
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-3">
              <StepTile
                step="01"
                title="Initialize"
                command="dagryn init"
                description="Generate dagryn.toml and starter tasks in your repo."
              />
              <StepTile
                step="02"
                title="Run"
                command="dagryn run"
                description="Execute your default workflow with dependency ordering."
              />
              <StepTile
                step="03"
                title="Scale"
                command="dagryn run --sync"
                description="Reuse the same task model for remote/CI execution."
              />
            </div>
          </div>
        </section>
      </main>

      <footer className="landing-reveal landing-footer mx-auto mt-16 max-w-6xl [--delay:260ms]">
        <FooterWordmark />
      </footer>
    </div>
  );
}

type PricingPlan = {
  name: string;
  summary: string;
  price: string;
  period?: string;
  perSeat?: string;
  ctaLabel: string;
  ctaVariant?: "default" | "outline";
  ctaDisabled?: boolean;
  ctaLink?: string;
  isCurrent?: boolean;
  features: Array<{ label: string; included?: boolean }>;
};

function PlanGrid({ plans }: { plans: PricingPlan[] }) {
  return (
    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
      {plans.map((plan) => (
        <Card
          key={plan.name}
          className="landing-card border-border/70 bg-background/35"
        >
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{plan.name}</CardTitle>
              {plan.isCurrent && <Badge>Current</Badge>}
            </div>
            <CardDescription>{plan.summary}</CardDescription>
          </CardHeader>

          <CardContent className="space-y-4 px-3 flex-1">
            <div>
              <span className="text-3xl font-bold">{plan.price}</span>
              {plan.period && (
                <span className="text-muted-foreground">{plan.period}</span>
              )}
              {plan.perSeat && (
                <span className="text-sm text-muted-foreground ml-1">
                  {plan.perSeat}
                </span>
              )}
            </div>

            <Button
              className="w-full"
              variant={plan.ctaVariant ?? "default"}
              disabled={plan.ctaDisabled ?? false}
              asChild
            >
              <Link to={plan.ctaLink}> {plan.ctaLabel}</Link>
            </Button>

            <Separator />

            <ul className="space-y-2 text-sm">
              {plan.features.map((feature) => (
                <PlanFeature
                  key={feature.label}
                  label={feature.label}
                  included={feature.included ?? true}
                />
              ))}
            </ul>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function PlanFeature({
  label,
  included = true,
}: {
  label: string;
  included?: boolean;
}) {
  return (
    <li className="flex items-center gap-2">
      {included ? (
        <Icons.Check className="h-4 w-4 text-primary shrink-0" />
      ) : (
        <Icons.Minus className="h-4 w-4 text-muted-foreground/50 shrink-0" />
      )}
      <span className={included ? "" : "text-muted-foreground/50"}>
        {label}
      </span>
    </li>
  );
}

function FooterWordmark() {
  return (
    <div className="landing-wordmark-wrap" aria-label="DAGRYN">
      <svg
        className="landing-wordmark-svg"
        viewBox="0 0 2600 300"
        preserveAspectRatio="xMinYMid slice"
        role="img"
      >
        <defs>
          <linearGradient
            id="dagrynWordmarkFill"
            x1="0%"
            x2="100%"
            y1="0%"
            y2="0%"
          >
            <stop offset="0%" stopColor="rgba(214,214,217,0.52)" />
            <stop offset="50%" stopColor="rgba(248,248,250,0.68)" />
            <stop offset="100%" stopColor="rgba(214,214,217,0.52)" />
            <animateTransform
              attributeName="gradientTransform"
              type="translate"
              from="-0.3 0"
              to="0.3 0"
              dur="9s"
              repeatCount="indefinite"
            />
          </linearGradient>
        </defs>
        <text
          className="landing-wordmark-svg-text"
          x="0"
          y="228"
          fill="url(#dagrynWordmarkFill)"
        >
          DAGRYN
        </text>
      </svg>
    </div>
  );
}

function TerminalPreview() {
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

function StepTile({
  step,
  title,
  command,
  description,
}: {
  step: string;
  title: string;
  command: string;
  description: string;
}) {
  return (
    <div className="border border-border/70 bg-background/35 p-4">
      <p className="text-xs uppercase tracking-[0.12em] text-muted-foreground">
        {step}
      </p>
      <p className="mt-2 text-sm font-semibold">{title}</p>
      <code className="mt-3 block bg-gray-900 px-2 py-1 text-xs">
        {command}
      </code>
      <p className="mt-2 text-xs text-muted-foreground">{description}</p>
    </div>
  );
}
