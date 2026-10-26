const {loadDashboard}=require('./helpers/dashboard.cjs');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {JSDOM}=require(process.env.JSDOM_MODULE||'jsdom');
const root=path.resolve(__dirname,'../..'),calls=[];
let project={id:'website',name:'Website',description:'A local website',kind:'web',path:'/tmp/website',launch:'npm run dev',url:'',ports:[],lifecycle:'active',owner:'Tester',maintenance:'',docs:'README.md',services:[],conflicts:[],pathExists:true,extraServices:false,status:'available',canLaunch:true,launchSetupRequired:true};
const state={projects:[project],services:[],revision:1,inspectionError:null,scannedAt:new Date().toISOString(),registryPath:'/tmp/workbench/registry.json',csrfToken:'csrf',features:{launch:true,discovery:true,logs:true},discoveryRoots:[],controlWarnings:[]};
const dom=new JSDOM(fs.readFileSync(path.join(root,'web/static/index.html'),'utf8'),{url:'http://127.0.0.1:8732/',runScripts:'outside-only',pretendToBeVisual:true}),w=dom.window;
w.HTMLDialogElement.prototype.showModal=function(){this.setAttribute('open','');};w.HTMLDialogElement.prototype.close=function(){this.removeAttribute('open');};w.open=()=>null;
w.fetch=async(url,options)=>{
 calls.push({url,options});let payload;
 if(url==='/api/state')payload=state;
 else if(url==='/api/project'){const body=JSON.parse(options.body);assert.equal(body.revision,state.revision);project={...project,...body.project,status:'stopped',launchSetupRequired:false};state.projects=[project];state.revision++;payload={ok:true};}
 else if(url.startsWith('/api/launch-plan'))payload={id:project.id,name:project.name,cwd:project.path,command:project.launch,revision:state.revision,fingerprint:'reviewed'};
 else if(url==='/api/start'){project.status='starting';payload={ok:true,message:'Starting Website'};}
 else if(url.startsWith('/api/logs'))payload={text:'Vite fixture started',message:'Recent launch output'};
 else throw new Error('Unexpected request '+url);
 return {ok:true,json:async()=>structuredClone(payload)};
};
const q=s=>w.document.querySelector(s),settle=()=>new Promise(r=>setTimeout(r,25)),submit=()=>q('#tool-form').dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));
(async()=>{try{
 await loadDashboard(dom);await settle();
 assert.equal(q('[data-setup-start]').textContent.trim(),'Set up & start');assert.equal(q('[data-start]'),null);
 q('[data-setup-start]').click();assert.ok(q('#editor').open);assert.equal(q('[name=launch]').value,'npm run dev');assert.equal(q('#save-tool').textContent,'Save & review start');
 submit();await settle();assert.ok(q('#form-error').textContent.includes('local website URL'));assert.ok(!calls.some(c=>c.url==='/api/project'||c.url==='/api/start'));
 q('[name=url]').value='https://example.com';q('[name=ports]').value='443';submit();await settle();assert.ok(q('#form-error').textContent.includes('local website URL'));assert.ok(!calls.some(c=>c.url==='/api/project'));
 q('[name=url]').value='http://127.0.0.1:5173/';q('[name=ports]').value='3000';submit();await settle();assert.ok(q('#form-error').textContent.includes('include its port'));assert.ok(!calls.some(c=>c.url==='/api/project'));
 q('[name=ports]').value='5173';submit();await settle();
 assert.equal(project.launch,'npm run dev');assert.deepEqual(project.ports,[5173]);assert.ok(q('#launch-dialog').open);assert.ok(!q('#editor').open);assert.ok(q('#launch-body').textContent.includes('npm run dev'));
 assert.ok(!calls.some(c=>c.url==='/api/start'),'Saving setup must only review, never execute');
 q('[data-close=launch-dialog]').click();assert.ok(!calls.some(c=>c.url==='/api/start'),'Cancelling review must not execute');
 q('[data-start]').click();await settle();q('#open-when-ready').checked=false;q('#confirm-launch').click();await settle();
 const start=calls.find(c=>c.url==='/api/start');assert.ok(start);assert.deepEqual(JSON.parse(start.options.body),{id:'website',fingerprint:'reviewed',revision:2});assert.equal(start.options.headers['X-Workbench-Token'],'csrf');
 assert.ok(q('[data-logs]'));q('[data-logs]').click();await settle();assert.ok(q('#log-output').textContent.includes('Vite fixture started'));
 console.log('Launch setup passed: missing configuration, local URL/port checks, save-to-review, cancellation, explicit start and launch logs.');
 }finally{w.close();}})().catch(error=>{console.error(error);process.exitCode=1;});
