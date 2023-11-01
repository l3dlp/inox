package project_server

import (
	"context"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/lsp"
)

type UpsertSecretParams struct {
	Name  string
	Value string
}

type ListSecretsParams struct {
}

type ListSecretsResponse struct {
	Secrets []core.ProjectSecretInfo `json:"secrets"`
}

type DeleteSecretParams struct {
	Name string
}

func registerSecretsMethodHandlers(server *lsp.Server, opts LSPServerConfiguration) {
	server.OnCustom(jsonrpc.MethodInfo{
		Name: "secrets/upsertSecret",
		NewRequest: func() interface{} {
			return &UpsertSecretParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			params := req.(*UpsertSecretParams)

			project, ok := getProject(session)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "no project is open",
				}
			}

			err := project.UpsertSecret(sessionCtx, params.Name, params.Value)
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}
			return nil, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "secrets/listSecrets",
		NewRequest: func() interface{} {
			return &ListSecretsParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			_ = req.(*ListSecretsParams)

			project, ok := getProject(session)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "no project is open",
				}
			}

			secrets, err := project.ListSecrets(sessionCtx)
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}
			return ListSecretsResponse{Secrets: secrets}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "secrets/deleteSecret",
		NewRequest: func() interface{} {
			return &DeleteSecretParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			params := req.(*DeleteSecretParams)

			project, ok := getProject(session)
			if !ok {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: "no project is open",
				}
			}

			err := project.DeleteSecret(sessionCtx, params.Name)
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}
			return nil, nil
		},
	})
}
