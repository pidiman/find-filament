import {test} from 'node:test';
import assert from 'node:assert/strict';
import {dominantColor,toHex} from '../public/color.mjs';
import {createServer,parseResult,validateInput} from '../server.mjs';
const image='data:image/jpeg;base64,/9j/AA==';
test('dominant color ignores a minority highlight and transparent pixels',()=>{assert.equal(dominantColor(new Uint8ClampedArray([33,101,64,255,34,100,65,255,255,255,255,255,255,0,0,0])),'#226541');assert.equal(toHex([0,255,16]),'#00FF10');assert.throws(()=>dominantColor(new Uint8ClampedArray(4)));});
test('validates image, name and hex before API use',()=>{assert.equal(validateInput({name:' PLA ',image,photoHex:'#123456'}).name,'PLA');assert.throws(()=>validateInput({name:'',image,photoHex:'#123456'}));assert.throws(()=>validateInput({name:'PLA',image:'https://example.com/image',photoHex:'#123456'}));});
function response(result,url='https://maker.example/pla'){return {output:[{type:'web_search_call',action:{sources:[{url}]}},{type:'message',content:[{type:'output_text',text:JSON.stringify(result)}]}]};}
const match={match:true,name:'PLA Green',hex:'#aabbcc',method:'manufacturer',sourceUrl:'https://maker.example/pla',note:'Výrobca uvádza HEX.'};
test('rejects invented sources and invalid hex, accepts cited manufacturer result',()=>{assert.equal(parseResult(response(match)).result.hex,'#AABBCC');assert.equal(parseResult(response({...match,sourceUrl:'https://invented.example'})).match,false);assert.equal(parseResult(response({...match,hex:'red'})).match,false);assert.equal(parseResult(response({match:false})).match,false);assert.throws(()=>parseResult({output:[]}));});
test('HTTP: shell, private config, missing key, authentication, validation and upstream search',async()=>{
 let calls=0,payload;
 const server=createServer({env:{OPENAI_API_KEY:'test',LOOKUP_PASSWORD:'secret'},fetcher:async(url,options)=>{calls++;payload=JSON.parse(options.body);return {ok:true,json:async()=>response(match)};}});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));const base=`http://127.0.0.1:${server.address().port}`;
 try{assert.equal((await fetch(base)).status,200);assert.equal((await fetch(base+'/.env')).status,404);assert.equal((await fetch(base+'/server.mjs')).status,404);assert.equal((await fetch(base+'/api/identify',{method:'POST'})).status,401);const headers={'Content-Type':'application/json','Authorization':'Bearer secret'};assert.equal((await fetch(base+'/api/identify',{method:'POST',headers,body:'{}'})).status,400);const r=await fetch(base+'/api/identify',{method:'POST',headers,body:JSON.stringify({name:'PLA',image,photoHex:'#123456'})});assert.equal(r.status,200);assert.equal((await r.json()).result.hex,'#AABBCC');assert.equal(calls,1);assert.equal(payload.tools[0].type,'web_search');assert.equal(payload.input[0].content[1].image_url,image);assert.equal((await fetch(base+'/api/identify',{method:'POST',headers:{...headers,Origin:'https://evil.example'},body:'{}'})).status,403);}finally{await new Promise(r=>server.close(r));}
 const offline=createServer({env:{}});await new Promise(r=>offline.listen(0,'127.0.0.1',r));try{assert.equal((await fetch(`http://127.0.0.1:${offline.address().port}/api/identify`,{method:'POST'})).status,503);}finally{await new Promise(r=>offline.close(r));}
});
