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
        const dark = document.documentElement.classList.contains("dark");
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: "base",
          themeVariables: {
            primaryColor: dark ? "#20242c" : "#f1f2f4",
            primaryTextColor: dark ? "#f0f2f5" : "#20232a",
            primaryBorderColor: dark ? "#414958" : "#cfd3dc",
            lineColor: dark ? "#aeb5c0" : "#686f7c",
            secondaryColor: dark ? "#171a20" : "#ffffff",
            tertiaryColor: dark ? "#101216" : "#f7f7f8",
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
    return <p className="py-3 text-sm text-faint">正在生成图表…</p>;
  }

  if (error) {
    return <p className="rounded-lg border border-danger/25 bg-danger-soft px-3 py-2 text-sm text-danger">{error}</p>;
  }

  if (!svg) {
    return <p className="py-3 text-sm text-faint">正在渲染图表…</p>;
  }

  return <div aria-label="Mermaid diagram" className="my-4 overflow-x-auto" dangerouslySetInnerHTML={{ __html: svg }} />;
}
