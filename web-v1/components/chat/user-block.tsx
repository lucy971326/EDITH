/* eslint-disable @next/next/no-img-element -- private BFF image URLs cannot use the Next optimizer. */

import { memo } from "react";

import type { UserBlock } from "@/lib/chat/type";

export const UserBlockView = memo(function UserBlockView({ block }: { block: UserBlock }) {
  return (
    <article className="ml-auto max-w-[78%] space-y-2 rounded-2xl rounded-tr-md border border-accent/10 bg-accent-soft px-4 py-3 text-sm leading-6 text-ink shadow-sm shadow-accent/5">
      {block.images.length > 0 && <div className="flex flex-wrap gap-2">
        {block.images.map((image) => <img alt="用户上传图片" className="max-h-64 rounded-xl border border-border object-contain" key={image.id} src={`/api/images/${encodeURIComponent(image.id)}`} />)}
      </div>}
      {block.content && <p>{block.content}</p>}
    </article>
  );
});
