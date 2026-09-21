import {FormEvent,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function SettingsPage(){
 const{principal,isOwner,loading,createMember}=useWorkspace();
 const[form,setForm]=useState({email:'',password:''});
 const[message,setMessage]=useState('');

 async function submit(event:FormEvent){
  event.preventDefault();
  await createMember(form.email,form.password);
  setMessage(`Member ${form.email} created.`);
  setForm({email:'',password:''});
 }

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Workspace</h2><span>Current session</span></div>
   <p><b>{principal?.organizationName}</b></p>
   <p className="muted">{principal?.email} · {principal?.role}</p>
  </div>
  <div className="panel">
   <div className="panel-title"><h2>Team access</h2><span>{isOwner?'Create member account':'Owner permission required'}</span></div>
   {message&&<div className="finding"><p>{message}</p></div>}
   {isOwner&&<form className="stack" onSubmit={submit}>
    <input type="email" value={form.email} onChange={e=>setForm({...form,email:e.target.value})} placeholder="Member email" required/>
    <input type="password" value={form.password} onChange={e=>setForm({...form,password:e.target.value})} placeholder="Temporary password (10+ characters)" required/>
    <button className="primary" disabled={loading}>Add member</button>
   </form>}
  </div>
 </section>;
}
