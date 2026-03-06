import { Link } from "@tanstack/react-router";
import { motion } from "motion/react";
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

// ── Motion variants ─────────────────────────────────────────────────

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

// ── Plan data ───────────────────────────────────────────────────────

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
  recommended?: boolean;
  features: Array<{ label: string; included?: boolean }>;
};

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
    recommended: true,
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

// ── Components ──────────────────────────────────────────────────────

export function PricingSection() {
  return (
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
  );
}

function PlanGrid({ plans }: { plans: PricingPlan[] }) {
  return (
    <motion.div
      className="grid gap-6 md:grid-cols-2 lg:grid-cols-4"
      variants={staggerContainer}
      initial="hidden"
      whileInView="visible"
      viewport={{ once: true, amount: 0.1 }}
    >
      {plans.map((plan) => (
        <motion.div key={plan.name} variants={cardVariant}>
          {plan.recommended ? (
            <AnimatedBorderCard plan={plan} />
          ) : (
            <PlanCard plan={plan} />
          )}
        </motion.div>
      ))}
    </motion.div>
  );
}

function PlanCard({ plan }: { plan: PricingPlan }) {
  return (
    <Card className="landing-card border-border/70 bg-background/35 h-full">
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
  );
}

function AnimatedBorderCard({ plan }: { plan: PricingPlan }) {
  return (
    <div className="relative rounded-none p-px h-full overflow-hidden">
      <motion.div
        className="absolute inset-[-50%] z-0"
        style={{
          background:
            "conic-gradient(from 0deg, transparent 0%, var(--color-primary) 10%, transparent 20%, transparent 50%, var(--color-primary) 60%, transparent 70%)",
        }}
        animate={{ rotate: 360 }}
        transition={{
          repeat: Infinity,
          duration: 4,
          ease: "linear",
        }}
      />
      <div className="relative z-10 h-full rounded-none bg-background">
        <Card className="border-0 bg-background/35 h-full">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{plan.name}</CardTitle>
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
      </div>
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
