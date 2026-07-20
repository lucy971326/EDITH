module github-agent

go 1.26.2

replace trpc.group/trpc-go/trpc-agent-go => ./docs/trpc-agent-go

replace trpc.group/trpc-go/trpc-agent-go/server/agui => ./docs/trpc-agent-go/server/agui

require github.com/danielgtaylor/huma/v2 v2.39.0

require github.com/go-chi/chi/v5 v5.3.1 // indirect
