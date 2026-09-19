import {useEffect,useMemo,useState} from 'react';
import {Link,useParams} from 'react-router-dom';
import {ActionRequest,api} from '../api';

const terminalStatuses=new Set(['completed','failed','verification_failed','rejected']);
const stages=['queued','executing','verifying','completed'];

function stageState(action:ActionRequest,stage:string){
 const current=stages.indexOf(action.executionStage);
 const target=stages.indexOf(stage);
 if(action.status==='failed'||action.status==='verification_failed'||action.status==='rejected'){
  if(target<current)return 'done';
  if(target===current)return 'failed';
  return 'pending';
 }
 if(current>target)return 'done';
 if(current===target)return stage==='completed'?'done':'active';
 return 'pending';
}

function mark(state:string){
 if(state==='done')return '✓';
 if(state==='active')return '●';
 if(state==='failed')return '✕';
 return '○';
}

export function LiveExecutionPage(){
 const{actionId=''}=useParams();
 const[action,setAction]=useState<ActionRequest|null>(null);
 const[error,setError]=useState('');

 useEffect(()=>{
  if(!actionId)return;
  let cancelled=false;
  let timer:number|undefined;

  async function poll(){
   try{
    const current=await api.getAction(actionId);
    if(cancelled)return;
    setAction(current);
    setError('');
    if(!terminalStatuses.has(current.status)){
     timer=window.setTimeout(()=>void poll(),750);
    }
   }catch(err){
    if(cancelled)return;
    setError((err as Error).message);
    timer=window.setTimeout(()=>void poll(),1500);
   }
  }

  void poll();
  return()=>{
   cancelled=true;
   if(timer)window.clearTimeout(timer);
  };
 },[actionId]);

 const steps=useMemo(()=>{
  if(!action)return[];
  return[
   {key:'approval',label:'Approval received',state:action.approvedBy?'done':action.status==='rejected'?'failed':'pending'},
   {key:'queued',label:'Queued for execution',state:stageState(action,'queued')},
   {key:'executing',label:'Executing '+action.action+' on '+action.target,state:stageState(action,'executing')},
   {key:'verifying',label:'Verifying service/container and configured health endpoint',state:stageState(action,'verifying')},
   {key:'completed',label:'Recovery verification complete',state:stageState(action,'completed')},
  ];
 },[action]);

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Live remediation execution</h2><span>{action?.status??'Loading'}</span></div>
   {error&&<p className="error">{error}</p>}
   {!action&&!error&&<p className="muted">Loading execution state…</p>}
   {action&&<>
    <div className="finding">
     <div className="confidence">{action.executionStage||action.status}</div>
     <h3>{action.action} → {action.target}</h3>
     <p>{action.reason}</p>
     {action.approvedBy&&<p>Approved by {action.approvedBy}</p>}
    </div>
    {steps.map(step=><div className="audit-row" key={step.key}>
     <b>{mark(step.state)} {step.label}</b>
     <span>{step.state}</span>
    </div>)}
   </>}
  </div>

  <div className="panel">
   <div className="panel-title"><h2>Execution result</h2><span>Persisted backend state</span></div>
   {action&&<>
    <p><b>Status:</b> {action.status}</p>
    <p><b>Stage:</b> {action.executionStage}</p>
    <p><b>Duration:</b> {action.durationMs?action.durationMs+' ms':'In progress'}</p>
    {action.startedAt&&<p><b>Started:</b> {new Date(action.startedAt).toLocaleString()}</p>}
    {action.completedAt&&<p><b>Completed:</b> {new Date(action.completedAt).toLocaleString()}</p>}
    <b>Command output</b>
    <pre>{action.result||'Waiting for execution output…'}</pre>
    <b>Verification</b>
    <p>{action.verification?.summary||'Verification has not completed yet.'}</p>
    {action.verification?.evidence?.map((item,index)=><details key={index}>
     <summary>{item.success?'✓':'✕'} {item.source} · {item.durationMs??0} ms</summary>
     <pre>{item.output}</pre>
    </details>)}
    <div className="quick-links">
     <Link className="nav" to="/approvals">Back to approvals</Link>
     {action.incidentId&&<Link className="nav" to="/incidents">View incident</Link>}
    </div>
   </>}
  </div>
 </section>;
}
