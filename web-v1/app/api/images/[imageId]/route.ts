import { auth } from "@clerk/nextjs/server";

const runtimeURL = process.env.EDITH_RUNTIME_URL ?? "http://127.0.0.1:8080";

// This stable EDITH endpoint is what the browser stores in img.src. Go signs
// a fresh private COS URL every time it is requested.
export async function GET(
  _request: Request,
  context: RouteContext<"/api/images/[imageId]">,
) {
  const { userId } = await auth();
  if (!userId) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { imageId } = await context.params;
  const url = new URL(
    `/internal/images/${encodeURIComponent(imageId)}`,
    runtimeURL,
  );
  url.searchParams.set("userId", userId);
  try {
    const response = await fetch(url, {
      cache: "no-store",
      redirect: "manual",
    });
    const location = response.headers.get("Location");
    if (response.status === 302 && location) {
      return new Response(null, {
        status: 302,
        headers: { Location: location, "Cache-Control": "no-store" },
      });
    }
    return new Response(response.body, {
      status: response.status,
      headers: {
        "Content-Type":
          response.headers.get("Content-Type") ?? "application/json",
        "Cache-Control": "no-store",
      },
    });
  } catch {
    return Response.json(
      { error: "EDITH runtime is unavailable" },
      { status: 502 },
    );
  }
}
