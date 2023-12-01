package project_server

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/lsp"
)

const (
	CURRENT_PROJECT_CTX_KEY = core.Identifier("current-project")
	LSP_FS_CTX_KEY          = core.Identifier("current-filesystem")

	OPEN_PROJECT_METHOD   = "project/open"
	CREATE_PROJECT_METHOD = "project/create"
	DEPLOY_PROD_METHOD    = "project/deployService"
)

type CreateProjectParams struct {
	Name        string `json:"name"`
	AddTutFile  bool   `json:"addTutFile"`
	AddMainFile bool   `json:"addMainFile"`
}

type OpenProjectParams struct {
	ProjectId     core.ProjectID               `json:"projectId"`
	DevSideConfig project.DevSideProjectConfig `json:"config"`
	TempTokens    *project.TempProjectTokens   `json:"tempTokens,omitempty"`
}

type OpenProjectResponse struct {
	project.TempProjectTokens `json:"tempTokens"`
}

func registerProjectMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {
	projDir := string(opts.ProjectsDir)
	projectRegistry, err := project.OpenRegistry(projDir, opts.ProjectsDirFilesystem, server.Context())

	if err != nil {
		panic(err)
	}

	server.OnCustom(jsonrpc.MethodInfo{
		Name:          CREATE_PROJECT_METHOD,
		SensitiveData: true,
		NewRequest: func() interface{} {
			return &CreateProjectParams{}
		},
		RateLimits: []int{0, 0, 2},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			params := req.(*CreateProjectParams)

			projectId, err := projectRegistry.CreateProject(sessionCtx, project.CreateProjectParams{
				Name:        params.Name,
				AddTutFile:  params.AddTutFile,
				AddMainFile: params.AddMainFile,
			})

			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			return projectId, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name:          OPEN_PROJECT_METHOD,
		SensitiveData: true,
		NewRequest: func() interface{} {
			return &OpenProjectParams{}
		},
		RateLimits: []int{0, 2, 5},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			params := req.(*OpenProjectParams)

			_, ok := getProject(session)
			if ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "a project is already open",
				}
			}

			project, err := projectRegistry.OpenProject(sessionCtx, project.OpenProjectParams{
				Id:            params.ProjectId,
				DevSideConfig: params.DevSideConfig,
				TempTokens:    params.TempTokens,
			})

			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			sessionCtx.AddUserData(CURRENT_PROJECT_CTX_KEY, project)

			tokens, err := project.TempProjectTokens(sessionCtx)
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			lspFilesystem := NewFilesystem(project.LiveFilesystem(), fs_ns.NewMemFilesystem(10_000_000))

			sessionData := getLockedSessionData(session)
			defer sessionData.lock.Unlock()
			sessionData.filesystem = lspFilesystem
			sessionData.project = project

			return OpenProjectResponse{TempProjectTokens: tokens}, nil
		},
	})
}

func getProject(session *jsonrpc.Session) (*project.Project, bool) {
	p := session.Context().ResolveUserData(CURRENT_PROJECT_CTX_KEY)
	project, ok := p.(*project.Project)
	return project, ok
}
