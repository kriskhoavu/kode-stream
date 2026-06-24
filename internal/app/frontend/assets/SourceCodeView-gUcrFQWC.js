import{c as n,r,v as k,j as e}from"./index-D0ekFKVs.js";import{H as p,y as v,x as f,t as j,s as w,r as M,a as N,p as T,m as S,k as C,j as E,b as O,c as L,g as _,d as H,e as z,f as D,h as R,i as I}from"./core-DjSkDD5G.js";/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const W=n("Check",[["path",{d:"M20 6 9 17l-5-5",key:"1gmf2c"}]]);/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const q=n("Copy",[["rect",{width:"14",height:"14",x:"8",y:"8",rx:"2",ry:"2",key:"17jyea"}],["path",{d:"M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2",key:"zix9uf"}]]);/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const A=n("ListOrdered",[["path",{d:"M10 12h11",key:"6m4ad9"}],["path",{d:"M10 18h11",key:"11hvi2"}],["path",{d:"M10 6h11",key:"c7qv1k"}],["path",{d:"M4 10h2",key:"16xx2s"}],["path",{d:"M4 6h1v4",key:"cnovpq"}],["path",{d:"M6 18H4c0-1 2-2 2-3s-1-1.5-2-1",key:"m9a95d"}]]);/**
 * @license lucide-react v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const K=n("WrapText",[["line",{x1:"3",x2:"21",y1:"6",y2:"6",key:"4m8b97"}],["path",{d:"M3 12h15a3 3 0 1 1 0 6h-4",key:"1cl7v7"}],["polyline",{points:"16 16 14 18 16 20",key:"1jznyi"}],["line",{x1:"3",x2:"10",y1:"18",y2:"18",key:"1h33wv"}]]);function B(s){return{name:"Dockerfile",aliases:["docker"],case_insensitive:!0,keywords:["from","maintainer","expose","env","arg","user","onbuild","stopsignal"],contains:[s.HASH_COMMENT_MODE,s.APOS_STRING_MODE,s.QUOTE_STRING_MODE,s.NUMBER_MODE,{beginKeywords:"run cmd entrypoint volume add copy workdir label healthcheck shell",starts:{end:/[^\\]$/,subLanguage:"bash"}}],illegal:"</"}}const G={bash:I,c:R,cpp:D,csharp:z,css:H,dockerfile:B,go:_,java:L,javascript:O,json:E,kotlin:C,makefile:S,python:T,ruby:N,rust:M,sql:w,typescript:j,xml:f,yaml:v};for(const[s,a]of Object.entries(G))p.registerLanguage(s,a);const P={shell:"bash",jsx:"javascript",tsx:"typescript",html:"xml"};function $({content:s,language:a,truncated:i=!1}){const[o,m]=r.useState(!1),[c,y]=r.useState(!0),[h,d]=r.useState(!1),l=new TextEncoder().encode(s).length<=k,x=r.useMemo(()=>s.split(`
`).map(t=>l?U(t,a):g(t)),[s,a,l]),b=async()=>{await navigator.clipboard.writeText(s),d(!0),window.setTimeout(()=>d(!1),1200)};return e.jsxs("div",{className:`source-code-view ${o?"wrap":""}`,children:[e.jsxs("div",{className:"viewer-toolbar source-toolbar","aria-label":"Source controls",children:[!l&&e.jsx("span",{className:"viewer-notice",children:"Highlighting paused for this large file."}),i&&e.jsx("span",{className:"viewer-notice",children:"Showing the first part of this file."}),e.jsx("span",{className:"viewer-toolbar-spacer"}),e.jsx("button",{type:"button",className:c?"active":"",title:"Toggle line numbers","aria-label":"Toggle line numbers","aria-pressed":c,onClick:()=>y(t=>!t),children:e.jsx(A,{size:15})}),e.jsx("button",{type:"button",className:o?"active":"",title:"Toggle line wrapping","aria-label":"Toggle line wrapping","aria-pressed":o,onClick:()=>m(t=>!t),children:e.jsx(K,{size:15})}),e.jsx("button",{type:"button",title:"Copy source","aria-label":"Copy source",onClick:()=>void b(),children:h?e.jsx(W,{size:15}):e.jsx(q,{size:15})}),e.jsx("span",{className:"sr-only","aria-live":"polite",children:h?"Source copied":""})]}),e.jsx("pre",{className:"source-code-scroll","data-language":a,children:e.jsx("code",{children:x.map((t,u)=>e.jsxs("span",{className:"source-code-line",children:[c&&e.jsx("span",{className:"source-line-number","aria-hidden":"true",children:u+1}),e.jsx("span",{className:"source-line-content",dangerouslySetInnerHTML:{__html:t||" "}})]},u))})})]})}function U(s,a){const i=P[a]??a;return p.getLanguage(i)?p.highlight(s,{language:i,ignoreIllegals:!0}).value:g(s)}function g(s){return s.replace(/[&<>"']/g,a=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#039;"})[a]??a)}export{$ as SourceCodeView};
