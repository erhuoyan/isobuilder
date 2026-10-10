import{E as e,P as t,g as n,k as r,ut as i}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{Br as a,Dn as o,Dr as s,Gn as c,Kn as l,Mn as u,Nr as d,Or as f,Rr as p,Ur as m,Vr as h,Xn as g,Yn as _,bn as v,dn as y,gn as b,hn as x,kn as S,vn as C,xn as w,yn as T,zr as E}from"./router-45ByXqB-.js";import{_ as D,g as O}from"./index-CgJlLFc_.js";function k(e){let{lineHeight:t,borderRadius:n,fontWeightStrong:r,baseColor:i,dividerColor:a,actionColor:o,textColor1:c,textColor2:l,closeColorHover:u,closeColorPressed:d,closeIconColor:p,closeIconColorHover:m,closeIconColorPressed:h,infoColor:g,successColor:_,warningColor:v,errorColor:y,fontSize:b}=e;return Object.assign(Object.assign({},D),{fontSize:b,lineHeight:t,titleFontWeight:r,borderRadius:n,border:`1px solid ${a}`,color:o,titleTextColor:c,iconColor:l,contentTextColor:l,closeBorderRadius:n,closeColorHover:u,closeColorPressed:d,closeIconColor:p,closeIconColorHover:m,closeIconColorPressed:h,borderInfo:`1px solid ${f(i,s(g,{alpha:.25}))}`,colorInfo:f(i,s(g,{alpha:.08})),titleTextColorInfo:c,iconColorInfo:g,contentTextColorInfo:l,closeColorHoverInfo:u,closeColorPressedInfo:d,closeIconColorInfo:p,closeIconColorHoverInfo:m,closeIconColorPressedInfo:h,borderSuccess:`1px solid ${f(i,s(_,{alpha:.25}))}`,colorSuccess:f(i,s(_,{alpha:.08})),titleTextColorSuccess:c,iconColorSuccess:_,contentTextColorSuccess:l,closeColorHoverSuccess:u,closeColorPressedSuccess:d,closeIconColorSuccess:p,closeIconColorHoverSuccess:m,closeIconColorPressedSuccess:h,borderWarning:`1px solid ${f(i,s(v,{alpha:.33}))}`,colorWarning:f(i,s(v,{alpha:.08})),titleTextColorWarning:c,iconColorWarning:v,contentTextColorWarning:l,closeColorHoverWarning:u,closeColorPressedWarning:d,closeIconColorWarning:p,closeIconColorHoverWarning:m,closeIconColorPressedWarning:h,borderError:`1px solid ${f(i,s(y,{alpha:.25}))}`,colorError:f(i,s(y,{alpha:.08})),titleTextColorError:c,iconColorError:y,contentTextColorError:l,closeColorHoverError:u,closeColorPressedError:d,closeIconColorError:p,closeIconColorHoverError:m,closeIconColorPressedError:h})}var A={name:`Alert`,common:y,self:k},j=E(`alert`,`
 line-height: var(--n-line-height);
 border-radius: var(--n-border-radius);
 position: relative;
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-color);
 text-align: start;
 word-break: break-word;
`,[a(`border`,`
 border-radius: inherit;
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 transition: border-color .3s var(--n-bezier);
 border: var(--n-border);
 pointer-events: none;
 `),h(`closable`,[E(`alert-body`,[a(`title`,`
 padding-right: 24px;
 `)])]),a(`icon`,{color:`var(--n-icon-color)`}),E(`alert-body`,{padding:`var(--n-padding)`},[a(`title`,{color:`var(--n-title-text-color)`}),a(`content`,{color:`var(--n-content-text-color)`})]),O({originalTransition:`transform .3s var(--n-bezier)`,enterToProps:{transform:`scale(1)`},leaveToProps:{transform:`scale(0.9)`}}),a(`icon`,`
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
 `),a(`close`,`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 position: absolute;
 right: 0;
 top: 0;
 margin: var(--n-close-margin);
 `),h(`show-icon`,[E(`alert-body`,{paddingLeft:`calc(var(--n-icon-margin-left) + var(--n-icon-size) + var(--n-icon-margin-right))`})]),h(`right-adjust`,[E(`alert-body`,{paddingRight:`calc(var(--n-close-size) + var(--n-padding) + 2px)`})]),E(`alert-body`,`
 border-radius: var(--n-border-radius);
 transition: border-color .3s var(--n-bezier);
 `,[a(`title`,`
 transition: color .3s var(--n-bezier);
 font-size: 16px;
 line-height: 19px;
 font-weight: var(--n-title-font-weight);
 `,[p(`& +`,[a(`content`,{marginTop:`9px`})])]),a(`content`,{transition:`color .3s var(--n-bezier)`,fontSize:`var(--n-font-size)`})]),a(`icon`,{transition:`color .3s var(--n-bezier)`})]),M=e({name:`Alert`,inheritAttrs:!1,props:Object.assign(Object.assign({},S.props),{title:String,showIcon:{type:Boolean,default:!0},type:{type:String,default:`default`},bordered:{type:Boolean,default:!0},closable:Boolean,onClose:Function,onAfterLeave:Function,onAfterHide:Function}),slots:Object,setup(e){let{mergedClsPrefixRef:t,mergedBorderedRef:r,inlineThemeDisabled:a,mergedRtlRef:o}=l(e),s=S(`Alert`,`-alert`,j,A,e,t),f=u(`Alert`,o,t),p=n(()=>{let{common:{cubicBezierEaseInOut:t},self:n}=s.value,{fontSize:r,borderRadius:i,titleFontWeight:a,lineHeight:o,iconSize:c,iconMargin:l,iconMarginRtl:u,closeIconSize:f,closeBorderRadius:p,closeSize:h,closeMargin:g,closeMarginRtl:_,padding:v}=n,{type:y}=e,{left:b,right:x}=d(l);return{"--n-bezier":t,"--n-color":n[m(`color`,y)],"--n-close-icon-size":f,"--n-close-border-radius":p,"--n-close-color-hover":n[m(`closeColorHover`,y)],"--n-close-color-pressed":n[m(`closeColorPressed`,y)],"--n-close-icon-color":n[m(`closeIconColor`,y)],"--n-close-icon-color-hover":n[m(`closeIconColorHover`,y)],"--n-close-icon-color-pressed":n[m(`closeIconColorPressed`,y)],"--n-icon-color":n[m(`iconColor`,y)],"--n-border":n[m(`border`,y)],"--n-title-text-color":n[m(`titleTextColor`,y)],"--n-content-text-color":n[m(`contentTextColor`,y)],"--n-line-height":o,"--n-border-radius":i,"--n-font-size":r,"--n-title-font-weight":a,"--n-icon-size":c,"--n-icon-margin":l,"--n-icon-margin-rtl":u,"--n-close-size":h,"--n-close-margin":g,"--n-close-margin-rtl":_,"--n-padding":v,"--n-icon-margin-left":b,"--n-icon-margin-right":x}}),h=a?c(`alert`,n(()=>e.type[0]),p,e):void 0,g=i(!0),_=()=>{let{onAfterLeave:t,onAfterHide:n}=e;t&&t(),n&&n()};return{rtlEnabled:f,mergedClsPrefix:t,mergedBordered:r,visible:g,handleCloseClick:()=>{Promise.resolve(e.onClose?.call(e)).then(e=>{e!==!1&&(g.value=!1)})},handleAfterLeave:()=>{_()},mergedTheme:s,cssVars:a?void 0:p,themeClass:h?.themeClass,onRender:h?.onRender}},render(){var e;return(e=this.onRender)==null||e.call(this),r(x,{onAfterLeave:this.handleAfterLeave},{default:()=>{let{mergedClsPrefix:e,$slots:n}=this,i={class:[`${e}-alert`,this.themeClass,this.closable&&`${e}-alert--closable`,this.showIcon&&`${e}-alert--show-icon`,!this.title&&this.closable&&`${e}-alert--right-adjust`,this.rtlEnabled&&`${e}-alert--rtl`],style:this.cssVars,role:`alert`};return this.visible?r(`div`,Object.assign({},t(this.$attrs,i)),this.closable&&r(b,{clsPrefix:e,class:`${e}-alert__close`,onClick:this.handleCloseClick}),this.bordered&&r(`div`,{class:`${e}-alert__border`}),this.showIcon&&r(`div`,{class:`${e}-alert__icon`,"aria-hidden":`true`},_(n.icon,()=>[r(o,{clsPrefix:e},{default:()=>{switch(this.type){case`success`:return r(T,null);case`info`:return r(v,null);case`warning`:return r(C,null);case`error`:return r(w,null);default:return null}}})])),r(`div`,{class:[`${e}-alert-body`,this.mergedBordered&&`${e}-alert-body--bordered`]},g(n.header,t=>{let n=t||this.title;return n?r(`div`,{class:`${e}-alert-body__title`},n):null}),n.default&&r(`div`,{class:`${e}-alert-body__content`},n))):null}})}});export{M as t};