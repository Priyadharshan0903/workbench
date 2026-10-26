const {loadDashboard,publicAssets}=require('./helpers/dashboard.cjs');
// Real vendored htmx + XMLHttpRequest against an isolated temporary HTTP fixture.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),assert=require('node:assert/strict');
const {JSDOM,VirtualConsole}=require(process.env.JSDOM_MODULE||'jsdom');
const root=path.resolve(__dirname,'../..');let fragmentHits=0,failFragment=false,dom;
const project={id:'sample',name:'Sample project',description:'Fixture',kind:'web',path:'/tmp/sample',url:'http://127.0.0.1:3000/',ports:[3000],lifecycle:'active',owner:'Tester',launch:'echo ready',maintenance:'',docs:'README.md',status:'failed',statusMessage:'Fixture failed',services:[],conflicts:[],pathExists:true,extraServices:false,canLaunch:true,run:{runId:'fixture',phase:'failed'}};
const state={projects:[project],services:[],revision:1,inspectionError:null,scannedAt:new Date().toISOString(),registryPath:'/tmp/workbench/registry.json',csrfToken:'fixture',features:{launch:true,discovery:true,logs:true,readiness:true,htmlLogs:true},discoveryRoots:[],controlWarnings:[]};
const files=publicAssets();
const server=http.createServer((req,res)=>{
 const url=new URL(req.url,'http://localhost');
 res.setHeader('Content-Security-Policy',"default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'");
 if(url.pathname==='/api/state'){res.setHeader('Content-Type','application/json');res.end(JSON.stringify(state));return;}
 if(url.pathname==='/fragments/logs'){
  fragmentHits++;assert.equal(url.searchParams.get('id'),'sample');assert.equal(req.headers['hx-request'],'true');
  if(failFragment){res.writeHead(500);res.end('fixture error');return;}
  res.setHeader('Content-Type','text/html');res.end(`<p id="log-caption" class="footnote">Recent output ${fragmentHits}</p><pre id="log-output" class="log-output">&lt;script&gt;unsafe markup&lt;/script&gt;\nfixture output ${fragmentHits}</pre>`);return;
 }
 const file=files[url.pathname];if(!file){res.writeHead(404);res.end();return;}
 res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':file.endsWith('.woff2')?'font/woff2':file.endsWith('.svg')?'image/svg+xml':'text/html');res.end(fs.readFileSync(path.join(root,'web/static',file)));
});
const wait=async(fn,label)=>{const deadline=Date.now()+8000;while(Date.now()<deadline){if(fn())return;await new Promise(r=>setTimeout(r,20));}throw new Error('Timed out: '+label);};
(async()=>{
 await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
 const address=`http://127.0.0.1:${server.address().port}/`;
 const virtualConsole=new VirtualConsole();virtualConsole.on('jsdomError',e=>{if(!String(e.message).includes('Could not parse CSS'))console.error(e);});
 dom=await JSDOM.fromURL(address,{resources:'usable',runScripts:'dangerously',pretendToBeVisual:true,virtualConsole,beforeParse(w){
  // jsdom requires explicit XPath arguments that browsers default for htmx.
  const evaluate=w.XPathExpression.prototype.evaluate;w.XPathExpression.prototype.evaluate=function(context,type=0,result=null){return evaluate.call(this,context,type,result);};
  w.HTMLDialogElement.prototype.showModal=function(){this.setAttribute('open','');};w.HTMLDialogElement.prototype.close=function(){this.removeAttribute('open');};
  w.fetch=(url,options)=>fetch(new URL(url,address),options);w.open=()=>null;
 }});
 if(dom.window.document.readyState!=='complete')await new Promise(resolve=>dom.window.addEventListener('load',resolve,{once:true}));
 await loadDashboard(dom,{overHTTP:true});
 const w=dom.window,q=s=>w.document.querySelector(s);
 await wait(()=>q('.tool-card'),'dashboard loaded');
 assert.equal(w.htmx.version,'2.0.11');assert.equal(w.htmx.config.allowEval,false);assert.equal(w.htmx.config.allowScriptTags,false);
 q('[data-logs]').click();await wait(()=>q('#log-output')?.textContent.includes('fixture output 1'),'initial htmx log swap');
 assert.ok(q('#log-output').textContent.includes('<script>'));assert.equal(q('#log-output script'),null);assert.ok(q('#log-fragment').hasAttribute('hx-get'));
 q('[data-refresh-logs]').click();await wait(()=>q('#log-output')?.textContent.includes('fixture output 2'),'manual htmx log refresh');
 failFragment=true;q('[data-refresh-logs]').click();await wait(()=>q('#log-caption')?.textContent.includes('Could not refresh'),'fragment error recovery');
 assert.ok(q('#log-output').textContent.includes('fixture output 2'),'Failed refresh must preserve existing output');
 failFragment=false;q('[data-refresh-logs]').click();await wait(()=>q('#log-output')?.textContent.includes('fixture output 4'),'retry succeeds');
 q('[data-project=sample]').click();assert.equal(q('#log-fragment'),null);assert.equal(q('#detail-title').textContent,'Sample project');
 console.log('htmx checks passed: real local XHR, initial swap, refresh, safe text, failed refresh retention and retry.');
})().catch(e=>{console.error(e);process.exitCode=1;}).finally(()=>{dom?.window.close();server.closeAllConnections();server.close();});
