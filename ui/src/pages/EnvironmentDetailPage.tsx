import {useEffect,useMemo} from 'react';
import {Link,useParams} from 'react-router-dom';
import {useWorkspace} from '../app/WorkspaceProvider';

export function EnvironmentDetailPage(){
 const{environmentId=''}=useParams();
 const{environments,incidents,audit,setSelectedEnvironmentId}=useWorkspace();
 const environment=environments.find(item=>item.id===environmentId);

 useEffect(()=>{
  if(environmentId)setSelectedEnvironmentId(environmentId);
 },[environmentId,setSelectedEnvironmentId]);

 const recentActivity=useMemo(()=>{
  const items=[
   ...incidents
    .filter(item=>item.environmentId===environmentId)
    .map(item=>({at:item.createdAt,label:`Incident · ${item.status}`,detail:item.summary||item.question})),
   ...audit
    .filter(item=>item.environmentId===environmentId)
    .map(item=>({at:item.createdAt,label:item.eventType,detail:item.detail})),
  ];
  return items.sort((a,b)=>new Date(b.at).getTime()-new Date(a.at).getTime()).slice(0,10);
 },[audit,incidents,environmentId]);

 if(!environment){
  return <section className="panel">
   <div className="panel-title"><h2>Environment</h2><Link to="/environments">Back to environments</Link></div>
   <p className="muted">Environment not found or still loading.</p>
  </section>;
 }

 const connectionStatus=environment.hostname?'Connected':'Waiting for check-in';

 return <>
  <section className="panel">
   <div className="panel-title"><h2>{environment.name}</h2><Link to="/environments">Back to environments</Link></div>
   <div className="finding compact">
    <div className="confidence">{connectionStatus}</div>
    <h3>{environment.hostname||environment.sshHost||environment.runnerUrl||'Runner not checked in yet'}</h3>
    <p>{environment.type} · {environment.connectionType} · {environment.os||'unknown OS'}</p>
    <p>Health endpoint: {environment.healthUrl||'Not configured'}</p>
    <p>{environment.capabilities.length?environment.capabilities.join(' · '):'No capabilities discovered yet'}</p>
   </div>
  </section>

  <section className="grid metrics">
   <article><small>CPU</small><strong>{environment.cpu||'—'}</strong><span>Discovered snapshot</span></article>
   <article><small>Containers</small><strong>{environment.containers.length}</strong><span>Running services</span></article>
   <article><small>Capabilities</small><strong>{environment.capabilities.length}</strong><span>Available typed tools</span></article>
   <article><small>Status</small><strong>{connectionStatus}</strong><span>{environment.type}</span></article>
  </section>

  <section className="grid workbench">
   <div className="panel">
    <div className="panel-title"><h2>Resources</h2><span>Read-only discovery</span></div>
    <b>Memory</b>
    <pre>{environment.memory||'Not discovered yet'}</pre>
    <b>Disk</b>
    <pre>{environment.disk||'Not discovered yet'}</pre>
   </div>

   <div className="panel">
    <div className="panel-title"><h2>Running containers</h2><span>{environment.containers.length} detected</span></div>
    {environment.containers.map((container,index)=><div className="finding compact" key={`${container}-${index}`}><b>{container}</b></div>)}
    {!environment.containers.length&&<p className="muted">No running Docker containers were discovered.</p>}
   </div>
  </section>

  <section className="panel">
   <div className="panel-title"><h2>Recent activity</h2><span>Incidents and audited actions</span></div>
   {recentActivity.map((item,index)=><div className="audit-row" key={`${item.at}-${index}`}>
    <b>{item.label}</b>
    <span>{new Date(item.at).toLocaleString()}</span>
    <p>{item.detail}</p>
   </div>)}
   {!recentActivity.length&&<p className="muted">No recent activity for this environment.</p>}
  </section>
 </>;
}
