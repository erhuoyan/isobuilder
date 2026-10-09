import{E as e,Q as t,g as n,k as r,n as i,ut as a}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{Gn as o,Kn as s,Pr as c,Rr as l,Ur as u,Vr as d,fn as f,hr as p,kn as m,mn as h,pn as g,zr as _}from"./router-CtFNVkoW.js";import{n as v}from"./index--SjCjhds.js";var y=l([l(`@keyframes spin-rotate`,`
 from {
 transform: rotate(0);
 }
 to {
 transform: rotate(360deg);
 }
 `),_(`spin-container`,`
 position: relative;
 `,[_(`spin-body`,`
 position: absolute;
 top: 50%;
 left: 50%;
 transform: translateX(-50%) translateY(-50%);
 `,[f()])]),_(`spin-body`,`
 display: inline-flex;
 align-items: center;
 justify-content: center;
 flex-direction: column;
 `),_(`spin`,`
 display: inline-flex;
 height: var(--n-size);
 width: var(--n-size);
 font-size: var(--n-size);
 color: var(--n-color);
 `,[d(`rotate`,`
 animation: spin-rotate 2s linear infinite;
 `)]),_(`spin-description`,`
 display: inline-block;
 font-size: var(--n-font-size);
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 margin-top: 8px;
 `),_(`spin-content`,`
 opacity: 1;
 transition: opacity .3s var(--n-bezier);
 pointer-events: all;
 `,[d(`spinning`,`
 user-select: none;
 -webkit-user-select: none;
 pointer-events: none;
 opacity: var(--n-opacity-spinning);
 `)])]),b={small:20,medium:18,large:16},x=e({name:`Spin`,props:Object.assign(Object.assign(Object.assign({},m.props),{contentClass:String,contentStyle:[Object,String],description:String,size:{type:[String,Number],default:`medium`},show:{type:Boolean,default:!0},rotate:{type:Boolean,default:!0},spinning:{type:Boolean,validator:()=>!0,default:void 0},delay:Number}),h),slots:Object,setup(e){let{mergedClsPrefixRef:r,inlineThemeDisabled:i}=s(e),l=m(`Spin`,`-spin`,y,v,e,r),d=n(()=>{let{size:t}=e,{common:{cubicBezierEaseInOut:n},self:r}=l.value,{opacitySpinning:i,color:a,textColor:o}=r;return{"--n-bezier":n,"--n-opacity-spinning":i,"--n-size":typeof t==`number`?c(t):r[u(`size`,t)],"--n-color":a,"--n-text-color":o}}),f=i?o(`spin`,n(()=>{let{size:t}=e;return typeof t==`number`?String(t):t[0]}),d,e):void 0,h=p(e,[`spinning`,`show`]),g=a(!1);return t(t=>{let n;if(h.value){let{delay:r}=e;if(r){n=window.setTimeout(()=>{g.value=!0},r),t(()=>{clearTimeout(n)});return}}g.value=h.value}),{mergedClsPrefix:r,active:g,mergedStrokeWidth:n(()=>{let{strokeWidth:t}=e;if(t!==void 0)return t;let{size:n}=e;return b[typeof n==`number`?`medium`:n]}),cssVars:i?void 0:d,themeClass:f?.themeClass,onRender:f?.onRender}},render(){var e;let{$slots:t,mergedClsPrefix:n,description:a}=this,o=t.icon&&this.rotate,s=(a||t.description)&&r(`div`,{class:`${n}-spin-description`},a||t.description?.call(t)),c=t.icon?r(`div`,{class:[`${n}-spin-body`,this.themeClass]},r(`div`,{class:[`${n}-spin`,o&&`${n}-spin--rotate`],style:t.default?``:this.cssVars},t.icon()),s):r(`div`,{class:[`${n}-spin-body`,this.themeClass]},r(g,{clsPrefix:n,style:t.default?``:this.cssVars,stroke:this.stroke,"stroke-width":this.mergedStrokeWidth,radius:this.radius,scale:this.scale,class:`${n}-spin`}),s);return(e=this.onRender)==null||e.call(this),t.default?r(`div`,{class:[`${n}-spin-container`,this.themeClass],style:this.cssVars},r(`div`,{class:[`${n}-spin-content`,this.active&&`${n}-spin-content--spinning`,this.contentClass],style:this.contentStyle},t),r(i,{name:`fade-in-transition`},{default:()=>this.active?c:null})):c}});export{x as t};