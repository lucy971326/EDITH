import type { Timeline } from "./type";

export const demoTimeline: Timeline = {
  blocks: [
    {
      type: "user",
      id: "user-1",
      content: "你好，帮我看看现在几点了。",
      createdAt: "2026-07-28T09:31:00+08:00",
    },
    {
      type: "assistant",
      id: "assistant-1",
      createdAt: "2026-07-28T09:31:02+08:00",
      blocks: [
        {
          type: "reasoning",
          id: "reasoning-1",
          content: "用户询问当前时间，需要调用 `get_current_time` 工具获取准确结果。",
        },
        {
          type: "text",
          id: "text-1",
          content: "我帮你看一下。",
        },
        {
          type: "tool",
          id: "tool-1",
          toolName: "get_current_time",
          arguments: '{"timezone":"Asia/Shanghai"}',
          status: "completed",
          result: '{"time":"2026-07-28 09:31:15","timezone":"Asia/Shanghai"}',
        },
        {
          type: "reasoning",
          id: "reasoning-2",
          content: "工具已返回北京时间，直接简洁回答即可。",
        },
        {
          type: "text",
          id: "text-2",
          content: "现在是 **北京时间 09:31**。\n\n如果你愿意，我也可以帮你整理今天的待办：\n\n- 明确聊天页面结构\n- 接入 BFF\n- 再连接 Go 的 SSE 接口",
        },
      ],
    },
  ],
};
