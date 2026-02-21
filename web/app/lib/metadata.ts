export async function generateMetadata({
  description,
  title,
}: {
  description?: string;
  title: string;
}) {
  return {
    meta: [
      { title },
      {
        name: "description",
        content:
          description ||
          "Local-first, self-hosted developer workflow orchestrator",
      },
    ],
  };
}
