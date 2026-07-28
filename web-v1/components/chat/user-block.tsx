import type { UserBlock } from "@/lib/chat/type";

export function UserBlockView({ block }: { block: UserBlock }) {
  return (
    <article className="ml-auto max-w-[80%] rounded-xl bg-zinc-200 px-4 py-3 text-sm leading-6 text-zinc-800">
      {block.content}
    </article>
  );
}
