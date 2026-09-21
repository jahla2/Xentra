import {FormEvent,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function LoginPage(){
 const{authenticate,loading,error,clearError}=useWorkspace();
 const[mode,setMode]=useState<'login'|'register'>('login');
 const[form,setForm]=useState({email:'',password:'',organizationName:''});

 async function submit(event:FormEvent){
  event.preventDefault();
  await authenticate(mode,form.email,form.password,form.organizationName);
 }

 function toggle(){
  setMode(current=>current==='login'?'register':'login');
  clearError();
 }

 return <main className="main">
  <section className="panel auth-panel">
   <div className="brand"><span>X</span>Xentra</div>
   <div className="panel-title"><h2>{mode==='login'?'Sign in':'Create workspace'}</h2><span>Secure DevOps access</span></div>
   {error&&<div className="error">{error}</div>}
   <form className="stack" onSubmit={submit}>
    <input type="email" value={form.email} onChange={e=>setForm({...form,email:e.target.value})} placeholder="Email" required/>
    <input type="password" value={form.password} onChange={e=>setForm({...form,password:e.target.value})} placeholder="Password (10+ characters)" required/>
    {mode==='register'&&<input value={form.organizationName} onChange={e=>setForm({...form,organizationName:e.target.value})} placeholder="Organization name" required/>}
    <button className="primary" disabled={loading}>{loading?'Working…':mode==='login'?'Sign in':'Create organization'}</button>
   </form>
   <button className="nav" onClick={toggle}>{mode==='login'?'Need an account? Register':'Already have an account? Sign in'}</button>
  </section>
 </main>;
}
