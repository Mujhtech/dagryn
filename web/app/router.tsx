import { createRouter as createTanStackRouter } from "@tanstack/react-router";
import { routeTree } from "./routeTree.gen";

export function createRouter() {
  const router = createTanStackRouter({
    routeTree,
    defaultPreload: "intent",
    defaultStaleTime: 5000,
    scrollRestoration: true,
    defaultViewTransition: true,
    // OR
    // defaultViewTransition: {
    //   types: ({ fromLocation, toLocation }) => {
    //     let direction = 'none'

    //     if (fromLocation) {
    //       const fromIndex = fromLocation.state.__TSR_index
    //       const toIndex = toLocation.state.__TSR_index

    //       direction = fromIndex > toIndex ? 'right' : 'left'
    //     }

    //     return [`slide-${direction}`]
    //   },
    // },
  });

  return router;
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createRouter>;
  }
}
