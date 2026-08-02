// Command server 启动 EDITH backend-v2。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"edith/backend-v2/internal/agentrun"
	"edith/backend-v2/internal/conversation"
	"edith/backend-v2/internal/cronadapter"
	"edith/backend-v2/internal/cronjob"
	"edith/backend-v2/internal/gateway"
	"edith/backend-v2/internal/images"
	"edith/backend-v2/internal/models"
	"edith/backend-v2/internal/sandbox"
	"edith/backend-v2/internal/skills"
	"edith/backend-v2/internal/systemtools"
	"edith/backend-v2/internal/tools"
	"edith/backend-v2/internal/usage"
	"edith/backend-v2/internal/userconfig"
	"edith/backend-v2/internal/volume"
	"edith/backend-v2/internal/webadapter"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessionsqlite "trpc.group/trpc-go/trpc-agent-go/session/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const appName = "EDITH"

func main() {
	loadEnvironment()
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	appDB, err := openDatabase(databasePath())
	if err != nil {
		log.Fatalf("打开 EDITH 数据库: %v", err)
	}
	defer appDB.Close()

	sessionDB, err := openDatabase(databasePath())
	if err != nil {
		log.Fatalf("打开会话数据库: %v", err)
	}
	rawSessions, err := sessionsqlite.NewService(sessionDB)
	if err != nil {
		sessionDB.Close()
		log.Fatalf("创建会话服务: %v", err)
	}
	defer rawSessions.Close()

	users, err := userconfig.New(userconfig.Dependencies{DB: appDB, DefaultModelID: models.DefaultModelID})
	if err != nil {
		log.Fatalf("创建用户配置模块: %v", err)
	}

	modelCatalog, err := models.New(models.Dependencies{Providers: users.Providers})
	if err != nil {
		log.Fatalf("创建模型模块: %v", err)
	}

	usageModule, err := usage.New(usage.Dependencies{DB: appDB})
	if err != nil {
		log.Fatalf("创建用量模块: %v", err)
	}

	imageModule, err := images.New(images.Dependencies{DB: appDB, Config: imageConfig()})
	if err != nil {
		log.Fatalf("创建图片模块: %v", err)
	}

	volumeModule, err := volume.New(volume.Dependencies{DB: appDB})
	if err != nil {
		log.Fatalf("创建 Volume 模块: %v", err)
	}

	sandboxModule, err := sandbox.New(sandbox.Dependencies{DB: appDB, Template: sandboxTemplate(), Volumes: volumeModule.Volumes})
	if err != nil {
		log.Fatalf("创建 Sandbox 模块: %v", err)
	}

	skillModule, err := skills.New(skills.Dependencies{Volumes: volumeModule.Volumes})
	if err != nil {
		log.Fatalf("创建 Skills 模块: %v", err)
	}

	cronJobs, err := cronjob.New(cronjob.Dependencies{DB: appDB, Settings: users.Settings})
	if err != nil {
		log.Fatalf("创建定时任务模块: %v", err)
	}

	conversations, err := conversation.New(conversation.Dependencies{AppName: appName, Sessions: rawSessions, Usage: usageModule.Reader})
	if err != nil {
		log.Fatalf("创建会话模块: %v", err)
	}

	systemTools := systemtools.New()
	agentTools := tools.New(tools.Dependencies{
		Tools: systemTools.Tools,
		ToolSets: []tool.ToolSet{
			sandboxModule.Tools,
			cronJobs.Tools,
		},
	})
	registeredModels := modelCatalog.Catalog.Registered()
	edithAgent := llmagent.New(
		"edith-chat",
		llmagent.WithModels(registeredModels),
		llmagent.WithModel(registeredModels[models.DefaultModelID]),
		llmagent.WithTools(agentTools.Tools),
		llmagent.WithToolSets(agentTools.ToolSets),
	)
	imageSessions := imageModule.SessionImages.Wrap(rawSessions)
	edithRunner := runner.NewRunner(
		appName,
		edithAgent,
		runner.WithSessionService(imageSessions),
	)
	managedRunner, ok := edithRunner.(runner.ManagedRunner)
	if !ok {
		log.Fatal("EDITH Runner 不支持任务控制")
	}
	defer func() {
		if err := managedRunner.Close(); err != nil {
			log.Printf("关闭 EDITH Runner: %v", err)
		}
	}()

	agentRuns, err := agentrun.New(agentrun.Dependencies{
		Runner:    managedRunner,        // Runner本体来执行Run
		Models:    modelCatalog.Catalog, // 模型信息<[fim-middle]>
		Settings:  users.Settings,
		Providers: users.Providers,
		MCP:       users.MCP,
		Images:    imageModule.AgentInput,
		Skills:    skillModule.Catalog,
		Usage:     usageModule.Recorder,
	})
	if err != nil {
		log.Fatalf("创建 AgentRun: %v", err)
	}

	agentGateway, err := gateway.New(gateway.Dependencies{Bindings: users.Bindings, AgentRuns: agentRuns})
	if err != nil {
		log.Fatalf("创建 Gateway: %v", err)
	}

	web, err := webadapter.New(agentGateway)
	if err != nil {
		log.Fatalf("创建 WebAdapter: %v", err)
	}
	cron, err := cronadapter.New(agentGateway)
	if err != nil {
		log.Fatalf("创建 CronAdapter: %v", err)
	}
	cronScheduler, err := cronJobs.NewScheduler(cron)
	if err != nil {
		log.Fatalf("创建定时任务调度器: %v", err)
	}

	mux := http.NewServeMux()

	users.HTTP.Register(mux)
	modelCatalog.HTTP.Register(mux)
	imageModule.HTTP.Register(mux)
	conversations.HTTP.Register(mux)
	cronJobs.HTTP.Register(mux)
	skillModule.HTTP.Register(mux)
	web.Register(mux)

	go cronScheduler.Run(shutdown)
	server := http.Server{Addr: runtimeAddress(), Handler: mux}
	go stopHTTPServer(shutdown, &server)
	log.Printf("EDITH backend-v2 正在监听 http://%s", server.Addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// stopHTTPServer 在进程收到退出信号后等待当前请求收尾。
func stopHTTPServer(shutdown context.Context, server *http.Server) {
	<-shutdown.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("关闭 HTTP 服务: %v", err)
	}
}
