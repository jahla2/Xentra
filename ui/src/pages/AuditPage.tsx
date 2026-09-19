import {useWorkspace} from '../app/WorkspaceProvider';

export function AuditPage(){
 const{audit,environments}=useWorkspace();
 return <section className="panel">
  <div className="panel-title"><h2>Audit trail</h2><span>Organization scoped · {audit.length} events</span></div>
  {audit.map(item=>{
   const environment=environments.find(env=>env.id===item.environmentId);
   return <div className="audit-row" key={item.id}>
    <b>{item.eventType}</b>
    <span>{item.success?'success':'failed'}</span>
    <span>{new Date(item.createdAt).toLocaleString()}</span>
    <p><b>Actor:</b> {item.actor} · <b>Environment:</b> {environment?.name||item.environmentId}</p>
    {(item.tool||item.target)&&<p><b>Tool:</b> {item.tool||'—'} · <b>Target:</b> {item.target||'—'}</p>}
    {item.approval&&<p><b>Approval:</b> {item.approval}</p>}
    {item.actionId&&<p><b>Action ID:</b> {item.actionId}</p>}
    {item.durationMs>0&&<p><b>Duration:</b> {item.durationMs} ms</p>}
    <p>{item.detail}</p>
    {item.result&&<details><summary>Execution result</summary><pre>{item.result}</pre></details>}
   </div>;
  })}
  {!audit.length&&<p className="muted">No audited actions yet.</p>}
 </section>;
}
