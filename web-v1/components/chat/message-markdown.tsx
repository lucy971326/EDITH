import ReactMarkdown from "react-markdown";

export function MessageMarkdown({ content }: { content: string }) {
  return (
    <ReactMarkdown
      components={{
        h1: ({ children }) => <h1 className="mt-6 text-xl font-semibold text-zinc-900">{children}</h1>,
        h2: ({ children }) => <h2 className="mt-5 text-lg font-semibold text-zinc-900">{children}</h2>,
        p: ({ children }) => <p className="whitespace-pre-wrap">{children}</p>,
        ul: ({ children }) => <ul className="list-disc space-y-1 pl-5">{children}</ul>,
        ol: ({ children }) => <ol className="list-decimal space-y-1 pl-5">{children}</ol>,
        blockquote: ({ children }) => <blockquote className="border-l-2 border-zinc-300 pl-3 text-zinc-500">{children}</blockquote>,
        a: ({ children, href }) => <a className="text-zinc-900 underline underline-offset-2" href={href}>{children}</a>,
        code: ({ children }) => <code className="rounded bg-zinc-100 px-1 py-0.5 font-mono text-[0.85em]">{children}</code>,
        pre: ({ children }) => <pre className="overflow-x-auto rounded-lg bg-zinc-800 p-3 text-zinc-100">{children}</pre>,
      }}
    >
      {content}
    </ReactMarkdown>
  );
}
