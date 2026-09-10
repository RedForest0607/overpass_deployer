import test from 'node:test';
import assert from 'node:assert/strict';
import { APIError, request } from './client.ts';
import { projectsAPI } from './projects.ts';

test('client preserves revision conflicts without retrying a write', async t => {
  let count=0;
  t.mock.method(globalThis,'fetch',async () => {
    count++;
    return new Response(JSON.stringify({error:{code:'REVISION_CONFLICT',message:'conflict'}}),{status:409});
  });
  await assert.rejects(request('/projects/a',{method:'PUT',body:'{}'}),
    e=>e instanceof APIError && e.status===409 && e.code==='REVISION_CONFLICT');
  assert.equal(count,1);
});
test('project listing follows cursors and save sends the expected revision', async t => {
  const calls=[];
  t.mock.method(globalThis,'fetch',async (url,options) => {
    calls.push({url,options});
    return new Response(JSON.stringify(url.includes('topologies')?{revision:3}:
      url.includes('cursor')?{items:[{id:'b'}],nextCursor:null}:{items:[{id:'a'}],nextCursor:'next'}));
  });
  assert.deepEqual((await projectsAPI.list()).map(p=>p.id),['a','b']);
  await projectsAPI.save('a','qa',2,{blueprint:{},layout:{}});
  assert.equal(JSON.parse(calls[2].options.body).expectedRevision,2);
  assert.match(calls[2].url,/\/projects\/a\/topologies\/qa$/);
});
test('invalid server response is not presented as success',async t=>{
  t.mock.method(globalThis,'fetch',async()=>new Response('not json'));
  await assert.rejects(request('/projects'),e=>e.code==='INVALID_RESPONSE');
});
