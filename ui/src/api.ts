export type Environment={id:string;name:string;type:string;connectionType:string;runnerUrl?:string;sshHost?:string;sshPort?:number;sshUser?:string;os:string;hostname:string;capabilities:string[]};
export type Evidence={source:string;output:string;success:boolean};
export type TimelineEvent={source:string;kind:string;summary:string;url?:string;occurredAt:string};
export type InvestigationResult={summary:string;confidence:string;probableRootCause:string;recommendedAction:string;evidence:Evidence[]};
export type Incident={id:string;environmentId:string;question:string;status:string;summary:string;rootCause:string;confidence:string;recommendedAction:string;evidence:Evidence[];timeline:TimelineEvent[];createdAt:string};
export type VerificationResult={healthy:boolean;summary:string;evidence:Evidence[]};
export type ActionRequest={id:string;incidentId?:string;environmentId:string;action:string;target:string;reason:string;status:string;approvedBy?:string;result?:string;verification:VerificationResult;createdAt:string;executedAt?:string};
export type AuditEvent={id:string;environmentId:string;actor:string;eventType:string;detail:string;success:boolean;createdAt:string};
export type RepositoryIntegration={id:string;environmentId:string;provider:string;owner:string;repo:string};
export type CreateEnvironmentInput={name:string;type:string;connectionType:'runner'|'ssh';runnerUrl?:string;sshHost?:string;sshPort?:number;sshUser?:string;sshHostKeyFingerprint?:string;sshPrivateKey?:string;sshPassphrase?:string};

const API_URL=import.meta.env.VITE_XENTRA_API_URL??'http://localhost:8080';

async function request<T>(path:string,init?:RequestInit):Promise<T>{
 const response=await fetch(`${API_URL}${path}`,{...init,headers:{'Content-Type':'application/json',...(init?.headers??{})}});
 if(!response.ok){const payload=await response.json().catch(()=>({error:'Request failed'}));throw new Error(payload.error??`Request failed with ${response.status}`)}
 return response.json() as Promise<T>
}

export const api={
 listEnvironments:()=>request<Environment[]>('/api/environments'),
 createEnvironment:(input:CreateEnvironmentInput)=>request<Environment>('/api/environments',{method:'POST',body:JSON.stringify(input)}),
 investigate:(environmentId:string,question:string)=>request<InvestigationResult>('/api/investigations',{method:'POST',body:JSON.stringify({environmentId,question})}),
 connectGitHub:(environmentId:string,owner:string,repo:string,accessToken:string)=>request<RepositoryIntegration>('/api/integrations/github',{method:'POST',body:JSON.stringify({environmentId,owner,repo,accessToken})}),
 listIncidents:()=>request<Incident[]>('/api/incidents'),
 createIncident:(environmentId:string,question:string)=>request<Incident>('/api/incidents',{method:'POST',body:JSON.stringify({environmentId,question})}),
 proposeAction:(input:{incidentId?:string;environmentId:string;action:string;target:string;reason:string})=>request<ActionRequest>('/api/actions',{method:'POST',body:JSON.stringify(input)}),
 approveAction:(id:string,approvedBy:string)=>request<ActionRequest>(`/api/actions/${id}/approve`,{method:'POST',body:JSON.stringify({approvedBy})}),
 listAudit:()=>request<AuditEvent[]>('/api/audit')
};
