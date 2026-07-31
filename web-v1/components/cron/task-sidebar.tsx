"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { AccountMenu } from "@/components/account-menu";
import { AppSidebar } from "@/components/app-sidebar";
import { SettingsDialog } from "@/components/settings/settings-dialog";
import type { ConversationListResponse } from "@/lib/chat/type";

import { ConversationList } from "@/components/chat/conversation-list";

export function TaskSidebar() {
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
          activePage="tasks"
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
