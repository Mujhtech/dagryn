import { useState, useCallback } from "react";
import { motion, useAnimation } from "motion/react";

const LETTERS = ["D", "A", "G", "R", "Y", "N"];

export function FooterWordmark() {
  const [bouncing, setBouncing] = useState(false);
  const controls = LETTERS.map(() => useAnimation());

  const triggerEasterEgg = useCallback(async () => {
    if (bouncing) return;
    setBouncing(true);

    // Staggered bounce wave — each letter jumps, rotates, then settles
    await Promise.all(
      controls.map((ctrl, i) =>
        ctrl.start({
          y: [0, -32, 0, -12, 0],
          rotate: [0, i % 2 === 0 ? -8 : 8, 0],
          scale: [1, 1.15, 1, 1.05, 1],
          transition: {
            duration: 0.7,
            delay: i * 0.07,
            ease: [0.22, 1, 0.36, 1],
          },
        }),
      ),
    );

    setBouncing(false);
  }, [bouncing, controls]);

  return (
    <div
      className="landing-wordmark-wrap cursor-pointer"
      aria-label="DAGRYN"
      onClick={triggerEasterEgg}
      role="img"
    >
      <div className="landing-wordmark-letters">
        {LETTERS.map((letter, i) => (
          <motion.span
            key={i}
            className="landing-wordmark-letter"
            animate={controls[i]}
          >
            {letter}
          </motion.span>
        ))}
      </div>
    </div>
  );
}
