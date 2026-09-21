import {FormEvent} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function AskPage(){
 const{
  environments,selectedEnvironmentId,setSelectedEnvironmentId,
  question,setQuestion,investigation,investigationProgress,loading,investigate,createIncident,
 }=useWorkspace();

 async function submit(event:FormEvent){
  event.preventDefault();
  await investigate();
 }

 return <section className="grid workbench">
  <div className="panel ai">
   <div className="panel-title"><h2>Ask Xentra</h2><span>Evidence-backed investigation</span></div>
   <form className="stack" onSubmit={submit}>
    <select value={selectedEnvironmentId} onChange={e=>setSelectedEnvironmentId(e.target.value)} required>
     <option value="">Select environment</option>
     {environments.map(env=><option value={env.id} key={env.id}>{env.name}</option>)}
    </select>
    <textarea value={question} onChange={e=>setQuestion(e.target.value)} placeholder="What happened to this environment?" required/>
    <button className="primary" disabled={!selectedEnvironmentId||loading}>{loading?'Investigating…':'Investigate'}</button>
   </form>
  </div>
  <div className="panel">
   <div className="panel-title"><h2>Finding</h2><span>{investigation?'Current result':loading?'Live investigation':'Waiting for investigation'}</span></div>
   {investigationProgress.length>0&&<details open={loading}>
    <summary>Live progress ({investigationProgress.length})</summary>
    {investigationProgress.map((event,index)=><div className="audit-row" key={index}>
     <b>{event.stage}</b>
     <span>{event.elapsedMs} ms</span>
     <span>{event.toolCalls} tool calls · {event.evidenceCount} evidence</span>
     <p>{event.message}</p>
     {event.evidence&&<pre>{event.evidence.output}</pre>}
    </div>)}
   </details>}
   {investigation
    ?<div className="finding">
      <div className="confidence">{investigation.confidence} confidence</div>
      <h3>{investigation.probableRootCause}</h3>
      <p>{investigation.summary}</p>
      <b>Recommended action</b><p>{investigation.recommendedAction}</p>
      <details><summary>Evidence ({investigation.evidence.length})</summary>
       {investigation.evidence.map((item,index)=><div key={index}><p><b>{item.source}</b> · {item.success?'success':'failed'} · {item.durationMs} ms</p><pre>{item.output}</pre></div>)}
      </details>
      <button className="primary" onClick={()=>void createIncident()} disabled={loading}>Create incident</button>
     </div>
    :<p className="muted">Select an environment and ask a question to begin.</p>}
  </div>
 </section>;
}
