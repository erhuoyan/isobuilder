import{E as e,P as t,g as n,k as r,ut as i}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{An as a,Gr as o,Hr as s,Kn as c,Lr as l,Mr as u,Nn as d,On as f,Qn as p,Sn as m,Ur as h,Wr as g,Xn as _,_n as v,bn as y,fn as b,gn as x,jr as S,qn as C,qr as w,xn as T,yn as E}from"./router-V1UvZaVY.js";import{_ as D,v as O}from"./index-hckf45nw.js";function k(e){let{lineHeight:t,borderRadius:n,fontWeightStrong:r,baseColor:i,dividerColor:a,actionColor:o,textColor1:s,textColor2:c,closeColorHover:l,closeColorPressed:d,closeIconColor:f,closeIconColorHover:p,closeIconColorPressed:m,infoColor:h,successColor:g,warningColor:_,errorColor:v,fontSize:y}=e;return Object.assign(Object.assign({},O),{fontSize:y,lineHeight:t,titleFontWeight:r,borderRadius:n,border:`1px solid ${a}`,color:o,titleTextColor:s,iconColor:c,contentTextColor:c,closeBorderRadius:n,closeColorHover:l,closeColorPressed:d,closeIconColor:f,closeIconColorHover:p,closeIconColorPressed:m,borderInfo:`1px solid ${u(i,S(h,{alpha:.25}))}`,colorInfo:u(i,S(h,{alpha:.08})),titleTextColorInfo:s,iconColorInfo:h,contentTextColorInfo:c,closeColorHoverInfo:l,closeColorPressedInfo:d,closeIconColorInfo:f,closeIconColorHoverInfo:p,closeIconColorPressedInfo:m,borderSuccess:`1px solid ${u(i,S(g,{alpha:.25}))}`,colorSuccess:u(i,S(g,{alpha:.08})),titleTextColorSuccess:s,iconColorSuccess:g,contentTextColorSuccess:c,closeColorHoverSuccess:l,closeColorPressedSuccess:d,closeIconColorSuccess:f,closeIconColorHoverSuccess:p,closeIconColorPressedSuccess:m,borderWarning:`1px solid ${u(i,S(_,{alpha:.33}))}`,colorWarning:u(i,S(_,{alpha:.08})),titleTextColorWarning:s,iconColorWarning:_,contentTextColorWarning:c,closeColorHoverWarning:l,closeColorPressedWarning:d,closeIconColorWarning:f,closeIconColorHoverWarning:p,closeIconColorPressedWarning:m,borderError:`1px solid ${u(i,S(v,{alpha:.25}))}`,colorError:u(i,S(v,{alpha:.08})),titleTextColorError:s,iconColorError:v,contentTextColorError:c,closeColorHoverError:l,closeColorPressedError:d,closeIconColorError:f,closeIconColorHoverError:p,closeIconColorPressedError:m})}var A={name:`Alert`,common:b,self:k},j=h(`alert`,`
 line-height: var(--n-line-height);
 border-radius: var(--n-border-radius);
 position: relative;
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-color);
 text-align: start;
 word-break: break-word;
`,[g(`border`,`
 border-radius: inherit;
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 transition: border-color .3s var(--n-bezier);
 border: var(--n-border);
 pointer-events: none;
 `),o(`closable`,[h(`alert-body`,[g(`title`,`
 padding-right: 24px;
 `)])]),g(`icon`,{color:`var(--n-icon-color)`}),h(`alert-body`,{padding:`var(--n-padding)`},[g(`title`,{color:`var(--n-title-text-color)`}),g(`content`,{color:`var(--n-content-text-color)`})]),D({originalTransition:`transform .3s var(--n-bezier)`,enterToProps:{transform:`scale(1)`},leaveToProps:{transform:`scale(0.9)`}}),g(`icon`,`
 position: absolute;
 left: 0;
 top: 0;
 align-items: center;
 justify-content: center;
 display: flex;
 width: var(--n-icon-size);
 height: var(--n-icon-size);
 font-size: var(--n-icon-size);
 margin: var(--n-icon-margin);
 `),g(`close`,`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 position: absolute;
 right: 0;
 top: 0;
 margin: var(--n-close-margin);
 `),o(`show-icon`,[h(`alert-body`,{paddingLeft:`calc(var(--n-icon-margin-left) + var(--n-icon-size) + var(--n-icon-margin-right))`})]),o(`right-adjust`,[h(`alert-body`,{paddingRight:`calc(var(--n-close-size) + var(--n-padding) + 2px)`})]),h(`alert-body`,`
 border-radius: var(--n-border-radius);
 transition: border-color .3s var(--n-bezier);
 `,[g(`title`,`
 transition: color .3s var(--n-bezier);
 font-size: 16px;
 line-height: 19px;
 font-weight: var(--n-title-font-weight);
 `,[s(`& +`,[g(`content`,{marginTop:`9px`})])]),g(`content`,{transition:`color .3s var(--n-bezier)`,fontSize:`var(--n-font-size)`})]),g(`icon`,{transition:`color .3s var(--n-bezier)`})]),M=e({name:`Alert`,inheritAttrs:!1,props:Object.assign(Object.assign({},a.props),{title:String,showIcon:{type:Boolean,default:!0},type:{type:String,default:`default`},bordered:{type:Boolean,default:!0},closable:Boolean,onClose:Function,onAfterLeave:Function,onAfterHide:Function}),slots:Object,setup(e){let{mergedClsPrefixRef:t,mergedBorderedRef:r,inlineThemeDisabled:o,mergedRtlRef:s}=C(e),u=a(`Alert`,`-alert`,j,A,e,t),f=d(`Alert`,s,t),p=n(()=>{let{common:{cubicBezierEaseInOut:t},self:n}=u.value,{fontSize:r,borderRadius:i,titleFontWeight:a,lineHeight:o,iconSize:s,iconMargin:c,iconMarginRtl:d,closeIconSize:f,closeBorderRadius:p,closeSize:m,closeMargin:h,closeMarginRtl:g,padding:_}=n,{type:v}=e,{left:y,right:b}=l(c);return{"--n-bezier":t,"--n-color":n[w(`color`,v)],"--n-close-icon-size":f,"--n-close-border-radius":p,"--n-close-color-hover":n[w(`closeColorHover`,v)],"--n-close-color-pressed":n[w(`closeColorPressed`,v)],"--n-close-icon-color":n[w(`closeIconColor`,v)],"--n-close-icon-color-hover":n[w(`closeIconColorHover`,v)],"--n-close-icon-color-pressed":n[w(`closeIconColorPressed`,v)],"--n-icon-color":n[w(`iconColor`,v)],"--n-border":n[w(`border`,v)],"--n-title-text-color":n[w(`titleTextColor`,v)],"--n-content-text-color":n[w(`contentTextColor`,v)],"--n-line-height":o,"--n-border-radius":i,"--n-font-size":r,"--n-title-font-weight":a,"--n-icon-size":s,"--n-icon-margin":c,"--n-icon-margin-rtl":d,"--n-close-size":m,"--n-close-margin":h,"--n-close-margin-rtl":g,"--n-padding":_,"--n-icon-margin-left":y,"--n-icon-margin-right":b}}),m=o?c(`alert`,n(()=>e.type[0]),p,e):void 0,h=i(!0),g=()=>{let{onAfterLeave:t,onAfterHide:n}=e;t&&t(),n&&n()};return{rtlEnabled:f,mergedClsPrefix:t,mergedBordered:r,visible:h,handleCloseClick:()=>{Promise.resolve(e.onClose?.call(e)).then(e=>{e!==!1&&(h.value=!1)})},handleAfterLeave:()=>{g()},mergedTheme:u,cssVars:o?void 0:p,themeClass:m?.themeClass,onRender:m?.onRender}},render(){var e;return(e=this.onRender)==null||e.call(this),r(x,{onAfterLeave:this.handleAfterLeave},{default:()=>{let{mergedClsPrefix:e,$slots:n}=this,i={class:[`${e}-alert`,this.themeClass,this.closable&&`${e}-alert--closable`,this.showIcon&&`${e}-alert--show-icon`,!this.title&&this.closable&&`${e}-alert--right-adjust`,this.rtlEnabled&&`${e}-alert--rtl`],style:this.cssVars,role:`alert`};return this.visible?r(`div`,Object.assign({},t(this.$attrs,i)),this.closable&&r(v,{clsPrefix:e,class:`${e}-alert__close`,onClick:this.handleCloseClick}),this.bordered&&r(`div`,{class:`${e}-alert__border`}),this.showIcon&&r(`div`,{class:`${e}-alert__icon`,"aria-hidden":`true`},_(n.icon,()=>[r(f,{clsPrefix:e},{default:()=>{switch(this.type){case`success`:return r(y,null);case`info`:return r(T,null);case`warning`:return r(E,null);case`error`:return r(m,null);default:return null}}})])),r(`div`,{class:[`${e}-alert-body`,this.mergedBordered&&`${e}-alert-body--bordered`]},p(n.header,t=>{let n=t||this.title;return n?r(`div`,{class:`${e}-alert-body__title`},n):null}),n.default&&r(`div`,{class:`${e}-alert-body__content`},n))):null}})}});export{M as t};