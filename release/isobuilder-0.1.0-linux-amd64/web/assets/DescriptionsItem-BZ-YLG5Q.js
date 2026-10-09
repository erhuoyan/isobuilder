import{E as e,g as t,k as n}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{Br as r,Er as i,Gn as a,Gr as o,Hr as s,Kn as c,Rr as l,Ur as u,Vr as d,Wr as f,hr as p,kn as m,nr as h,zr as g}from"./router-CtFNVkoW.js";import{n as _}from"./Space-DBH9f8i4.js";import{u as v}from"./index--SjCjhds.js";function y(e,t=`default`,n=[]){let{children:r}=e;if(typeof r==`object`&&r&&!Array.isArray(r)){let e=r[t];if(typeof e==`function`)return e()}return n}var b=l([g(`descriptions`,{fontSize:`var(--n-font-size)`},[g(`descriptions-separator`,`
 display: inline-block;
 margin: 0 8px 0 2px;
 `),g(`descriptions-table-wrapper`,[g(`descriptions-table`,[g(`descriptions-table-row`,[g(`descriptions-table-header`,{padding:`var(--n-th-padding)`}),g(`descriptions-table-content`,{padding:`var(--n-td-padding)`})])])]),s(`bordered`,[g(`descriptions-table-wrapper`,[g(`descriptions-table`,[g(`descriptions-table-row`,[l(`&:last-child`,[g(`descriptions-table-content`,{paddingBottom:0})])])])])]),d(`left-label-placement`,[g(`descriptions-table-content`,[l(`> *`,{verticalAlign:`top`})])]),d(`left-label-align`,[l(`th`,{textAlign:`left`})]),d(`center-label-align`,[l(`th`,{textAlign:`center`})]),d(`right-label-align`,[l(`th`,{textAlign:`right`})]),d(`bordered`,[g(`descriptions-table-wrapper`,`
 border-radius: var(--n-border-radius);
 overflow: hidden;
 background: var(--n-merged-td-color);
 border: 1px solid var(--n-merged-border-color);
 `,[g(`descriptions-table`,[g(`descriptions-table-row`,[l(`&:not(:last-child)`,[g(`descriptions-table-content`,{borderBottom:`1px solid var(--n-merged-border-color)`}),g(`descriptions-table-header`,{borderBottom:`1px solid var(--n-merged-border-color)`})]),g(`descriptions-table-header`,`
 font-weight: 400;
 background-clip: padding-box;
 background-color: var(--n-merged-th-color);
 `,[l(`&:not(:last-child)`,{borderRight:`1px solid var(--n-merged-border-color)`})]),g(`descriptions-table-content`,[l(`&:not(:last-child)`,{borderRight:`1px solid var(--n-merged-border-color)`})])])])])]),g(`descriptions-header`,`
 font-weight: var(--n-th-font-weight);
 font-size: 18px;
 transition: color .3s var(--n-bezier);
 line-height: var(--n-line-height);
 margin-bottom: 16px;
 color: var(--n-title-text-color);
 `),g(`descriptions-table-wrapper`,`
 transition:
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[g(`descriptions-table`,`
 width: 100%;
 border-collapse: separate;
 border-spacing: 0;
 box-sizing: border-box;
 `,[g(`descriptions-table-row`,`
 box-sizing: border-box;
 transition: border-color .3s var(--n-bezier);
 `,[g(`descriptions-table-header`,`
 font-weight: var(--n-th-font-weight);
 line-height: var(--n-line-height);
 display: table-cell;
 box-sizing: border-box;
 color: var(--n-th-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `),g(`descriptions-table-content`,`
 vertical-align: top;
 line-height: var(--n-line-height);
 display: table-cell;
 box-sizing: border-box;
 color: var(--n-td-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[r(`content`,`
 transition: color .3s var(--n-bezier);
 display: inline-block;
 color: var(--n-td-text-color);
 `)]),r(`label`,`
 font-weight: var(--n-th-font-weight);
 transition: color .3s var(--n-bezier);
 display: inline-block;
 margin-right: 14px;
 color: var(--n-th-text-color);
 `)])])])]),g(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color);
 --n-merged-td-color: var(--n-td-color);
 --n-merged-border-color: var(--n-border-color);
 `),f(g(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color-modal);
 --n-merged-td-color: var(--n-td-color-modal);
 --n-merged-border-color: var(--n-border-color-modal);
 `)),o(g(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color-popover);
 --n-merged-td-color: var(--n-td-color-popover);
 --n-merged-border-color: var(--n-border-color-popover);
 `))]),x=`DESCRIPTION_ITEM_FLAG`;function S(e){return typeof e==`object`&&e&&!Array.isArray(e)?e.type&&e.type.DESCRIPTION_ITEM_FLAG:!1}var C=e({name:`Descriptions`,props:Object.assign(Object.assign({},m.props),{title:String,column:{type:Number,default:3},columns:Number,labelPlacement:{type:String,default:`top`},labelAlign:{type:String,default:`left`},separator:{type:String,default:`:`},size:String,bordered:Boolean,labelClass:String,labelStyle:[Object,String],contentClass:String,contentStyle:[Object,String]}),slots:Object,setup(e){let{mergedClsPrefixRef:n,inlineThemeDisabled:r,mergedComponentPropsRef:i}=c(e),o=t(()=>e.size||i?.value?.Descriptions?.size||`medium`),s=m(`Descriptions`,`-descriptions`,b,v,e,n),l=t(()=>{let{bordered:t}=e,n=o.value,{common:{cubicBezierEaseInOut:r},self:{titleTextColor:i,thColor:a,thColorModal:c,thColorPopover:l,thTextColor:d,thFontWeight:f,tdTextColor:p,tdColor:m,tdColorModal:h,tdColorPopover:g,borderColor:_,borderColorModal:v,borderColorPopover:y,borderRadius:b,lineHeight:x,[u(`fontSize`,n)]:S,[u(t?`thPaddingBordered`:`thPadding`,n)]:C,[u(t?`tdPaddingBordered`:`tdPadding`,n)]:w}}=s.value;return{"--n-title-text-color":i,"--n-th-padding":C,"--n-td-padding":w,"--n-font-size":S,"--n-bezier":r,"--n-th-font-weight":f,"--n-line-height":x,"--n-th-text-color":d,"--n-td-text-color":p,"--n-th-color":a,"--n-th-color-modal":c,"--n-th-color-popover":l,"--n-td-color":m,"--n-td-color-modal":h,"--n-td-color-popover":g,"--n-border-radius":b,"--n-border-color":_,"--n-border-color-modal":v,"--n-border-color-popover":y}}),d=r?a(`descriptions`,t(()=>{let t=``,{bordered:n}=e;return n&&(t+=`a`),t+=o.value[0],t}),l,e):void 0;return{mergedClsPrefix:n,cssVars:r?void 0:l,themeClass:d?.themeClass,onRender:d?.onRender,compitableColumn:p(e,[`columns`,`column`]),inlineThemeDisabled:r,mergedSize:o}},render(){let e=this.$slots.default,t=e?h(e()):[];t.length;let{contentClass:r,labelClass:a,compitableColumn:o,labelPlacement:s,labelAlign:c,mergedSize:l,bordered:u,title:d,cssVars:f,mergedClsPrefix:p,separator:m,onRender:g}=this;g?.();let v=t.filter(e=>S(e)),b=v.reduce((e,t,i)=>{let c=t.props||{},l=v.length-1===i,d=[`label`in c?c.label:y(t,`label`)],f=[y(t)],h=c.span||1,g=e.span;e.span+=h;let _=c.labelStyle||c[`label-style`]||this.labelStyle,b=c.contentStyle||c[`content-style`]||this.contentStyle;if(s===`left`)u?e.row.push(n(`th`,{class:[`${p}-descriptions-table-header`,a],colspan:1,style:_},d),n(`td`,{class:[`${p}-descriptions-table-content`,r],colspan:l?(o-g)*2+1:h*2-1,style:b},f)):e.row.push(n(`td`,{class:`${p}-descriptions-table-content`,colspan:l?(o-g)*2:h*2},n(`span`,{class:[`${p}-descriptions-table-content__label`,a],style:_},[...d,m&&n(`span`,{class:`${p}-descriptions-separator`},m)]),n(`span`,{class:[`${p}-descriptions-table-content__content`,r],style:b},f)));else{let t=l?(o-g)*2:h*2;e.row.push(n(`th`,{class:[`${p}-descriptions-table-header`,a],colspan:t,style:_},d)),e.secondRow.push(n(`td`,{class:[`${p}-descriptions-table-content`,r],colspan:t,style:b},f))}return(e.span>=o||l)&&(e.span=0,e.row.length&&(e.rows.push(e.row),e.row=[]),s!==`left`&&e.secondRow.length&&(e.rows.push(e.secondRow),e.secondRow=[])),e},{span:0,row:[],secondRow:[],rows:[]}).rows.map(e=>n(`tr`,{class:`${p}-descriptions-table-row`},e));return n(`div`,{style:f,class:[`${p}-descriptions`,this.themeClass,`${p}-descriptions--${s}-label-placement`,`${p}-descriptions--${c}-label-align`,`${p}-descriptions--${l}-size`,u&&`${p}-descriptions--bordered`]},d||this.$slots.header?n(`div`,{class:`${p}-descriptions-header`},d||_(this,`header`)):null,n(`div`,{class:`${p}-descriptions-table-wrapper`},n(`table`,{class:`${p}-descriptions-table`},n(`tbody`,null,s===`top`&&n(`tr`,{class:`${p}-descriptions-table-row`,style:{visibility:`collapse`}},i(o*2,n(`td`,null))),b))))}}),w={label:String,span:{type:Number,default:1},labelClass:String,labelStyle:[Object,String],contentClass:String,contentStyle:[Object,String]},T=e({name:`DescriptionsItem`,[x]:!0,props:w,slots:Object,render(){return null}});export{C as n,T as t};