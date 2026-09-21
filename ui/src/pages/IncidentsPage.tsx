import {useMemo,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function IncidentsPage(){
 const{incidents,environments}=useWorkspace();
 const[selectedId,setSelectedId]=useState('');
 const selected=useMemo(()=>incidents.find(item=>item.id===selectedId)??incidents[0],[incidents,selectedId]);
 const environment=selected?environments.find(item=>item.id===selected.environmentId):undefined;

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Incidents</h2><span>{incidents.length} total</span></div>
   {incidents.map(item=><button className={'env '+(selected?.id===item.id?'selected':'')} key={item.id} onClick={()=>setSelectedId(item.id)}>
    <span className="dot"/><div><b>{item.summary||item.question}</b><small>{item.status} · {item.confidence||'unknown'} confidence</small><em>{new Date(item.createdAt).toLocaleString()}</em></div>
   </button>)}
   {!incidents.length&&<p className="muted">No incidents yet. Create one from an investigation or a failed GitHub workflow.</p>}
  </div>

  <div className="panel">
   <div className="panel-title"><h2>Incident detail</h2><span>{selected?.status??'No selection'}</span></div>
   {selected&&<>
    <div className="finding">
     <div className="confidence">{selected.confidence||'unknown'} confidence</div>
     <h3>{selected.rootCause||selected.summary||'Root cause not established'}</h3>
     <p>{selected.summary}</p>
     <p><b>Environment:</b> {environment?.name||selected.environmentId}</p>
     <p><b>Question:</b> {selected.question}</p>
     <p><b>Created:</b> {new Date(selected.createdAt).toLocaleString()}</p>
    </div>

    <h3>Recommended action</h3>
    <p>{selected.recommendedAction||'No recommendation yet.'}</p>

    <details open>
     <summary>Evidence ({selected.evidence.length})</summary>
     {selected.evidence.map((item,index)=><div className="finding compact" key={index}>
      <p><b>{item.success?'✓':'✕'} {item.source}</b> · {item.durationMs??0} ms · {new Date(item.occurredAt).toLocaleString()}</p>
      <pre>{item.output}</pre>
     </div>)}
     {!selected.evidence.length&&<p className="muted">No evidence persisted for this incident.</p>}
    </details>

    <details open>
     <summary>Investigation history / timeline ({selected.timeline.length})</summary>
     {selected.timeline.map((event,index)=><div className="audit-row" key={index}>
      <b>{event.kind} · {event.source}</b>
      <span>{new Date(event.occurredAt).toLocaleString()}</span>
      <p>{event.summary}</p>
      {event.url&&<a href={event.url} target="_blank" rel="noreferrer">Open source</a>}
     </div>)}
     {!selected.timeline.length&&<p className="muted">No timeline events recorded.</p>}
    </details>
   </>}
  </div>
 </section>;
}
