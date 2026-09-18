import {FormEvent,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function ApprovalsPage(){
 const{
  principal,environments,selectedEnvironmentId,setSelectedEnvironmentId,
  latestIncident,action,isOwner,loading,proposeAction,approveAction,rejectAction,
 }=useWorkspace();
 const[form,setForm]=useState({action:'docker.restart',target:'',reason:'Recover unhealthy service'});

 async function submit(event:FormEvent){
  event.preventDefault();
  await proposeAction(form.action,form.target,form.reason);
 }

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Propose remediation</h2><span>{isOwner?'Human approval required':'Owner permission required'}</span></div>
   {isOwner&&<form className="stack" onSubmit={submit}>
    <select value={selectedEnvironmentId} onChange={e=>setSelectedEnvironmentId(e.target.value)} required>
     <option value="">Select environment</option>{environments.map(env=><option value={env.id} key={env.id}>{env.name}</option>)}
    </select>
    <select value={form.action} onChange={e=>setForm({...form,action:e.target.value})}>
     <option value="docker.restart">Restart Docker container</option>
     <option value="system.service_restart">Restart system service</option>
    </select>
    <input value={form.target} onChange={e=>setForm({...form,target:e.target.value})} placeholder="Target container / service" required/>
    <input value={form.reason} onChange={e=>setForm({...form,reason:e.target.value})} placeholder="Reason" required/>
    <button className="primary" disabled={!selectedEnvironmentId||loading}>Propose action</button>
   </form>}
   {latestIncident&&<p className="muted">Current incident context: {latestIncident.summary||latestIncident.question}</p>}
  </div>
  <div className="panel">
   <div className="panel-title"><h2>Approval</h2><span>Typed mutations only</span></div>
   {action
    ?<div className="finding">
      <p><b>{action.action}</b> → {action.target}</p>
      <p>Status: {action.status}</p>
      <p>{action.reason}</p>
      {action.status==='pending_approval'&&isOwner&&<div className="quick-links">
       <button className="nav" disabled={loading} onClick={()=>void rejectAction()}>Reject</button>
       <button className="primary" disabled={loading} onClick={()=>void approveAction()}>Approve & Run as {principal?.email}</button>
      </div>}
      {action.status==='rejected'&&<p>Rejected by {action.rejectedBy||'owner'}.</p>}
      {action.verification?.summary&&<p>Verification: {action.verification.summary}</p>}
     </div>
    :<p className="muted">No remediation proposal is active in this session.</p>}
  </div>
 </section>;
}
