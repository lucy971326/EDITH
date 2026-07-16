# HTTP 网页抓取工具示例

本示例演示如何在 AI 智能体的交互式对话中使用 HTTP 网页抓取工具。该工具可以抓取并提取网页内容，将 HTML 转换为更易读的 Markdown，并支持多种文本格式。

## 运行示例

### 使用环境变量

```bash
export OPENAI_API_KEY="your-api-key-here"
export OPENAI_BASE_URL="https://api.openai.com/v1"  # 可选
go run main.go
```

### 使用自定义模型

```bash
export OPENAI_API_KEY="your-api-key-here"
go run main.go -model gpt-4o-mini
```

## 会话示例

```
amdahliu@AMDAHLIU-MC2 trpc-agent-go % ./dpskv3.sh
🚀 HTTP 网页抓取对话演示
模型：deepseek-v3-local-II
输入 'exit' 结束对话
可用工具：web_fetch
==================================================
✅ 网页抓取对话已就绪！会话：web-fetch-session-1763989894

💡 可以尝试提出以下问题：
   - 总结 https://example.com 的内容
   - 抓取并比较 https://site1.com 和 https://site2.com
   - https://news.ycombinator.com 的首页有什么内容
   - 提取 https://blog.example.com/article 的要点
   - 获取 https://api.example.com/docs 的 API 文档

ℹ️  注意：该工具支持 HTML、JSON、XML 和纯文本格式

👤 用户：总结 https://ai.google.dev/gemini-api/docs/text-generation 的内容
🤖 助手：🌐 已开始网页抓取：
   • web_fetch (ID: chatcmpl-tool-2f80eb6504fc43b0adb62f36f21ee339)
     参数：{"urls":["https://ai.google.dev/gemini-api/docs/text-generation"]}

🔄 正在抓取网页内容……
✅ 抓取结果（ID：chatcmpl-tool-2f80eb6504fc43b0adb62f36f21ee339）：{"results":[{"retrieved_url":"https://ai.google.dev/gemini-api/docs/text-generation","status_code":200,"content_type":"text/html","content":"[跳到主要内容](#main-content)\n\n[![Gemini API](htt...

🤖 助手：该页面提供了使用 Gemini API 生成文本的完整指南。以下是要点摘要：

### **使用 Gemini API 生成文本**
1. **基本文本生成**：
   - 该 API 可以根据多种输入（文本、图像、视频和音频）生成文本。
   - 文档提供了 Python、JavaScript、Go、Java、REST 和 Apps Script 的示例代码片段。

2. **Gemini 2.5 的思考功能**：
   - Gemini 2.5 Flash 和 Pro 等模型默认启用“思考”功能，以提升输出质量。
   - 将 `thinking_budget` 设置为零即可禁用思考功能。

3. **系统指令**：
   - 可以使用系统指令引导模型的行为（例如：“你是一只猫，名字叫 Neko。”）。
   - 文档提供了多种编程语言的示例。

4. **多模态输入**：
   - 该 API 支持将文本与媒体文件（例如图像）结合使用。
   - 代码示例演示了如何处理多模态输入。

5. **流式响应**：
   - 在实时交互中，流式传输可以逐步返回响应。
   - 文档提供了 Python、JavaScript、Go、Java、REST 和 Apps Script 的流式传输示例。

6. **多轮对话（聊天）**：
   - SDK 支持聊天功能，可以保留对话历史记录。
   - 示例展示了如何实现多轮对话和流式聊天。

7. **支持的模型**：
   - 所有 Gemini 模型都支持文本生成。有关模型及其能力的详细信息，请参阅[模型](https://ai.google.dev/gemini-api/docs/models)页面。

8. **最佳实践**：
   - **提示技巧**：使用零样本或少样本提示来获得符合需求的输出。
   - **结构化输出**：该 API 可以生成 JSON 等结构化输出。

### **后续步骤**
- 尝试使用 [Gemini API 入门 Colab](https://colab.research.google.com/github/google-gemini/cookbook/blob/main/quickstarts/Get_started.ipynb)。
- 探索多模态能力（图像、视频、音频和文档理解）。

更多详细信息请参阅[官方文档](https://ai.google.dev/gemini-api/docs/text-generation)。

👤 用户：exit
👋 再见！
```
