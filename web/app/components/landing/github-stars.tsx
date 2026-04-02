import { useEffect, useRef, useState } from "react";
import { useMotionValue, useSpring, useTransform, motion } from "motion/react";
import { Icons } from "~/components/icons";
import { Button } from "../ui/button";

const REPO = "mujhtech/dagryn";

function AnimatedNumber({ value }: { value: number }) {
  const motionValue = useMotionValue(0);
  const spring = useSpring(motionValue, { stiffness: 80, damping: 20 });
  const display = useTransform(spring, (v) =>
    v >= 1000 ? `${(v / 1000).toFixed(1)}k` : Math.round(v).toLocaleString(),
  );
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    motionValue.set(value);
  }, [value, motionValue]);

  useEffect(() => {
    const unsubscribe = display.on("change", (v) => {
      if (ref.current) ref.current.textContent = v;
    });
    return unsubscribe;
  }, [display]);

  return <span ref={ref}>0</span>;
}

export function GitHubStars() {
  const [stars, setStars] = useState<number | null>(null);

  useEffect(() => {
    fetch(`https://api.github.com/repos/${REPO}`)
      .then((res) => res.json())
      .then((data) => {
        if (typeof data.stargazers_count === "number") {
          setStars(data.stargazers_count);
        }
      })
      .catch(() => {});
  }, []);

  return (
    <Button size="lg" variant="outline" className="bg-card/35" asChild>
      <a
        href={`https://github.com/${REPO}`}
        target="_blank"
        rel="noopener noreferrer"
      >
        <Icons.Github className="h-4 w-4" />
        <span>Star on GitHub</span>
        {stars !== null && (
          <motion.span
            className="ml-1 inline-flex items-center gap-1 bg-muted px-2 py-0.5 text-xs font-semibold tabular-nums"
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ type: "spring", stiffness: 100, damping: 15 }}
          >
            <AnimatedNumber value={stars} />
          </motion.span>
        )}
      </a>
    </Button>
  );
}
