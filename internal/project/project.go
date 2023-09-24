package project

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
)

const (
	PROJECTS_KV_PREFIX = "/projects"
)

var (
	ErrProjectNotFound      = errors.New("project not found")
	ErrNoCloudflareProvider = errors.New("cloudflare provider not present")

	_ core.Value               = (*Project)(nil)
	_ core.PotentiallySharable = (*Project)(nil)
	_ core.Project             = (*Project)(nil)
)

type Project struct {
	id                core.ProjectID
	projectFilesystem afs.Filesystem
	lock              core.SmartLock
	tempTokens        *TempProjectTokens
	secretsBucket     *s3_ns.Bucket

	//providers
	cloudflare *Cloudflare //can be nil
}

func (p *Project) Id() core.ProjectID {
	return p.id
}

func (p *Project) HasProviders() bool {
	return p.cloudflare != nil
}

func getProjectKvKey(id core.ProjectID) core.Path {
	return core.PathFrom(PROJECTS_KV_PREFIX + "/" + string(id))
}

type CreateProjectParams struct {
	Name string
}

// CreateProject
func (r *Registry) CreateProject(ctx *core.Context, params CreateProjectParams) (core.ProjectID, error) {
	id := core.RandomProjectID(params.Name)

	err := r.filesystem.MkdirAll(r.projectsDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", fmt.Errorf("failed to create directory to store projects: %w", err)
	}

	r.kv.Insert(ctx, getProjectKvKey(id), core.Nil, r)

	projectDir := r.filesystem.Join(r.projectsDir, string(id))

	err = r.filesystem.MkdirAll(projectDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for project %s: %w", id, err)
	}

	return id, nil
}

type OpenProjectParams struct {
	Id            core.ProjectID
	DevSideConfig DevSideProjectConfig `json:"config"`
	TempTokens    *TempProjectTokens   `json:"tempTokens,omitempty"`
}

type DevSideProjectConfig struct {
	Cloudflare *DevSideCloudflareConfig `json:"cloudflare,omitempty"`
}

type DevSideCloudflareConfig struct {
	AdditionalTokensApiToken string `json:"additional-tokens-api-token"`
	AccountID                string `json:"account-id"`
}

// NewDummyProject creates a project without any providers or tokens,
// the returned project should only be used in test.
func NewDummyProject(name string, fls afs.Filesystem) *Project {
	return &Project{
		id:                core.RandomProjectID(name),
		projectFilesystem: fls,
	}
}

// OpenProject
func (r *Registry) OpenProject(ctx *core.Context, params OpenProjectParams) (*Project, error) {
	if project, ok := r.openProjects[params.Id]; ok {
		return project, nil
	}

	_, found, err := r.kv.Get(ctx, getProjectKvKey(params.Id), r)

	if err != nil {
		return nil, fmt.Errorf("error while reading KV: %w", err)
	}

	if !found {
		return nil, ErrProjectNotFound
	}

	projectDir := r.filesystem.Join(r.projectsDir, string(params.Id))

	projectFS, err := fs_ns.OpenMetaFilesystem(ctx, r.filesystem, fs_ns.MetaFilesystemOptions{
		Dir: projectDir,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open filesystem of project %s", params.Id)
	}

	project := &Project{
		id:                params.Id,
		projectFilesystem: projectFS,
		tempTokens:        params.TempTokens,
	}

	if params.DevSideConfig.Cloudflare != nil {
		cf, err := newCloudflare(project.id, params.DevSideConfig.Cloudflare)
		if err != nil {
			return nil, fmt.Errorf("failed to create clouflare helper: %w", err)
		}
		project.cloudflare = cf
	}

	project.Share(nil)
	r.openProjects[project.id] = project

	return project, nil
}

func (p *Project) DeleteSecretsBucket(ctx *core.Context) error {
	bucket, err := p.getCreateSecretsBucket(ctx, false)
	if err != nil {
		return err
	}
	if bucket == nil {
		return nil
	}

	return p.cloudflare.DeleteR2Bucket(ctx, bucket)
}

func (p *Project) GetS3CredentialsForBucket(
	ctx *core.Context,
	bucketName string,
	provider string,
) (accessKey, secretKey string, s3Endpoint core.Host, _ error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	creds, err := p.cloudflare.GetCreateS3CredentialsForSingleBucket(ctx, bucketName, p.Id())
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %w", ErrNoR2Token, err)
	}
	accessKey = creds.accessKey
	secretKey = creds.secretKey
	s3Endpoint = creds.s3Endpoint
	return
}

func (p *Project) CanProvideS3Credentials(s3Provider string) (bool, error) {
	switch s3Provider {
	case "cloudflare":
		return p.cloudflare != nil, nil
	}
	return false, nil
}

func (p *Project) Filesystem() afs.Filesystem {
	return p.projectFilesystem
}

func (p *Project) IsMutable() bool {
	return true
}

func (p *Project) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherProject, ok := other.(*Project)
	if !ok {
		return false
	}

	return p == otherProject
}

func (p *Project) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	core.PrintType(w, p)
}

func (p *Project) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.ANY, nil
}

func (p *Project) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (p *Project) Share(originState *core.GlobalState) {
	p.lock.Share(originState, func() {

	})
}

func (p *Project) IsShared() bool {
	return p.lock.IsValueShared()
}

func (p *Project) ForceLock() {
	p.lock.ForceLock()
}

func (p *Project) ForceUnlock() {
	p.lock.ForceUnlock()
}
