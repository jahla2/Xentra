import {useWorkspace} from '../app/WorkspaceProvider';

export function AuditPage(){
 const{audit}=useWorkspace();
 return <section className="panel">
  <div className="panel-title"><h2>Audit trail</h2><span>Organization scoped</span></div>
  {audit.map(item=><div className="audit-row" key={item.id}>
   <b>{item.eventType}</b>
   <span>{item.actor}</span>
   <span>{item.success?'success':'failed'}</span>
   <span>{new Date(item.createdAt).toLocaleString()}</span>
   <p>{item.detail}</p>
  </div>)}
  {!audit.length&&<p className="muted">No audited actions yet.</p>}
 </section>;
}
