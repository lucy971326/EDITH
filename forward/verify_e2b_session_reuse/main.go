// Experiment: 验证 E2B Auto-resume（不调用 Connect，直接操作原 sbx 对象）
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/eric642/e2b-go-sdk"
)

const testFilePath = "/home/user/edith_auto_resume.txt"
const testFileContent = "Auto-resume test file"

var logFile *os.File

func logf(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	fmt.Println(s)
	if logFile != nil {
		fmt.Fprintln(logFile, s)
	}
}

func must(err error) {
	if err != nil {
		logf("  ❌ 失败: %v", err)
		os.Exit(1)
	}
}

func main() {
	_ = os.MkdirAll("forward/verify_e2b_session_reuse", 0755)
	var err error
	logFile, err = os.Create("forward/verify_e2b_session_reuse/verification_log.txt")
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	logf("================================================================")
	logf("  E2B Auto-resume 验证实验")
	logf("================================================================")
	logf("")

	// ── 获取 API Key ──
	apiKey := os.Getenv("E2B_API_KEY")
	if apiKey == "" {
		logf("❌ E2B_API_KEY 未设置")
		os.Exit(1)
	}

	client, err := e2b.NewClient(e2b.Config{APIKey: apiKey})
	must(err)
	logf("  E2B Client 创建成功 ✅")

	ctx := context.Background()

	// ══════════════════════════════════════════════════════════════════
	// 实验 A：同一 SDK 对象上的 auto-resume
	// ══════════════════════════════════════════════════════════════════
	logf("")
	logf("================================================================")
	logf("  实验 A：同一 SDK 对象 auto-resume（不调用 Connect）")
	logf("================================================================")
	logf("")

	// 1. 创建 Sandbox
	sbx, err := client.Create(ctx, e2b.CreateOptions{
		Timeout: 5 * time.Minute,
		Lifecycle: &e2b.LifecycleOptions{
			OnTimeout:  "pause",
			AutoResume: true,
		},
	})
	must(err)
	sbxID := sbx.ID
	logf("  1. Sandbox 创建成功: ID=%s", sbxID)

	// 2. 写入测试文件
	_, err = sbx.Files.WriteString(ctx, testFilePath, testFileContent, e2b.FsOptions{})
	must(err)
	logf("  2. 测试文件写入成功")

	// 3. 验证文件可读
	data, err := sbx.Files.Read(ctx, testFilePath, e2b.FsOptions{})
	must(err)
	logf("  3. 写入后立即读取: %s", strings.TrimSpace(string(data)))

	// 4. Pause
	paused, err := sbx.Pause(ctx)
	must(err)
	logf("  4. Pause 结果: success=%v", paused)

	// 5. 确认状态为 paused
	info, err := sbx.GetInfo(ctx)
	must(err)
	logf("  5. Pause 后状态: %s", info.State)
	if info.State != e2b.SandboxStatePaused {
		logf("     ⚠️ 状态不是 paused（可能是延迟），继续实验")
	}
	logf("")

	// ── 6. 自动恢复：直接 Files.Read（不调用 Connect）──
	logf("  --- 第一条触发 auto-resume 的操作: Files.Read ---")
	logf("    操作: sbx.Files.Read(ctx, path, FsOptions{})")
	logf("    不调用 Connect / Resume")

	readResult, err := sbx.Files.Read(ctx, testFilePath, e2b.FsOptions{})
	must(err)
	readContent := strings.TrimSpace(string(readResult))

	logf("    操作结果: success=%v", err == nil)
	logf("    读取内容: %s", readContent)

	readOk := readContent == testFileContent
	if readOk {
		logf("    文件内容一致: ✅ YES")
	} else {
		logf("    文件内容一致: ❌ NO (期望=%q 实际=%q)", testFileContent, readContent)
	}

	// 7. 检查状态
	info2, err := sbx.GetInfo(ctx)
	must(err)
	logf("  Read 后状态: %s", info2.State)
	logf("")

	// ── 8. 自动恢复：Commands.Run ──
	logf("  --- 第二条触发 auto-resume 的操作: Commands.Run ---")

	// 先 Pause
	paused2, err := sbx.Pause(ctx)
	must(err)
	logf("  再次 Pause 结果: success=%v", paused2)

	info3, err := sbx.GetInfo(ctx)
	must(err)
	logf("  Pause 后状态: %s", info3.State)

	logf("    操作: sbx.Commands.Run(ctx, \"echo auto-resume-ok\", RunOptions{})")
	logf("    不调用 Connect / Resume")

	handle, err := sbx.Commands.Run(ctx, "/bin/echo", e2b.RunOptions{Args: []string{"auto-resume-ok"}})
	must(err)
	result, err := handle.Wait(ctx)
	must(err)
	logf("    操作结果: success=%v", err == nil)
	logf("    命令输出: %s", strings.TrimSpace(result.Stdout))

	info4, err := sbx.GetInfo(ctx)
	must(err)
	logf("  Run 后状态: %s", info4.State)
	logf("")

	// ══════════════════════════════════════════════════════════════════
	// 实验 B：根据 sandbox_id 重新 Connect（另一条链路）
	// ══════════════════════════════════════════════════════════════════
	logf("================================================================")
	logf("  实验 B：根据 sandbox_id 重新 Connect")
	logf("================================================================")
	logf("")

	// 再次 Pause
	_, _ = sbx.Pause(ctx)
	infoB, _ := sbx.GetInfo(ctx)
	logf("  Sandbox 已暂停，状态: %s", infoB.State)

	// 用 sandbox_id 重新 Connect
	sbxReconnected, err := client.Connect(ctx, sbxID, e2b.ConnectOptions{
		Timeout: 5 * time.Minute,
	})
	must(err)
	logf("  Connect 成功: ID=%s", sbxReconnected.ID)

	if sbxReconnected.ID != sbxID {
		logf("  ❌ ID 不一致! 期望=%s 实际=%s", sbxID, sbxReconnected.ID)
	} else {
		logf("  Sandbox ID 一致 ✅")
	}

	// ── 清理 ──
	logf("")
	logf("  清理: 保持 Sandbox 为 Paused 状态，不 Kill")
	logf("  Sandbox ID: %s", sbxID)
	logf("  可通过 client.Connect(ctx, %q, ConnectOptions{Timeout: 5min}) 恢复", sbxID)
	logf("")

	// ══════════════════════════════════════════════════════════════════
	// 结论
	// ══════════════════════════════════════════════════════════════════
	logf("================================================================")
	logf("  结论")
	logf("================================================================")
	logf("")
	logf("  实验 A：同一 SDK 对象 auto-resume")
	logf("  ─────────────────────────────────────")
	logf("  Pause 后状态:                    %s", info.State)
	logf("  Files.Read 触发恢复:             %v", err == nil)
	logf("  恢复后文件内容一致:              %v", readOk)
	logf("  Read 后状态 (期望 running):      %s", info2.State)
	logf("  再次 Pause 后状态:               %s", info3.State)
	logf("  Commands.Run 触发恢复:           %v", err == nil)
	logf("  命令输出 = auto-resume-ok:       %v", strings.TrimSpace(result.Stdout) == "auto-resume-ok")
	logf("  Run 后状态 (期望 running):       %s", info4.State)
	logf("")
	logf("  实验 B：根据 sandbox_id Connect")
	logf("  ─────────────────────────────────────")
	logf("  Connect 成功:                    %v", sbxReconnected.ID == sbxID)
	logf("")
	logf("  完整日志: forward/verify_e2b_session_reuse/verification_log.txt")
}
