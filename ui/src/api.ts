export type Principal={userId:string;email:string;organizationId:string;organizationName:string;role:'owner'|'member'};
export type AuthSession={token:string;expiresAt:string;principal:Principal};
export type User={id:string;email:string;createdAt:string};
export type Project={id:string;name:string;description?:string;createdAt:string};
export type Environment={id:string;projectId:string;name:string;type:string;connectionType:string;runnerUrl?:string;sshHost?:string;sshPort?:number;sshUser?:string;os:string;hostname:string;capabilities:string[]};
export type Evidence={source:string;output:string;success:boolean};
export type TimelineEvent={source:string;kind:string;summary:string;url?:string;occurredAt:string};
export type InvestigationResult={summary:string;confidence:string;probableRootCause:string;recommendedAction:string;evidence:Evidence[]};
export type Incident={id:string;environmentId:string;question:string;status:string;summary:string;rootCause:string;confidence:string;recommendedAction:string;evidence:Evidence[];timeline:TimelineEvent[];createdAt:string};
export type VerificationResult={healthy:boolean;summary:string;evidence:Evidence[]};
export type ActionRequest={id:string;incidentId?:string;environmentId:string;action:string;target:string;reason:string;status:string;approvedBy?:string;result?:string;verification:VerificationResult;createdAt:string;executedAt?:string};
export type AuditEvent={id:string;environmentId:string;actor:string;eventType:string;detail:string;success:boolean;createdAt:string};
export type RepositoryIntegration={id:string;environmentId:string;provider:string;owner:string;repo:string};
export type CreateEnvironmentInput={projectId:string;name:string;type:string;connectionType:'runner'|'ssh';runnerUrl?:string;sshHost?:string;sshPort?:number;sshUser?:string;sshHostKeyFingerprint?:string;sshPrivateKey?:string;sshPassphrase?:string};

const API_URL=import.meta.env.VITE_XENTRA_API_URL??'http://localhost:8080';
const TOKEN_KEY='xentra_session';
let authToken=typeof window!=='undefined'?window.sessionStorage.getItem(TOKEN_KEY):null;

function setToken(token:string|null){
 authToken=token;
 if(typeof window==='undefined')return;
 if(token)window.sessionStorage.setItem(TOKEN_KEY,token);
 else window.sessionStorage.removeItem(TOKEN_KEY);
}

async function request<T>(path:string,init?:RequestInit):Promise<T>{
 const headers:Record<string,string>={'Content-Type':'application/json'};
 if(authToken)headers.Authorization=`Bearer ${authToken}`;
 const response=await fetch(`${API_URL}${path}`,{...init,headers:{...headers,...(init?.headers??{})}});
 if(!response.ok){
  const payload=await response.json().catch(()=>({error:'Request failed'}));
  throw new Error(payload.error??`Request failed with ${response.status}`);
 }
 if(response.status===204)return undefined as T;
 return response.json() as Promise<T>;
}

export const api={
 hasToken:()=>Boolean(authToken),
 setToken,
 register:(email:string,password:string,organizationName:string)=>request<AuthSession>('/api/auth/register',{method:'POST',body:JSON.stringify({email,password,organizationName})}),
 login:(email:string,password:string)=>request<AuthSession>('/api/auth/login',{method:'POST',body:JSON.stringify({email,password})}),
 logout:()=>request<void>('/api/auth/logout',{method:'POST'}),
 me:()=>request<Principal>('/api/auth/me'),
 createMember:(email:string,password:string)=>request<User>('/api/auth/members',{method:'POST',body:JSON.stringify({email,password})}),
 listProjects:()=>request<Project[]>('/api/projects'),
 createProject:(name:string,description:string)=>request<Project>('/api/projects',{method:'POST',body:JSON.stringify({name,description})}),
 listEnvironments:()=>request<Environment[]>('/api/environments'),
 createEnvironment:(input:CreateEnvironmentInput)=>request<Environment>('/api/environments',{method:'POST',body:JSON.stringify(input)}),
 investigate:(environmentId:string,question:string)=>request<InvestigationResult>('/api/investigations',{method:'POST',body:JSON.stringify({environmentId,question})}),
 connectGitHub:(environmentId:string,owner:string,repo:string,accessToken:string)=>request<RepositoryIntegration>('/api/integrations/github',{method:'POST',body:JSON.stringify({environmentId,owner,repo,accessToken})}),
 listIncidents:()=>request<Incident[]>('/api/incidents'),
 createIncident:(environmentId:string,question:string)=>request<Incident>('/api/incidents',{method:'POST',body:JSON.stringify({environmentId,question})}),
 proposeAction:(input:{incidentId?:string;environmentId:string;action:string;target:string;reason:string})=>request<ActionRequest>('/api/actions',{method:'POST',body:JSON.stringify(input)}),
 approveAction:(id:string)=>request<ActionRequest>(`/api/actions/${id}/approve`,{method:'POST'}),
 listAudit:()=>request<AuditEvent[]>('/api/audit')
};
