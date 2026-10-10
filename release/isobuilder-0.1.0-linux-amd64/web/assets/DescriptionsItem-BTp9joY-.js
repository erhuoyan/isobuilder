import{E as e,g as t,k as n}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{An as r,Br as i,Dr as a,Gr as o,Hr as s,Kn as c,Kr as l,Ur as u,Vr as d,Wr as f,gr as p,qn as m,rr as h,zr as g}from"./router-nu8PVV-0.js";import{n as _}from"./Space-CAsbN1s2.js";import{u as v}from"./index-DaPKSVAL.js";function y(e,t=`default`,n=[]){let{children:r}=e;if(typeof r==`object`&&r&&!Array.isArray(r)){let e=r[t];if(typeof e==`function`)return e()}return n}var b=g([i(`descriptions`,{fontSize:`var(--n-font-size)`},[i(`descriptions-separator`,`
 display: inline-block;
 margin: 0 8px 0 2px;
 `),i(`descriptions-table-wrapper`,[i(`descriptions-table`,[i(`descriptions-table-row`,[i(`descriptions-table-header`,{padding:`var(--n-th-padding)`}),i(`descriptions-table-content`,{padding:`var(--n-td-padding)`})])])]),u(`bordered`,[i(`descriptions-table-wrapper`,[i(`descriptions-table`,[i(`descriptions-table-row`,[g(`&:last-child`,[i(`descriptions-table-content`,{paddingBottom:0})])])])])]),s(`left-label-placement`,[i(`descriptions-table-content`,[g(`> *`,{verticalAlign:`top`})])]),s(`left-label-align`,[g(`th`,{textAlign:`left`})]),s(`center-label-align`,[g(`th`,{textAlign:`center`})]),s(`right-label-align`,[g(`th`,{textAlign:`right`})]),s(`bordered`,[i(`descriptions-table-wrapper`,`
 border-radius: var(--n-border-radius);
 overflow: hidden;
 background: var(--n-merged-td-color);
 border: 1px solid var(--n-merged-border-color);
 `,[i(`descriptions-table`,[i(`descriptions-table-row`,[g(`&:not(:last-child)`,[i(`descriptions-table-content`,{borderBottom:`1px solid var(--n-merged-border-color)`}),i(`descriptions-table-header`,{borderBottom:`1px solid var(--n-merged-border-color)`})]),i(`descriptions-table-header`,`
 font-weight: 400;
 background-clip: padding-box;
 background-color: var(--n-merged-th-color);
 `,[g(`&:not(:last-child)`,{borderRight:`1px solid var(--n-merged-border-color)`})]),i(`descriptions-table-content`,[g(`&:not(:last-child)`,{borderRight:`1px solid var(--n-merged-border-color)`})])])])])]),i(`descriptions-header`,`
 font-weight: var(--n-th-font-weight);
 font-size: 18px;
 transition: color .3s var(--n-bezier);
 line-height: var(--n-line-height);
 margin-bottom: 16px;
 color: var(--n-title-text-color);
 `),i(`descriptions-table-wrapper`,`
 transition:
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[i(`descriptions-table`,`
 width: 100%;
 border-collapse: separate;
 border-spacing: 0;
 box-sizing: border-box;
 `,[i(`descriptions-table-row`,`
 box-sizing: border-box;
 transition: border-color .3s var(--n-bezier);
 `,[i(`descriptions-table-header`,`
 font-weight: var(--n-th-font-weight);
 line-height: var(--n-line-height);
 display: table-cell;
 box-sizing: border-box;
 color: var(--n-th-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `),i(`descriptions-table-content`,`
 vertical-align: top;
 line-height: var(--n-line-height);
 display: table-cell;
 box-sizing: border-box;
 color: var(--n-td-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[d(`content`,`
 transition: color .3s var(--n-bezier);
 display: inline-block;
 color: var(--n-td-text-color);
 `)]),d(`label`,`
 font-weight: var(--n-th-font-weight);
 transition: color .3s var(--n-bezier);
 display: inline-block;
 margin-right: 14px;
 color: var(--n-th-text-color);
 `)])])])]),i(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color);
 --n-merged-td-color: var(--n-td-color);
 --n-merged-border-color: var(--n-border-color);
 `),o(i(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color-modal);
 --n-merged-td-color: var(--n-td-color-modal);
 --n-merged-border-color: var(--n-border-color-modal);
 `)),l(i(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color-popover);
 --n-merged-td-color: var(--n-td-color-popover);
 --n-merged-border-color: var(--n-border-color-popover);
 `))]),x=`DESCRIPTION_ITEM_FLAG`;function S(e){return typeof e==`object`&&e&&!Array.isArray(e)?e.type&&e.type.DESCRIPTION_ITEM_FLAG:!1}var C=e({name:`Descriptions`,props:Object.assign(Object.assign({},r.props),{title:String,column:{type:Number,default:3},columns:Number,labelPlacement:{type:String,default:`top`},labelAlign:{type:String,default:`left`},separator:{type:String,default:`:`},size:String,bordered:Boolean,labelClass:String,labelStyle:[Object,String],contentClass:String,contentStyle:[Object,String]}),slots:Object,setup(e){let{mergedClsPrefixRef:n,inlineThemeDisabled:i,mergedComponentPropsRef:a}=m(e),o=t(()=>e.size||a?.value?.Descriptions?.size||`medium`),s=r(`Descriptions`,`-descriptions`,b,v,e,n),l=t(()=>{let{bordered:t}=e,n=o.value,{common:{cubicBezierEaseInOut:r},self:{titleTextColor:i,thColor:a,thColorModal:c,thColorPopover:l,thTextColor:u,thFontWeight:d,tdTextColor:p,tdColor:m,tdColorModal:h,tdColorPopover:g,borderColor:_,borderColorModal:v,borderColorPopover:y,borderRadius:b,lineHeight:x,[f(`fontSize`,n)]:S,[f(t?`thPaddingBordered`:`thPadding`,n)]:C,[f(t?`tdPaddingBordered`:`tdPadding`,n)]:w}}=s.value;return{"--n-title-text-color":i,"--n-th-padding":C,"--n-td-padding":w,"--n-font-size":S,"--n-bezier":r,"--n-th-font-weight":d,"--n-line-height":x,"--n-th-text-color":u,"--n-td-text-color":p,"--n-th-color":a,"--n-th-color-modal":c,"--n-th-color-popover":l,"--n-td-color":m,"--n-td-color-modal":h,"--n-td-color-popover":g,"--n-border-radius":b,"--n-border-color":_,"--n-border-color-modal":v,"--n-border-color-popover":y}}),u=i?c(`descriptions`,t(()=>{let t=``,{bordered:n}=e;return n&&(t+=`a`),t+=o.value[0],t}),l,e):void 0;return{mergedClsPrefix:n,cssVars:i?void 0:l,themeClass:u?.themeClass,onRender:u?.onRender,compitableColumn:p(e,[`columns`,`column`]),inlineThemeDisabled:i,mergedSize:o}},render(){let e=this.$slots.default,t=e?h(e()):[];t.length;let{contentClass:r,labelClass:i,compitableColumn:o,labelPlacement:s,labelAlign:c,mergedSize:l,bordered:u,title:d,cssVars:f,mergedClsPrefix:p,separator:m,onRender:g}=this;g?.();let v=t.filter(e=>S(e)),b=v.reduce((e,t,a)=>{let c=t.props||{},l=v.length-1===a,d=[`label`in c?c.label:y(t,`label`)],f=[y(t)],h=c.span||1,g=e.span;e.span+=h;let _=c.labelStyle||c[`label-style`]||this.labelStyle,b=c.contentStyle||c[`content-style`]||this.contentStyle;if(s===`left`)u?e.row.push(n(`th`,{class:[`${p}-descriptions-table-header`,i],colspan:1,style:_},d),n(`td`,{class:[`${p}-descriptions-table-content`,r],colspan:l?(o-g)*2+1:h*2-1,style:b},f)):e.row.push(n(`td`,{class:`${p}-descriptions-table-content`,colspan:l?(o-g)*2:h*2},n(`span`,{class:[`${p}-descriptions-table-content__label`,i],style:_},[...d,m&&n(`span`,{class:`${p}-descriptions-separator`},m)]),n(`span`,{class:[`${p}-descriptions-table-content__content`,r],style:b},f)));else{let t=l?(o-g)*2:h*2;e.row.push(n(`th`,{class:[`${p}-descriptions-table-header`,i],colspan:t,style:_},d)),e.secondRow.push(n(`td`,{class:[`${p}-descriptions-table-content`,r],colspan:t,style:b},f))}return(e.span>=o||l)&&(e.span=0,e.row.length&&(e.rows.push(e.row),e.row=[]),s!==`left`&&e.secondRow.length&&(e.rows.push(e.secondRow),e.secondRow=[])),e},{span:0,row:[],secondRow:[],rows:[]}).rows.map(e=>n(`tr`,{class:`${p}-descriptions-table-row`},e));return n(`div`,{style:f,class:[`${p}-descriptions`,this.themeClass,`${p}-descriptions--${s}-label-placement`,`${p}-descriptions--${c}-label-align`,`${p}-descriptions--${l}-size`,u&&`${p}-descriptions--bordered`]},d||this.$slots.header?n(`div`,{class:`${p}-descriptions-header`},d||_(this,`header`)):null,n(`div`,{class:`${p}-descriptions-table-wrapper`},n(`table`,{class:`${p}-descriptions-table`},n(`tbody`,null,s===`top`&&n(`tr`,{class:`${p}-descriptions-table-row`,style:{visibility:`collapse`}},a(o*2,n(`td`,null))),b))))}}),w={label:String,span:{type:Number,default:1},labelClass:String,labelStyle:[Object,String],contentClass:String,contentStyle:[Object,String]},T=e({name:`DescriptionsItem`,[x]:!0,props:w,slots:Object,render(){return null}});export{C as n,T as t};