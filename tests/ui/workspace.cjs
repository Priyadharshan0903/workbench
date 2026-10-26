const {loadDashboard}=require('./helpers/dashboard.cjs');
// First-run, engine separation and theme migration use isolated browser storage.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {JSDOM}=require(process.env.JSDOM_MODULE||'jsdom');
const root=path.resolve(__dirname,'../..');
const base={description:'Local fixture',kind:'web',ports:[3100],lifecycle:'active',owner:'Tester',launch:'go run .',maintenance:'',docs:'README.md',status:'stopped',services:[],conflicts:[],extraServices:false,pathExists:true,canLaunch:true};
const engine={...base,id:'workbench',name:'Workbench',path:'/tmp/workbench',url:'http://127.0.0.1:8732/',ports:[8732],status:'ready',canLaunch:false};
const project={...base,id:'notes',name:'Notes',path:'/tmp/notes',url:'http://127.0.0.1:3100/'};
const state={projects:[engine],services:[],revision:1,inspectionError:null,scannedAt:new Date().toISOString(),registryPath:'/tmp/workbench/registry.json',csrfToken:'fixture',features:{launch:true,discovery:true,logs:true,readiness:true},discoveryRoots:['/tmp'],controlWarnings:[]};
const settle=()=>new Promise(r=>setTimeout(r,25));
async function create(saved,mobile=false){
 const dom=new JSDOM(fs.readFileSync(path.join(root,'web/static/index.html'),'utf8'),{url:'http://127.0.0.1:8732/',runScripts:'outside-only',pretendToBeVisual:true}),w=dom.window;
 w.HTMLDialogElement.prototype.showModal=function(){this.setAttribute('open','');};w.HTMLDialogElement.prototype.close=function(){this.removeAttribute('open');};
 w.localStorage.setItem('workbench.workspace.v2',JSON.stringify(saved));
 w.matchMedia=()=>({matches:mobile,addEventListener:()=>{}});
 w.fetch=async url=>{assert.equal(url,'/api/state');return {ok:true,json:async()=>structuredClone(state)}};
 await loadDashboard(dom);return dom;
}
(async()=>{
 const old={theme:'light',favorites:['notes'],order:['notes'],recent:{notes:123},layout:'board'};
 const dom=await create(old),w=dom.window,q=s=>w.document.querySelector(s);
 try{
  await settle();assert.equal(w.document.documentElement.dataset.theme,'dark');
  const migrated=JSON.parse(w.localStorage.getItem('workbench.workspace.v2'));assert.deepEqual(migrated.favorites,old.favorites);assert.deepEqual(migrated.order,old.order);assert.deepEqual(migrated.recent,old.recent);
  assert.equal(q('#stat-tools').textContent,'0');assert.equal(q('#tools-count').textContent,'0');assert.equal(q('.tool-card'),null);assert.ok(q('.first-run').textContent.includes('Add your first project'));
  q('#engine-details').click();assert.equal(q('#detail-title').textContent,'Workbench');q('[data-close=details]').click();
  q('.first-run [data-discover]').click();assert.ok(q('#discover-dialog').open);q('[data-close=discover-dialog]').click();
  q('[data-add-project]').click();assert.ok(q('#editor').open);q('[data-close=editor]').click();
  state.projects.push(project);q('#refresh').click();await settle();assert.equal(q('#tool-list').dataset.count,'1');assert.equal(q('#stat-tools').textContent,'1');assert.equal(q('.tool-card').dataset.id,'notes');assert.equal(q('.launch-command').textContent.trim(),'go run .');
  const firstTab=q('.project-tab[data-filter=all]');firstTab.focus();firstTab.dispatchEvent(new w.KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));assert.equal(w.document.activeElement.dataset.filter,'ready');assert.equal(q('.project-tab[data-filter=ready]').getAttribute('aria-selected'),'true');q('[data-reset-filters]').click();await settle();
  q('#tool-search').value='unmatched';q('#tool-search').dispatchEvent(new w.Event('input',{bubbles:true}));assert.equal(q('#tools-count').textContent,'0');assert.ok(q('.empty').textContent.includes('No projects match'));q('[data-reset-filters]').click();await settle();assert.equal(q('.tool-card').dataset.id,'notes');
  q('#theme-toggle').click();const saved=JSON.parse(w.localStorage.getItem('workbench.workspace.v2'));assert.equal(saved.theme,'light');assert.equal(saved.themeVersion,3);
  const restored=await create(saved);try{await settle();assert.equal(restored.window.document.documentElement.dataset.theme,'light','An explicit choice after migration must persist');}finally{restored.window.close();}
  const phone=await create(saved,true);try{await settle();const q=s=>phone.window.document.querySelector(s);assert.equal(q('#sidebar').inert,true);q('#menu-toggle').click();assert.equal(q('#sidebar').inert,false);assert.equal(q('#menu-toggle').getAttribute('aria-expanded'),'true');phone.window.document.dispatchEvent(new phone.window.KeyboardEvent('keydown',{key:'Escape',bubbles:true}));assert.equal(q('#sidebar').inert,true);assert.equal(phone.window.document.activeElement.id,'menu-toggle');}finally{phone.window.close();}
  console.log('Workspace checks passed: engine separation, first run, one-project layout, filtered counts/reset, saved themes, keyboard tabs and mobile navigation focus.');
 }finally{w.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
