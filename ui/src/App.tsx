import {FormEvent,useEffect,useState} from 'react';
import {api,ActionRequest,AuditEvent,CreateEnvironmentInput,Environment,Incident,InvestigationResult} from './api';

const nav=['Overview','Environments','Incidents','Ask AI','Approvals','Audit'];

export function App(){
 const[environments,setEnvironments]=useState<Environment[]>([]);
 const[incidents,setIncidents]=useState<Incident[]>([]);
 const[audit,setAudit]=useState<AuditEvent[]>([]);
 const[selected,setSelected]=useState('');
 const[question,setQuestion]=useState('Why is the API down?');
 const[result,setResult]=useState<InvestigationResult|null>(null);
 const[latestIncident,setLatestIncident]=useState<Incident|null>(null);
 const[action,setAction]=useState<ActionRequest|null>(null);
 const[error,setError]=useState('');
 const[loading,setLoading]=useState(false);
 const[form,setForm]=useState<CreateEnvironmentInput>({name:'Production Ubuntu',type:'production',connectionType:'runner',runnerUrl:'http://localhost:8090',sshPort:22});
 const[github,setGitHub]=useState({owner:'',repo:'',accessToken:''});
 const[remediation,setRemediation]=useState({action:'docker.restart',target:'',reason:'Recover unhealthy service',approvedBy:''});

 const refresh=async()=>{
  try{
   const[envItems,incidentItems,auditItems]=await Promise.all([api.listEnvironments(),api.listIncidents(),api.listAudit()]);
   setEnvironments(envItems);setIncidents(incidentItems);setAudit(auditItems);
   if(!selected&&envItems[0])setSelected(envItems[0].id);
  }catch(err){setError((err as Error).message)}
 };
 useEffect(()=>{void refresh()},[]);

 const update=(patch:Partial<CreateEnvironmentInput>)=>setForm(current=>({...current,...patch}));
 async function run<T>(operation:()=>Promise<T>,onSuccess:(value:T)=>void){setLoading(true);setError('');try{onSuccess(await operation());await refresh()}catch(err){setError((err as Error).message)}finally{setLoading(false)}}
 async function addEnvironment(event:FormEvent){event.preventDefault();await run(()=>api.createEnvironment(form),env=>setSelected(env.id))}
 async function investigate(event:FormEvent){event.preventDefault();if(!selected)return;setResult(null);await run(()=>api.investigate(selected,question),setResult)}
 async function connectGitHub(event:FormEvent){event.preventDefault();if(!selected)return;await run(()=>api.connectGitHub(selected,github.owner,github.repo,github.accessToken),()=>setGitHub(current=>({...current,accessToken:''})))}
 async function createIncident(){if(!selected)return;await run(()=>api.createIncident(selected,question),incident=>{setLatestIncident(incident);setAction(null)})}
 async function proposeAction(event:FormEvent){event.preventDefault();if(!selected)return;await run(()=>api.proposeAction({incidentId:latestIncident?.id,environmentId:selected,action:remediation.action,target:remediation.target,reason:remediation.reason}),setAction)}
 async function approveAction(){if(!action)return;await run(()=>api.approveAction(action.id,remediation.approvedBy),setAction)}

 return <div className="shell">
  <aside className="sidebar"><div className="brand"><span>X</span>Xentra</div><nav>{nav.map((item,i)=><button key={item} className={i===0?'nav active':'nav'}>{item}</button>)}</nav></aside>
  <main className="main">
   <header><div><p className="eyebrow">DEVOPS COMMAND CENTER</p><h1>Infrastructure overview</h1></div><span className="status">● Control plane</span></header>
   {error&&<div className="error">{error}</div>}
   <section className="grid metrics">
    <article><small>Environments</small><strong>{environments.length}</strong><span>Runner or SSH connected</span></article>
    <article><small>Incidents</small><strong>{incidents.filter(i=>i.status!=='resolved').length}</strong><span>Open / action required</span></article>
    <article><small>Audit events</small><strong>{audit.length}</strong><span>Recorded actions</span></article>
   </section>

   <section className="grid workbench">
    <div className="panel">
     <div className="panel-title"><h2>Environments</h2><span>Secure discovery</span></div>
     {environments.map(env=><button className={`env ${selected===env.id?'selected':''}`} key={env.id} onClick={()=>setSelected(env.id)}><span className="dot"/><div><b>{env.name}</b><small>{env.hostname||env.sshHost||env.runnerUrl}</small><em>{env.connectionType} · {env.os} · {env.capabilities.join(' · ')||'basic'}</em></div></button>)}
     <form className="stack" onSubmit={addEnvironment}>
      <input value={form.name} onChange={e=>update({name:e.target.value})} placeholder="Environment name"/>
      <select value={form.connectionType} onChange={e=>update({connectionType:e.target.value as 'runner'|'ssh'})}><option value="runner">Xentra Runner</option><option value="ssh">Ubuntu / Linux SSH</option></select>
      {form.connectionType==='runner'?<input value={form.runnerUrl??''} onChange={e=>update({runnerUrl:e.target.value})} placeholder="Runner URL"/>:<><input value={form.sshHost??''} onChange={e=>update({sshHost:e.target.value})} placeholder="Host / IP"/><input value={form.sshUser??''} onChange={e=>update({sshUser:e.target.value})} placeholder="SSH username"/><input value={form.sshHostKeyFingerprint??''} onChange={e=>update({sshHostKeyFingerprint:e.target.value})} placeholder="Host key fingerprint (SHA256:...)"/><textarea value={form.sshPrivateKey??''} onChange={e=>update({sshPrivateKey:e.target.value})} placeholder="SSH private key"/><input type="password" value={form.sshPassphrase??''} onChange={e=>update({sshPassphrase:e.target.value})} placeholder="Key passphrase (optional)"/></>}
      <button className="primary" disabled={loading}>Test & connect environment</button>
     </form>
    </div>

    <div className="panel ai">
     <div className="panel-title"><h2>Ask Xentra</h2><span>Evidence-backed investigation</span></div>
     <form className="stack" onSubmit={investigate}>
      <select value={selected} onChange={e=>setSelected(e.target.value)}><option value="">Select environment</option>{environments.map(env=><option value={env.id} key={env.id}>{env.name}</option>)}</select>
      <textarea value={question} onChange={e=>setQuestion(e.target.value)}/>
      <button className="primary" disabled={!selected||loading}>{loading?'Working…':'Investigate'}</button>
     </form>
     {result&&<div className="finding"><div className="confidence">{result.confidence} confidence</div><h3>{result.probableRootCause}</h3><p>{result.summary}</p><b>Recommended action</b><p>{result.recommendedAction}</p><button className="primary" onClick={()=>void createIncident()} disabled={loading}>Create incident</button></div>}
    </div>
   </section>

   <section className="grid workbench">
    <div className="panel">
     <div className="panel-title"><h2>GitHub correlation</h2><span>Encrypted credential</span></div>
     <form className="stack" onSubmit={connectGitHub}><input value={github.owner} onChange={e=>setGitHub({...github,owner:e.target.value})} placeholder="GitHub owner"/><input value={github.repo} onChange={e=>setGitHub({...github,repo:e.target.value})} placeholder="Repository"/><input type="password" value={github.accessToken} onChange={e=>setGitHub({...github,accessToken:e.target.value})} placeholder="Access / installation token"/><button className="primary" disabled={!selected||loading}>Connect repository</button></form>
     {latestIncident&&<><h3>{latestIncident.rootCause}</h3><p>{latestIncident.status} · {latestIncident.confidence} confidence</p><details><summary>Timeline ({latestIncident.timeline.length})</summary>{latestIncident.timeline.map((event,i)=><p key={i}><b>{event.kind}</b> {event.summary}</p>)}</details></>}
    </div>

    <div className="panel">
     <div className="panel-title"><h2>Approved remediation</h2><span>Human gate required</span></div>
     <form className="stack" onSubmit={proposeAction}>
      <select value={remediation.action} onChange={e=>setRemediation({...remediation,action:e.target.value})}><option value="docker.restart">Restart Docker container</option><option value="system.service_restart">Restart system service</option></select>
      <input value={remediation.target} onChange={e=>setRemediation({...remediation,target:e.target.value})} placeholder="Target container / service"/>
      <input value={remediation.reason} onChange={e=>setRemediation({...remediation,reason:e.target.value})} placeholder="Reason"/>
      <button className="primary" disabled={!selected||loading}>Propose action</button>
     </form>
     {action&&<div className="finding"><p><b>{action.action}</b> → {action.target}</p><p>Status: {action.status}</p>{action.status==='pending_approval'&&<><input value={remediation.approvedBy} onChange={e=>setRemediation({...remediation,approvedBy:e.target.value})} placeholder="Approver name"/><button className="primary" disabled={!remediation.approvedBy||loading} onClick={()=>void approveAction()}>Approve & run</button></>}{action.verification?.summary&&<p>Verification: {action.verification.summary}</p>}</div>}
    </div>
   </section>

   <section className="panel">
    <div className="panel-title"><h2>Audit trail</h2><span>Last {Math.min(audit.length,8)} events</span></div>
    {audit.slice(0,8).map(item=><p key={item.id}><b>{item.eventType}</b> · {item.actor} · {item.success?'success':'failed'} — {item.detail}</p>)}
   </section>
  </main>
 </div>
}
