import {test} from 'node:test';
import assert from 'node:assert/strict';
import {dominantColor,toHex} from '../public/color.mjs';
test('dominant color ignores a minority highlight and transparent pixels',()=>{
 assert.equal(dominantColor(new Uint8ClampedArray([33,101,64,255,34,100,65,255,255,255,255,255,255,0,0,0])),'#226541');
 assert.equal(toHex([0,255,16]),'#00FF10');
 assert.throws(()=>dominantColor(new Uint8ClampedArray(4)));
});
