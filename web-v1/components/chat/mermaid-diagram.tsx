"use client";

import { useEffect, useId, useState } from "react";

export function MermaidDiagram({ chart, isComplete }: { chart: string; isComplete: boolean }) {
  const id = useId().replace(/:/g, "");
  const [svg, setSVG] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function render() {
      if (!isComplete) {
        return;
      }
      try {
        const mermaid = (await import("mermaid")).default;
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: "base",
          themeVariables: {
            primaryColor: "#f4f4f5",
            primaryTextColor: "#18181b",
            primaryBorderColor: "#d4d4d8",
            lineColor: "#71717a",
            secondaryColor: "#fafafa",
            tertiaryColor: "#ffffff",
            pie1: "#60a5fa",
            pie2: "#34d399",
            pie3: "#fbbf24",
            pie4: "#fb7185",
            pie5: "#a78bfa",
            pie6: "#2dd4bf",
            pie7: "#fb923c",
            pie8: "#f472b6",
            pie9: "#818cf8",
            pie10: "#a3e635",
            pie11: "#22d3ee",
            pie12: "#facc15",
          },
        });
        const valid = await mermaid.parse(chart, { suppressErrors: true });
        if (!valid) {
          if (!cancelled) {
            setSVG("");
            setError("Mermaid 图表语法有误，无法渲染。");
          }
          return;
        }
        const result = await mermaid.render(`edith-mermaid-${id}`, chart);
        if (!cancelled) {
          setSVG(result.svg);
          setError("");
        }
      } catch {
        if (!cancelled) {
          setSVG("");
          setError("Mermaid 图表语法有误，无法渲染。");
        }
      }
    }

    render();
    return () => {
      cancelled = true;
    };
  }, [chart, id, isComplete]);

  if (!isComplete) {
    return <p className="py-3 text-sm text-zinc-400">正在生成图表…</p>;
  }

  if (error) {
    return <p className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>;
  }

  if (!svg) {
    return <p className="py-3 text-sm text-zinc-400">正在渲染图表…</p>;
  }

  return <div aria-label="Mermaid diagram" className="my-4 overflow-x-auto" dangerouslySetInnerHTML={{ __html: svg }} />;
}
