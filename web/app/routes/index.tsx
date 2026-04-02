import { createFileRoute, Link } from "@tanstack/react-router";
import { lazy, Suspense } from "react";
import { motion } from "motion/react";
import { TerminalPreview } from "~/components/landing/terminal-preview";
import { PricingSection } from "~/components/landing/pricing-section";
import { GettingStarted } from "~/components/landing/getting-started";
import { FooterWordmark } from "~/components/landing/footer-wordmark";
import { GitHubStars } from "~/components/landing/github-stars";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import { Icons } from "~/components/icons";
import { generateMetadata } from "~/lib/metadata";

const WorkflowConverter = lazy(() => import("~/components/workflow-converter"));
const ShowcaseDag = lazy(() => import("~/components/landing/showcase-dag"));
const ShowcaseWaterfall = lazy(
  () => import("~/components/landing/showcase-waterfall"),
);
const ShowcaseAnalytics = lazy(
  () => import("~/components/landing/showcase-analytics"),
);

export const Route = createFileRoute("/")({
  component: IndexPage,
  head: ({}) => {
    return generateMetadata({
      title: "Local-first workflow runtime",
    });
  },
});

const fadeUp = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { type: "spring" as const, stiffness: 100, damping: 20 },
  },
};

const staggerContainer = {
  hidden: {},
  visible: {
    transition: { staggerChildren: 0.1 },
  },
};

const cardVariant = {
  hidden: { opacity: 0, y: 24, scale: 0.97 },
  visible: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: { type: "spring" as const, stiffness: 100, damping: 18 },
  },
};

const scaleIn = {
  hidden: { opacity: 0, scale: 0.95 },
  visible: {
    opacity: 1,
    scale: 1,
    transition: { type: "spring" as const, stiffness: 80, damping: 20 },
  },
};

// ── Page ────────────────────────────────────────────────────────────

function IndexPage() {
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
    <div className="landing-shell relative min-h-screen px-2 md:px-6 pt-8 md:pt-12">
      <div className="landing-glow landing-glow-a" />
      <div className="landing-glow landing-glow-b" />
      <main className="landing-grid mx-auto w-full max-w-6xl">
        {/* Hero */}
        <motion.section
          className="space-y-8 pt-8 md:pt-14"
          variants={staggerContainer}
          initial="hidden"
          animate="visible"
        >
          <motion.div variants={fadeUp}>
            <Badge variant="outline" className="landing-badge w-fit">
              Local-first workflow runtime
            </Badge>
          </motion.div>
          <div className="space-y-5">
            <motion.h1
              className="landing-title max-w-5xl text-5xl font-semibold tracking-tight md:text-7xl"
              variants={fadeUp}
            >
              Build and ship software with reproducible, graph-based pipelines.
            </motion.h1>
            <motion.p
              className="max-w-2xl text-base text-muted-foreground md:text-lg"
              variants={fadeUp}
            >
              Dagryn keeps execution deterministic from local to CI, so teams
              move faster without guessing why builds drift.
            </motion.p>
          </div>
          <motion.div
            className="flex flex-wrap items-center gap-3"
            variants={fadeUp}
          >
            <Button size="lg" asChild>
              <Link to="/login">Start Building</Link>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <Link to="/projects/new/github">Import from GitHub</Link>
            </Button>
            <GitHubStars />
          </motion.div>
        </motion.section>

        {/* Terminal Preview */}
        <motion.section
          variants={scaleIn}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.1 }}
        >
          <TerminalPreview />
        </motion.section>

        {/* Feature highlights */}
        <motion.section
          className="grid gap-4 md:grid-cols-3"
          variants={staggerContainer}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.2 }}
        >
          {highlights.map((item) => (
            <motion.article
              key={item.title}
              className="landing-card border border-border/70 bg-card/35 p-6"
              variants={cardVariant}
            >
              <div className="space-y-3">
                <item.icon className="h-5 w-5 text-muted-foreground" />
                <h2 className="text-base font-semibold">{item.title}</h2>
              </div>
              <p className="mt-3 text-sm text-muted-foreground">
                {item.description}
              </p>
            </motion.article>
          ))}
        </motion.section>

        {/* Showcase: DAG */}
        <section>
          <Suspense
            fallback={
              <div className="flex items-center justify-center h-64">
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            }
          >
            <ShowcaseDag />
          </Suspense>
        </section>

        {/* Showcase: Waterfall */}
        <section>
          <Suspense
            fallback={
              <div className="flex items-center justify-center h-64">
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            }
          >
            <ShowcaseWaterfall />
          </Suspense>
        </section>

        {/* Showcase: Analytics */}
        <section>
          <Suspense
            fallback={
              <div className="flex items-center justify-center h-64">
                <Icons.Loader className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            }
          >
            <ShowcaseAnalytics />
          </Suspense>
        </section>

        {/* Pricing */}
        <motion.section
          variants={fadeUp}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.1 }}
        >
          <PricingSection />
        </motion.section>

        {/* Workflow Converter */}
        <motion.section
          variants={fadeUp}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.1 }}
        >
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
        </motion.section>

        {/* Getting Started Steps */}
        <motion.section
          variants={fadeUp}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.15 }}
        >
          <GettingStarted />
        </motion.section>
      </main>

      <motion.footer
        className="landing-footer mx-auto mt-16 max-w-6xl relative"
        variants={scaleIn}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, amount: 0.3 }}
      >
        <FooterWordmark />
      </motion.footer>
    </div>
  );
}
