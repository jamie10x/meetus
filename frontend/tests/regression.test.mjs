import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import ts from 'typescript';
import { webcrypto } from 'node:crypto';

function runtime(fetch) {
 const values = new Map();
 const localStorage = { getItem: k => values.get(k) ?? null, setItem: (k,v) => values.set(k,v), removeItem: k => values.delete(k) };
 const globals = { localStorage, window: { dispatchEvent() {} }, navigator: {}, crypto: webcrypto, Event, Headers, Response, URL, process: { env: {} }, fetch };
 const modules = new Map();
 function load(file) {
  file = path.resolve(file);
  if (modules.has(file)) return modules.get(file);
  const exports = {}; modules.set(file,exports);
  const source=ts.transpileModule(fs.readFileSync(file,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
  vm.runInNewContext(source,{ ...globals, exports, require: name => load(path.resolve(path.dirname(file), name+'.ts')) });
  return exports;
 }
 return {load,localStorage};
}
const expired = () => new Response(JSON.stringify({error:{message:'expired'}}),{status:401});
const pair={accessToken:'fresh',refreshToken:'successor'};
test('missing refresh token does not poison a subsequent session',async()=>{
 let calls=0;
 const {load}=runtime(async url=>url.endsWith('/refresh') ? (calls++,new Response(JSON.stringify({data:pair}))) : expired());
 const api=load('src/lib/api.ts');
 await assert.rejects(api.api('/me',{auth:true}));
 api.storeTokens({accessToken:'old',refreshToken:'valid'});
 await assert.rejects(api.api('/me',{auth:true}));
 assert.equal(calls,1);
});
test('logout wins against a delayed refresh response',async()=>{
 let finish;let started;
 const inRefresh=new Promise(resolve=>{started=resolve});
 const {load}=runtime(async url=>{
  if(url.endsWith('/refresh')) {started();return new Promise(resolve=>{finish=resolve});}
  if(url.endsWith('/logout')) return new Response('{}');
  return expired();
 });
 const api=load('src/lib/api.ts');api.storeTokens({accessToken:'old',refreshToken:'valid'});
 const pending=api.api('/me',{auth:true});await inRefresh;
 api.clearTokens();finish(new Response(JSON.stringify({data:pair})));
 await assert.rejects(pending);assert.equal(api.getAccessToken(),null);assert.equal(api.getRefreshToken(),null);
});
test('transient refresh failure preserves credentials',async()=>{
 const {load}=runtime(async url=>url.endsWith('/refresh') ? new Response('{}',{status:503}) : expired());
 const api=load('src/lib/api.ts');api.storeTokens({accessToken:'old',refreshToken:'valid'});
 await assert.rejects(api.api('/me',{auth:true}));assert.equal(api.getRefreshToken(),'valid');
});
test('offline tickets cannot survive logout or cross an account switch',()=>{
 const {load}=runtime(()=>{});const api=load('src/lib/api.ts');const offline=load('src/lib/offline.ts');
 api.storeTokens(pair);offline.rememberUser({id:1,name:'A',isAdmin:true});
 assert.equal(offline.offlineUser().isAdmin,false);
 api.clearTokens();assert.equal(offline.offlineUser(),null);
 api.storeTokens(pair);offline.rememberUser({id:2,name:'B'});assert.equal(offline.offlineTickets(),undefined);
});
test('calendar identity is stable on edits and distinct for equal-length titles',()=>{
 const {load}=runtime(()=>{});const {buildIcsContent}=load('src/lib/ics.ts');
 const base={id:1,title:'One',startsAt:'2026-12-01T10:00:00Z',endsAt:null,description:'',isOnline:true};
 const uid=v=>buildIcsContent(v).match(/UID:(.*)/)[1];
 assert.notEqual(uid(base),uid({...base,id:2,title:'Two'}));
 assert.equal(uid(base),uid({...base,title:'Edited',startsAt:'2026-12-02T10:00:00Z'}));
});
test('worker only intercepts navigations and fingerprinted static assets',()=>{
 const handlers={};vm.runInNewContext(fs.readFileSync('public/sw.js','utf8'),{URL,self:{location:{origin:'https://meetus.uz'},addEventListener:(event,fn)=>{handlers[event]=fn;}}});
 for(const pathname of ['/api/me','/api/me/tickets','/api/events/mine','/api/explore/events','/uploads/image.jpg']){
  let intercepted=false;handlers.fetch({request:{method:'GET',url:'https://meetus.uz'+pathname,headers:new Headers()},respondWith:()=>{intercepted=true;}});
  assert.equal(intercepted,false,pathname);
 }
});
test('translation catalogs have exact key parity',()=>{
 const leaves=(value,prefix='')=>Object.entries(value).flatMap(([k,v])=>typeof v==='object'?leaves(v,prefix+k+'.'):[prefix+k]).sort();
 const catalogs=['uz','ru','en'].map(lang=>leaves(JSON.parse(fs.readFileSync(`messages/${lang}.json`,'utf8'))));
 assert.deepEqual(catalogs[0],catalogs[1]);assert.deepEqual(catalogs[1],catalogs[2]);
});

test('API failures use localized details and never arbitrary exception text', () => {
 const {load}=runtime(()=>{});
 const {ApiError}=load('src/lib/api.ts');
 const {errorMessage}=load('src/lib/errorMessage.ts');
 const ru=JSON.parse(fs.readFileSync('messages/ru.json','utf8')).errors;
 assert.equal(errorMessage(new ApiError('conflict','ticket already checked in',409), key=>ru[key], 'fallback'),ru.alreadyCheckedIn);
 assert.equal(errorMessage(new ApiError('forbidden','an unrecognized permission error',403), key=>ru[key], 'fallback'),ru.forbidden);
 assert.equal(errorMessage(new ApiError('internal_error','private implementation detail',500), key=>ru[key], 'fallback'),'fallback');
 assert.equal(errorMessage(new Error('private detail'), key=>ru[key], 'fallback'),'fallback');
});

test('event form schedules match Tashkent regardless of browser timezone', () => {
 const { load } = runtime(() => { throw new Error('Unexpected request'); });
 const { toEventTimeInput, fromEventTimeInput } = load('src/lib/eventTime.ts');
 assert.equal(toEventTimeInput('2026-10-06T22:30:00Z'), '2026-10-07T03:30');
 assert.equal(fromEventTimeInput('2026-10-07T03:30'), '2026-10-06T22:30:00.000Z');
 assert.equal(toEventTimeInput(null), '');
});
