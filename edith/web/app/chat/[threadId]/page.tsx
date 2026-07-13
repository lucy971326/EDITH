import { auth } from "@clerk/nextjs/server";

// 阶段 1 占位：URL 即 threadId / SessionID
// 真实 CopilotKit 工作台留到阶段 2
export default async function ChatPage({
  params,
}: {
  params: Promise<{ threadId: string }>;
}) {
  const { threadId } = await params;
  const { userId } = await auth();

  // 二次保险：proxy 已放过；这里再确认一次，没登录就跳 sign-in
  if (!userId) {
    const { redirect } = await import("next/navigation");
    redirect("/sign-in");
  }

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4 p-8">
      <h1 className="text-2xl font-semibold">EDITH 工作台</h1>
      <div className="rounded-md border border-zinc-300 bg-zinc-50 px-6 py-4 font-mono text-sm dark:border-zinc-700 dark:bg-zinc-900">
        threadId = {threadId}
      </div>
      <p className="text-sm text-zinc-500">
        阶段 1 占位。CopilotKit 流式聊天界面将在阶段 2 接入。
      </p>
    </main>
  );
}