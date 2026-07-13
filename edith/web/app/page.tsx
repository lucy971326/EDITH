import Link from "next/link";
import { auth } from "@clerk/nextjs/server";
import { SignInButton, SignUpButton } from "@clerk/nextjs";

export default async function HomePage() {
  const { userId } = await auth();

  // 生成新工作台的 threadId（前端路由即 SessionID）
  const newThreadId = crypto.randomUUID();

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 p-8">
      {userId ? (
        <>
          <h1 className="text-2xl font-semibold">EDITH 工作台</h1>
          <Link
            href={`/chat/${newThreadId}`}
            className="rounded-md bg-foreground px-6 py-3 text-background hover:opacity-90"
          >
            新建工作台
          </Link>
        </>
      ) : (
        <div className="flex flex-col items-center gap-4">
          <h1 className="text-2xl font-semibold">EDITH 工作台</h1>
          <p className="text-zinc-600 dark:text-zinc-400">请先登录</p>
          <div className="flex gap-3">
            <SignInButton />
            <SignUpButton />
          </div>
        </div>
      )}
    </main>
  );
}