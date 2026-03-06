import { motion } from "motion/react";

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

const steps = [
  {
    step: "01",
    title: "Initialize",
    command: "dagryn init",
    description: "Generate dagryn.toml and starter tasks in your repo.",
  },
  {
    step: "02",
    title: "Run",
    command: "dagryn run",
    description: "Execute your default workflow with dependency ordering.",
  },
  {
    step: "03",
    title: "Scale",
    command: "dagryn run --sync",
    description: "Reuse the same task model for remote/CI execution.",
  },
];

export function GettingStarted() {
  return (
    <div className="border border-border/70 bg-card/25 p-6 md:p-8">
      <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
        Typical flow
      </p>
      <motion.div
        className="mt-5 grid gap-3 md:grid-cols-3"
        variants={staggerContainer}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, amount: 0.2 }}
      >
        {steps.map((item) => (
          <motion.div key={item.step} variants={cardVariant}>
            <div className="border border-border/70 bg-background/35 p-4">
              <p className="text-xs uppercase tracking-[0.12em] text-muted-foreground">
                {item.step}
              </p>
              <p className="mt-2 text-sm font-semibold">{item.title}</p>
              <code className="mt-3 block bg-gray-900 px-2 py-1 text-xs">
                {item.command}
              </code>
              <p className="mt-2 text-xs text-muted-foreground">
                {item.description}
              </p>
            </div>
          </motion.div>
        ))}
      </motion.div>
    </div>
  );
}
