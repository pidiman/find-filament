export function toHex(rgb) { return '#' + rgb.map(v => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2,'0')).join('').toUpperCase(); }
// Most common quantized colour in a central patch; ignore isolated highlights/shadows.
export function dominantColor(data) {
 const bins = new Map();
 for(let i=0;i<data.length;i+=4){if(data[i+3]<128)continue;const rgb=[data[i],data[i+1],data[i+2]];const key=rgb.map(v=>v>>4).join(',');const b=bins.get(key)||{n:0,sum:[0,0,0]};b.n++;rgb.forEach((v,j)=>b.sum[j]+=v);bins.set(key,b);}
 const best=[...bins.values()].sort((a,b)=>b.n-a.n)[0];
 if(!best)throw new Error('Na tomto mieste nie je farba.');
 return toHex(best.sum.map(v=>v/best.n));
}
