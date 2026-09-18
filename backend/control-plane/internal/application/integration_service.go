package application

import(
	"context"
	"errors"
	"sync"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type RepositoryIntegrationRepository interface{Save(context.Context,domain.RepositoryIntegration)error;FindByEnvironment(context.Context,string)(domain.RepositoryIntegration,error)}
type GitHubTokenWriter interface{StoreGitHubToken(context.Context,string)(string,error)}
type IntegrationService struct{repo RepositoryIntegrationRepository;credentials GitHubTokenWriter}

func NewIntegrationService(repo RepositoryIntegrationRepository,credentials GitHubTokenWriter)*IntegrationService{return &IntegrationService{repo:repo,credentials:credentials}}

func(s *IntegrationService)ConnectGitHub(ctx context.Context,environmentID,owner,repoName,token string)(domain.RepositoryIntegration,error){
	if environmentID==""||owner==""||repoName==""||token==""{return domain.RepositoryIntegration{},errors.New("environmentId, owner, repo and accessToken are required")}
	credentialID,err:=s.credentials.StoreGitHubToken(ctx,token);if err!=nil{return domain.RepositoryIntegration{},err}
	id,err:=newResourceID("int");if err!=nil{return domain.RepositoryIntegration{},err}
	integration:=domain.RepositoryIntegration{ID:id,EnvironmentID:environmentID,Provider:"github",Owner:owner,Repo:repoName,CredentialID:credentialID}
	if err:=s.repo.Save(ctx,integration);err!=nil{return domain.RepositoryIntegration{},err}
	return integration,nil
}

type MemoryIntegrationRepository struct{mu sync.RWMutex;byEnvironment map[string]domain.RepositoryIntegration}
func NewMemoryIntegrationRepository()*MemoryIntegrationRepository{return &MemoryIntegrationRepository{byEnvironment:map[string]domain.RepositoryIntegration{}}}
func(r *MemoryIntegrationRepository)Save(_ context.Context,item domain.RepositoryIntegration)error{r.mu.Lock();defer r.mu.Unlock();r.byEnvironment[item.EnvironmentID]=item;return nil}
func(r *MemoryIntegrationRepository)FindByEnvironment(_ context.Context,id string)(domain.RepositoryIntegration,error){r.mu.RLock();defer r.mu.RUnlock();item,ok:=r.byEnvironment[id];if !ok{return domain.RepositoryIntegration{},errors.New("repository integration not found")};return item,nil}
