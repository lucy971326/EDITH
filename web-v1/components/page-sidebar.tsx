"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { AccountMenu } from "@/components/account-menu";
import { AppSidebar } from "@/components/app-sidebar";
import { ConversationList } from "@/components/chat/conversation-list";
import { SettingsDialog } from "@/components/settings/settings-dialog";
import type { ConversationListResponse } from "@/lib/chat/type";

type PageSidebarProps = {
  activePage: "tasks" | "extensions";
};

// PageSidebar 为非聊天页面提供统一的会话导航、页面入口和设置入口。
export function PageSidebar({ activePage }: PageSidebarProps) {
  const router = useRouter();
  const [isLoading, setIsLoading] = useState(true);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [sessions, setSessions] = useState<{ id: string; title: string }[]>([]);

  useEffect(() => {
    async function loadConversations() {
      try {
        const response = await fetch("/api/conversations");
        if (!response.ok) return;
        const data = await response.json() as ConversationListResponse;
        setSessions(data.conversations);
      } finally {
        setIsLoading(false);
      }
    }
    void loadConversations();
  }, []);

  return (
    <>
      <AppSidebar footer={<AccountMenu onOpenSettings={() => setSettingsOpen(true)} />}>
        <ConversationList
          activePage={activePage}
          activeSessionID=""
          isLoading={isLoading}
          onCreate={() => router.push("/chat")}
          onSelect={(sessionID) => router.push(`/chat?sessionId=${encodeURIComponent(sessionID)}`)}
          sessions={sessions}
        />
      </AppSidebar>
      <SettingsDialog open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </>
  );
}
