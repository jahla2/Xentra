package application
import("context";"testing";"github.com/jahla2/Xentra/backend/control-plane/internal/domain")
type fakeInvestigator struct{}
func(fakeInvestigator)Investigate(context.Context,string,string)(domain.InvestigationResult,error){return domain.InvestigationResult{Summary:"failure",Confidence:"high",ProbableRootCause:"bad deploy",RecommendedAction:"rollback",Evidence:[]domain.Evidence{{Source:"docker.logs",Output:"error",Success:true}}},nil}
type fakeTimeline struct{}
func(fakeTimeline)FetchTimeline(context.Context,domain.RepositoryIntegration)([]domain.TimelineEvent,error){return []domain.TimelineEvent{{Source:"github",Kind:"commit",Summary:"deploy change"}},nil}
func TestIncidentCreatesTimelineAndActionRequiredStatus(t *testing.T){integrations:=NewMemoryIntegrationRepository();_ = integrations.Save(context.Background(),domain.RepositoryIntegration{ID:"int-1",EnvironmentID:"env-1",Provider:"github"});service:=NewIncidentService(NewMemoryIncidentRepository(),integrations,fakeInvestigator{},fakeTimeline{});incident,err:=service.Create(context.Background(),"env-1","why failed?");if err!=nil{t.Fatal(err)};if incident.Status!="action_required"||len(incident.Timeline)!=1||len(incident.Evidence)!=1{t.Fatalf("unexpected incident: %#v",incident)}}
