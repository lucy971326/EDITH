"use client";

import { useState } from "react";
import { CopilotChat, useDefaultRenderTool } from "@copilotkit/react-core/v2";
import { SessionSidebar } from "./session-sidebar";

function ToolRenderer() {
  // 一行解决所有未注册工具的卡片渲染
  useDefaultRenderTool();
  return null;
}

export default function Home() {
  // 当前选中的 session ID（= AG-UI 协议的 threadId）
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  // 让侧边栏主动重新拉列表的"计数器" — 新建/发消息后 +1
  const [refreshKey, setRefreshKey] = useState(0);

  return (
    <div style={{ display: "flex", height: "100vh" }}>
      <SessionSidebar
        activeId={activeSessionId}
        onSelect={setActiveSessionId}
        onNewChat={() => {
          setActiveSessionId(null); // 切回 null 让 CopilotChat 自动生成新 threadId
          setRefreshKey((k) => k + 1); // 立即刷新列表
        }}
        refreshKey={refreshKey}
      />

      <main style={{ flex: 1 }}>
        <ToolRenderer />
        <CopilotChat
          agentId="agui-demo"
          key={activeSessionId ?? "default"}
          threadId={activeSessionId ?? undefined}
          labels={{
            chatInputPlaceholder: "问小天任何事…",
            welcomeMessageText:
              "你好！我是小天，能查天气、记笔记、读文件、查 GitHub。",
          }}
        />
      </main>
    </div>
  );
}