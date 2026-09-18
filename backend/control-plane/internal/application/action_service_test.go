package application

import(
	"context"
	"testing"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)
type fakeRemediation struct{executions int}
func(f *fakeRemediation)ExecuteAction(context.Context,domain.Environment,string,string)(string,error){f.executions++;return "restarted",nil}
func(f *fakeRemediation)VerifyAction(context.Context,domain.Environment,string,string)(domain.VerificationResult,error){return domain.VerificationResult{Healthy:true,Summary:"service healthy"},nil}

func TestActionRequiresApprovalBeforeExecution(t *testing.T){
	envs:=NewMemoryEnvironmentRepository();_ = envs.Save(context.Background(),domain.Environment{ID:"env-1"})
	incidents:=NewMemoryIncidentRepository();_ = incidents.Save(context.Background(),domain.Incident{ID:"inc-1",EnvironmentID:"env-1",Status:"action_required"})
	remediation:=&fakeRemediation{}
	service:=NewActionService(NewMemoryActionRepository(),NewMemoryAuditRepository(),envs,incidents,remediation)
	action,err:=service.Propose(context.Background(),"inc-1","env-1","docker.restart","api-prod","recover unhealthy API");if err!=nil{t.Fatal(err)}
	if remediation.executions!=0||action.Status!="pending_approval"{t.Fatalf("executed before approval: %#v",action)}
	action,err=service.Approve(context.Background(),action.ID,"rey");if err!=nil{t.Fatal(err)}
	if remediation.executions!=1||action.Status!="completed"||!action.Verification.Healthy{t.Fatalf("unexpected action: %#v",action)}
	incident,_:=incidents.Get(context.Background(),"inc-1");if incident.Status!="resolved"{t.Fatalf("incident not resolved: %#v",incident)}
}
func TestActionRejectsNonAllowlistedMutation(t *testing.T){envs:=NewMemoryEnvironmentRepository();_ = envs.Save(context.Background(),domain.Environment{ID:"env-1"});service:=NewActionService(NewMemoryActionRepository(),NewMemoryAuditRepository(),envs,NewMemoryIncidentRepository(),&fakeRemediation{});if _,err:=service.Propose(context.Background(),"","env-1","shell.exec","anything","test");err==nil{t.Fatal("expected non-allowlisted action to be rejected")}}
