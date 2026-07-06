export type AguiSseEvent = Record<string, any>;

function parseSseFrame(frame: string): AguiSseEvent | null {
  const lines = frame.split(/\r?\n/);
  const dataLines: string[] = [];
  for (const line of lines) {
    if (line.startsWith("data:")) {
      dataLines.push(line.slice("data:".length).trimStart());
    }
  }
  const data = dataLines.join("\n").trim();
  if (!data) return null;
  try {
    return JSON.parse(data) as AguiSseEvent;
  } catch {
    return { type: "RAW", raw: data };
  }
}

export async function streamAguiSse(
  url: string,
  payload: unknown,
  options: {
    signal?: AbortSignal;
    onEvent: (evt: AguiSseEvent) => void;
  },
): Promise<void> {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify(payload),
    signal: options.signal,
  });
  if (!response.ok) {
    const text = await response.text().catch(() => "");
    throw new Error(`AG-UI request failed (${response.status}): ${text || response.statusText}`);
  }
  if (!response.body) throw new Error("AG-UI response body is empty.");

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    buffer = buffer.replace(/\r\n/g, "\n");

    while (true) {
      const i = buffer.indexOf("\n\n");
      if (i === -1) break;
      const frame = buffer.slice(0, i);
      buffer = buffer.slice(i + 2);
      const event = parseSseFrame(frame);
      if (event) options.onEvent(event);
    }
  }

  const tail = buffer.trim();
  if (tail) {
    const event = parseSseFrame(tail);
    if (event) options.onEvent(event);
  }
}
