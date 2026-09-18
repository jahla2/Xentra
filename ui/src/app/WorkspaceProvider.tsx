import {createContext,useContext,useEffect,useMemo,useState} from 'react';
import {api,ActionRequest,AuditEvent,AuthSession,CreateEnvironmentInput,Environment,GitHubIntegrationSetup,Incident,InvestigationResult,Principal,Project} from '../api';

type AuthMode='login'|'register';
type GitHubAuthMode='github_app'|'token';

type WorkspaceContextValue={
 principal:Principal|null;
 authReady:boolean;
 loading:boolean;
 error:string;
 projects:Project[];
 environments:Environment[];
 incidents:Incident[];
 audit:AuditEvent[];
 selectedEnvironmentId:string;
 question:string;
 investigation:InvestigationResult|null;
 latestIncident:Incident|null;
 action:ActionRequest|null;
 githubSetup:GitHubIntegrationSetup|null;
 isOwner:boolean;
 setSelectedEnvironmentId:(id:string)=>void;
 setQuestion:(value:string)=>void;
 clearError:()=>void;
 dismissGitHubSetup:()=>void;
 refresh:()=>Promise<void>;
 authenticate:(mode:AuthMode,email:string,password:string,organizationName:string)=>Promise<void>;
 logout:()=>Promise<void>;
 createMember:(email:string,password:string)=>Promise<void>;
 createProject:(name:string,description:string)=>Promise<void>;
 createEnvironment:(input:CreateEnvironmentInput)=>Promise<void>;
 investigate:()=>Promise<void>;
 createIncident:()=>Promise<void>;
 connectGitHub:(owner:string,repo:string,authMode:GitHubAuthMode,accessToken:string)=>Promise<void>;
 proposeAction:(action:string,target:string,reason:string)=>Promise<void>;
 approveAction:()=>Promise<void>;
};

const WorkspaceContext=createContext<WorkspaceContextValue|null>(null);

export function WorkspaceProvider({children}:{children:React.ReactNode}){
 const[principal,setPrincipal]=useState<Principal|null>(null);
 const[authReady,setAuthReady]=useState(false);
 const[loading,setLoading]=useState(false);
 const[error,setError]=useState('');
 const[projects,setProjects]=useState<Project[]>([]);
 const[environments,setEnvironments]=useState<Environment[]>([]);
 const[incidents,setIncidents]=useState<Incident[]>([]);
 const[audit,setAudit]=useState<AuditEvent[]>([]);
 const[selectedEnvironmentId,setSelectedEnvironmentId]=useState('');
 const[question,setQuestion]=useState('Why is the API down?');
 const[investigation,setInvestigation]=useState<InvestigationResult|null>(null);
 const[latestIncident,setLatestIncident]=useState<Incident|null>(null);
 const[action,setAction]=useState<ActionRequest|null>(null);
 const[githubSetup,setGitHubSetup]=useState<GitHubIntegrationSetup|null>(null);

 async function refresh(){
  if(!principal)return;
  try{
   const[projectItems,environmentItems,incidentItems,auditItems]=await Promise.all([
    api.listProjects(),
    api.listEnvironments(),
    api.listIncidents(),
    api.listAudit(),
   ]);
   setProjects(projectItems);
   setEnvironments(environmentItems);
   setIncidents(incidentItems);
   setAudit(auditItems);
   setSelectedEnvironmentId(current=>current||environmentItems[0]?.id||'');
  }catch(err){
   setError((err as Error).message);
  }
 }

 useEffect(()=>{
  if(!api.hasToken()){
   setAuthReady(true);
   return;
  }
  api.me()
   .then(setPrincipal)
   .catch(()=>api.setToken(null))
   .finally(()=>setAuthReady(true));
 },[]);

 useEffect(()=>{
  if(principal)void refresh();
 },[principal]);

 async function run<T>(operation:()=>Promise<T>,onSuccess?:(value:T)=>void,shouldRefresh=true){
  setLoading(true);
  setError('');
  try{
   const value=await operation();
   onSuccess?.(value);
   if(shouldRefresh)await refresh();
  }catch(err){
   setError((err as Error).message);
   throw err;
  }finally{
   setLoading(false);
  }
 }

 async function authenticate(mode:AuthMode,email:string,password:string,organizationName:string){
  setLoading(true);
  setError('');
  try{
   let session:AuthSession;
   if(mode==='register')session=await api.register(email,password,organizationName);
   else session=await api.login(email,password);
   api.setToken(session.token);
   setPrincipal(session.principal);
  }catch(err){
   setError((err as Error).message);
  }finally{
   setLoading(false);
   setAuthReady(true);
  }
 }

 async function logout(){
  try{await api.logout()}catch{/* expired sessions still clear locally */}
  api.setToken(null);
  setPrincipal(null);
  setProjects([]);
  setEnvironments([]);
  setIncidents([]);
  setAudit([]);
  setSelectedEnvironmentId('');
  setInvestigation(null);
  setLatestIncident(null);
  setAction(null);
  setGitHubSetup(null);
 }

 async function createMember(email:string,password:string){
  await run(()=>api.createMember(email,password),undefined,false);
 }

 async function createProject(name:string,description:string){
  await run(()=>api.createProject(name,description));
 }

 async function createEnvironment(input:CreateEnvironmentInput){
  await run(()=>api.createEnvironment(input),environment=>setSelectedEnvironmentId(environment.id));
 }

 async function investigate(){
  if(!selectedEnvironmentId)return;
  setInvestigation(null);
  await run(
   ()=>api.investigate(selectedEnvironmentId,question),
   setInvestigation,
   false,
  );
 }

 async function createIncident(){
  if(!selectedEnvironmentId)return;
  await run(
   ()=>api.createIncident(selectedEnvironmentId,question),
   incident=>{
    setLatestIncident(incident);
    setAction(null);
   },
  );
 }

 async function connectGitHub(owner:string,repo:string,authMode:GitHubAuthMode,accessToken:string){
  if(!selectedEnvironmentId)return;
  setGitHubSetup(null);
  await run(
   ()=>api.connectGitHub(selectedEnvironmentId,owner,repo,authMode,accessToken),
   setGitHubSetup,
   false,
  );
 }

 async function proposeAction(actionName:string,target:string,reason:string){
  if(!selectedEnvironmentId)return;
  await run(
   ()=>api.proposeAction({
    incidentId:latestIncident?.id,
    environmentId:selectedEnvironmentId,
    action:actionName,
    target,
    reason,
   }),
   setAction,
   false,
  );
 }

 async function approveAction(){
  if(!action)return;
  await run(()=>api.approveAction(action.id),setAction);
 }

 const value=useMemo<WorkspaceContextValue>(()=>({
  principal,authReady,loading,error,projects,environments,incidents,audit,
  selectedEnvironmentId,question,investigation,latestIncident,action,githubSetup,
  isOwner:principal?.role==='owner',
  setSelectedEnvironmentId,setQuestion,
  clearError:()=>setError(''),
  dismissGitHubSetup:()=>setGitHubSetup(null),
  refresh,authenticate,logout,createMember,createProject,createEnvironment,investigate,
  createIncident,connectGitHub,proposeAction,approveAction,
 }),[
  principal,authReady,loading,error,projects,environments,incidents,audit,
  selectedEnvironmentId,question,investigation,latestIncident,action,githubSetup,
 ]);

 return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}

export function useWorkspace(){
 const value=useContext(WorkspaceContext);
 if(!value)throw new Error('useWorkspace must be used inside WorkspaceProvider');
 return value;
}
