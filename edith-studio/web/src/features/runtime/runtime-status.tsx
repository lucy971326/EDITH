export function RuntimeStatus({ isRunning, requestID }: { isRunning: boolean; requestID: string | null }) {
  return <div className="run-status"><i className={`status-dot ${isRunning ? "running" : ""}`} /><span>{isRunning ? "Agent 运行中" : "Agent 就绪"}</span>{requestID && <span className="muted">· {requestID.slice(0, 8)}</span>}</div>;
}
