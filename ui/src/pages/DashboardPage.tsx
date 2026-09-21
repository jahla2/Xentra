import {Link} from 'react-router-dom';
import {useWorkspace} from '../app/WorkspaceProvider';

export function DashboardPage(){
 const{projects,environments,incidents,audit}=useWorkspace();
 const openIncidents=incidents.filter(item=>item.status!=='resolved');

 return <>
  <section className="grid metrics">
   <article><small>Projects</small><strong>{projects.length}</strong><span>Organization scoped</span></article>
   <article><small>Environments</small><strong>{environments.length}</strong><span>Connected targets</span></article>
   <article><small>Incidents</small><strong>{openIncidents.length}</strong><span>Open / action required</span></article>
   <article><small>Audit events</small><strong>{audit.length}</strong><span>Recorded operations</span></article>
  </section>
  <section className="grid workbench">
   <div className="panel">
    <div className="panel-title"><h2>Active incidents</h2><Link to="/incidents">View all</Link></div>
    {openIncidents.slice(0,5).map(item=><div className="finding compact" key={item.id}><b>{item.summary||item.question}</b><p>{item.status} · {item.confidence||'unknown'} confidence</p></div>)}
    {!openIncidents.length&&<p className="muted">No active incidents.</p>}
   </div>
   <div className="panel">
    <div className="panel-title"><h2>Quick actions</h2><span>Common workflows</span></div>
    <div className="quick-links">
     <Link className="primary link-button" to="/ask">Investigate environment</Link>
     <Link className="nav link-button" to="/environments">Connect environment</Link>
     <Link className="nav link-button" to="/integrations">Connect GitHub</Link>
    </div>
   </div>
  </section>
 </>;
}
