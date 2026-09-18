import {useMemo,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function IncidentsPage(){
 const{incidents}=useWorkspace();
 const[selectedId,setSelectedId]=useState('');
 const selected=useMemo(()=>incidents.find(item=>item.id===selectedId)??incidents[0],[incidents,selectedId]);

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Incidents</h2><span>{incidents.length} total</span></div>
   {incidents.map(item=><button className={`env ${selected?.id===item.id?'selected':''}`} key={item.id} onClick={()=>setSelectedId(item.id)}>
    <span className="dot"/><div><b>{item.summary||item.question}</b><small>{item.status} · {item.confidence||'unknown'} confidence</small><em>{new Date(item.createdAt).toLocaleString()}</em></div>
   </button>)}
   {!incidents.length&&<p className="muted">No incidents yet. Create one from an investigation or a failed GitHub workflow.</p>}
  </div>
  <div className="panel">
   <div className="panel-title"><h2>Incident detail</h2><span>{selected?.status??'No selection'}</span></div>
   {selected&&<>
    <div className="confidence">{selected.confidence||'unknown'} confidence</div>
    <h3>{selected.rootCause||selected.summary}</h3>
    <p>{selected.question}</p>
    <b>Recommended action</b><p>{selected.recommendedAction||'No recommendation yet.'}</p>
    <details open><summary>Timeline ({selected.timeline.length})</summary>
     {selected.timeline.map((event,index)=><p key={index}><b>{event.kind}</b> · {event.source} · {new Date(event.occurredAt).toLocaleString()}<br/>{event.summary}</p>)}
    </details>
   </>}
  </div>
 </section>;
}
