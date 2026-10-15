import{E as e,Q as t,g as n,k as r,n as i,ut as a}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{An as o,Gr as s,Hr as c,Kn as l,Rr as u,Ur as d,hn as f,mn as p,pn as m,qn as h,qr as g,vr as _}from"./router-CncGxI6-.js";import{n as v}from"./index-MD8mUYkX.js";var y=c([c(`@keyframes spin-rotate`,`
 from {
 transform: rotate(0);
 }
 to {
 transform: rotate(360deg);
 }
 `),d(`spin-container`,`
 position: relative;
 `,[d(`spin-body`,`
 position: absolute;
 top: 50%;
 left: 50%;
 transform: translateX(-50%) translateY(-50%);
 `,[m()])]),d(`spin-body`,`
 display: inline-flex;
 align-items: center;
 justify-content: center;
 flex-direction: column;
 `),d(`spin`,`
 display: inline-flex;
 height: var(--n-size);
 width: var(--n-size);
 font-size: var(--n-size);
 color: var(--n-color);
 `,[s(`rotate`,`
 animation: spin-rotate 2s linear infinite;
 `)]),d(`spin-description`,`
 display: inline-block;
 font-size: var(--n-font-size);
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 margin-top: 8px;
 `),d(`spin-content`,`
 opacity: 1;
 transition: opacity .3s var(--n-bezier);
 pointer-events: all;
 `,[s(`spinning`,`
 user-select: none;
 -webkit-user-select: none;
 pointer-events: none;
 opacity: var(--n-opacity-spinning);
 `)])]),b={small:20,medium:18,large:16},x=e({name:`Spin`,props:Object.assign(Object.assign(Object.assign({},o.props),{contentClass:String,contentStyle:[Object,String],description:String,size:{type:[String,Number],default:`medium`},show:{type:Boolean,default:!0},rotate:{type:Boolean,default:!0},spinning:{type:Boolean,validator:()=>!0,default:void 0},delay:Number}),f),slots:Object,setup(e){let{mergedClsPrefixRef:r,inlineThemeDisabled:i}=h(e),s=o(`Spin`,`-spin`,y,v,e,r),c=n(()=>{let{size:t}=e,{common:{cubicBezierEaseInOut:n},self:r}=s.value,{opacitySpinning:i,color:a,textColor:o}=r;return{"--n-bezier":n,"--n-opacity-spinning":i,"--n-size":typeof t==`number`?u(t):r[g(`size`,t)],"--n-color":a,"--n-text-color":o}}),d=i?l(`spin`,n(()=>{let{size:t}=e;return typeof t==`number`?String(t):t[0]}),c,e):void 0,f=_(e,[`spinning`,`show`]),p=a(!1);return t(t=>{let n;if(f.value){let{delay:r}=e;if(r){n=window.setTimeout(()=>{p.value=!0},r),t(()=>{clearTimeout(n)});return}}p.value=f.value}),{mergedClsPrefix:r,active:p,mergedStrokeWidth:n(()=>{let{strokeWidth:t}=e;if(t!==void 0)return t;let{size:n}=e;return b[typeof n==`number`?`medium`:n]}),cssVars:i?void 0:c,themeClass:d?.themeClass,onRender:d?.onRender}},render(){var e;let{$slots:t,mergedClsPrefix:n,description:a}=this,o=t.icon&&this.rotate,s=(a||t.description)&&r(`div`,{class:`${n}-spin-description`},a||t.description?.call(t)),c=t.icon?r(`div`,{class:[`${n}-spin-body`,this.themeClass]},r(`div`,{class:[`${n}-spin`,o&&`${n}-spin--rotate`],style:t.default?``:this.cssVars},t.icon()),s):r(`div`,{class:[`${n}-spin-body`,this.themeClass]},r(p,{clsPrefix:n,style:t.default?``:this.cssVars,stroke:this.stroke,"stroke-width":this.mergedStrokeWidth,radius:this.radius,scale:this.scale,class:`${n}-spin`}),s);return(e=this.onRender)==null||e.call(this),t.default?r(`div`,{class:[`${n}-spin-container`,this.themeClass],style:this.cssVars},r(`div`,{class:[`${n}-spin-content`,this.active&&`${n}-spin-content--spinning`,this.contentClass],style:this.contentStyle},t),r(i,{name:`fade-in-transition`},{default:()=>this.active?c:null})):c}});export{x as t};