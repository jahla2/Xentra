import {FormEvent,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function IntegrationsPage(){
 const{
  environments,selectedEnvironmentId,setSelectedEnvironmentId,isOwner,loading,
  githubSetup,dismissGitHubSetup,connectGitHub,
 }=useWorkspace();
 const[form,setForm]=useState<{owner:string;repo:string;authMode:'github_app'|'token';accessToken:string}>({
  owner:'',repo:'',authMode:'github_app',accessToken:'',
 });

 async function submit(event:FormEvent){
  event.preventDefault();
  await connectGitHub(form.owner,form.repo,form.authMode,form.accessToken);
  setForm(current=>({...current,accessToken:''}));
 }

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>GitHub</h2><span>{isOwner?'Repository correlation':'Owner permission required'}</span></div>
   {isOwner&&<form className="stack" onSubmit={submit}>
    <select value={selectedEnvironmentId} onChange={e=>setSelectedEnvironmentId(e.target.value)} required>
     <option value="">Select environment</option>
     {environments.map(env=><option value={env.id} key={env.id}>{env.name}</option>)}
    </select>
    <input value={form.owner} onChange={e=>setForm({...form,owner:e.target.value})} placeholder="GitHub owner / organization" required/>
    <input value={form.repo} onChange={e=>setForm({...form,repo:e.target.value})} placeholder="Repository" required/>
    <select value={form.authMode} onChange={e=>setForm({...form,authMode:e.target.value as 'github_app'|'token'})}>
     <option value="github_app">GitHub App (recommended)</option>
     <option value="token">Access token fallback</option>
    </select>
    {form.authMode==='token'&&<input type="password" value={form.accessToken} onChange={e=>setForm({...form,accessToken:e.target.value})} placeholder="Access token" required/>}
    <button className="primary" disabled={!selectedEnvironmentId||loading}>Connect repository</button>
   </form>}
  </div>
  <div className="panel">
   <div className="panel-title"><h2>Webhook automation</h2><span>Failed workflow → incident</span></div>
   {githubSetup
    ?<div className="finding">
      <div className="confidence">One-time setup</div>
      <p>Configure a GitHub webhook for <b>workflow_run</b> events.</p>
      <b>Webhook path</b><pre>{githubSetup.webhookPath}</pre>
      <b>Webhook secret</b><pre>{githubSetup.webhookSecret}</pre>
      <p className="muted">Save this secret in GitHub now. Xentra keeps only the encrypted copy.</p>
      <button className="primary" onClick={dismissGitHubSetup}>I saved the secret</button>
     </div>
    :<p className="muted">Connect a repository to receive the one-time signed webhook setup details.</p>}
  </div>
 </section>;
}
