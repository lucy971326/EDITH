import { Children, isValidElement, memo, type ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { MermaidDiagram } from "./mermaid-diagram";

function isMermaidCodeBlock(children: ReactNode) {
  const child = Children.toArray(children)[0];
  return isValidElement<{ className?: string }>(child) && child.props.className === "language-mermaid";
}

type MarkdownNode = {
  position?: {
    start?: { offset?: number };
    end?: { offset?: number };
  };
};

function isClosedMermaidCodeBlock(content: string, node?: MarkdownNode) {
  const start = node?.position?.start?.offset;
  const end = node?.position?.end?.offset;
  if (start === undefined || end === undefined) {
    return false;
  }
  return content.slice(start, end).trimEnd().endsWith("```");
}

export const MessageMarkdown = memo(function MessageMarkdown({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        h1: ({ children }) => <h1 className="mt-6 text-xl font-semibold text-ink">{children}</h1>,
        h2: ({ children }) => <h2 className="mt-5 text-lg font-semibold text-ink">{children}</h2>,
        p: ({ children }) => <p className="whitespace-pre-wrap">{children}</p>,
        ul: ({ children }) => <ul className="list-disc space-y-1 pl-5">{children}</ul>,
        ol: ({ children }) => <ol className="list-decimal space-y-1 pl-5">{children}</ol>,
        blockquote: ({ children }) => <blockquote className="border-l-2 border-border-strong pl-3 text-muted">{children}</blockquote>,
        a: ({ children, href }) => <a className="text-accent underline underline-offset-2" href={href}>{children}</a>,
        table: ({ children }) => (
          <div className="my-4 overflow-x-auto rounded-lg border border-border">
            <table className="w-full border-collapse text-left text-sm">{children}</table>
          </div>
        ),
        th: ({ children }) => <th className="border-b border-border bg-surface-subtle px-3 py-2 font-medium text-ink">{children}</th>,
        td: ({ children }) => <td className="border-b border-border px-3 py-2 align-top last:border-b-0">{children}</td>,
        code: ({ children, className, node }) => {
          if (className === "language-mermaid") {
            return <MermaidDiagram chart={String(children).replace(/\n$/, "")} isComplete={isClosedMermaidCodeBlock(content, node)} />;
          }
          return <code className="rounded bg-surface-subtle px-1 py-0.5 font-mono text-[0.85em] text-ink">{children}</code>;
        },
        pre: ({ children }) => (
          isMermaidCodeBlock(children) ? children : (
            <pre className="overflow-x-auto rounded-lg bg-[#171a20] p-3 font-mono text-sm leading-6 text-[#f0f2f5] [&>code]:bg-transparent [&>code]:p-0 [&>code]:text-inherit">
              {children}
            </pre>
          )
        ),
      }}
    >
      {content}
    </ReactMarkdown>
  );
});
