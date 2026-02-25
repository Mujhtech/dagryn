import type { ReactNode } from "react";
import { motion } from "motion/react";
import { cn } from "~/lib/utils";

type GradientPreset = "bottom" | "bottom-heavy" | "radial" | "edges";

const GRADIENT_MASKS: Record<GradientPreset, string> = {
  bottom:
    "linear-gradient(to bottom, black 60%, rgba(0,0,0,0.4) 85%, transparent 100%)",
  "bottom-heavy":
    "linear-gradient(to bottom, black 40%, rgba(0,0,0,0.5) 70%, transparent 100%)",
  radial:
    "radial-gradient(ellipse 80% 80% at center 40%, black 50%, transparent 100%)",
  edges:
    "linear-gradient(to bottom, transparent 0%, black 8%, black 85%, transparent 100%)",
};

const containerVariants = {
  hidden: {},
  visible: {
    transition: { staggerChildren: 0.12 },
  },
} as const;

const textVariants = {
  hidden: { opacity: 0, y: 16 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { type: "spring" as const, stiffness: 120, damping: 20 },
  },
};

const previewVariants = {
  hidden: { opacity: 0, scale: 0.97, y: 24 },
  visible: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: {
      type: "spring" as const,
      stiffness: 80,
      damping: 22,
      delay: 0.3,
    },
  },
};

interface ShowcaseFrameProps {
  label: string;
  title: string;
  description: string;
  gradient?: GradientPreset;
  maxHeight?: string;
  perspective?: boolean;
  className?: string;
  children: ReactNode;
}

export function ShowcaseFrame({
  label,
  title,
  description,
  gradient = "bottom",
  maxHeight = "36rem",
  perspective = false,
  className,
  children,
}: ShowcaseFrameProps) {
  return (
    <motion.div
      className={cn("border border-border/70 bg-card/25 p-6 md:p-8", className)}
      variants={containerVariants}
      initial="hidden"
      whileInView="visible"
      viewport={{ once: true, amount: 0.15 }}
    >
      <motion.p
        className="text-xs uppercase tracking-[0.16em] text-muted-foreground"
        variants={textVariants}
      >
        {label}
      </motion.p>
      <motion.h2
        className="mt-3 text-2xl font-semibold tracking-tight md:text-3xl"
        variants={textVariants}
      >
        {title}
      </motion.h2>
      <motion.p
        className="mt-2 max-w-2xl text-sm text-muted-foreground md:text-base"
        variants={textVariants}
      >
        {description}
      </motion.p>

      <motion.div
        className={cn("mt-6 overflow-hidden", perspective && "landing-showcase-perspective")}
        style={{
          maxHeight,
          maskImage: GRADIENT_MASKS[gradient],
          WebkitMaskImage: GRADIENT_MASKS[gradient],
        }}
        variants={previewVariants}
      >
        <div
          className="pointer-events-none select-none"
          aria-hidden="true"
        >
          {children}
        </div>
      </motion.div>
    </motion.div>
  );
}
