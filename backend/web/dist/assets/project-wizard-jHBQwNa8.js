import{$ as e,B as t,C as n,E as r,H as i,N as a,St as o,U as s,W as c,Y as l,Z as u,_ as d,b as f,d as p,g as m,ht as h,j as g,k as _,l as v,ut as y,v as b,vt as x,w as S,y as C,yt as w}from"./vue.runtime.esm-bundler-CoGLJnd5.js";import{Br as T,C as E,Dn as D,E as O,En as k,G as ee,Gn as A,Gr as j,Hr as M,It as te,J as N,Jt as ne,Kn as P,M as F,Mn as I,Mt as re,Ot as ie,Q as L,Rr as R,Rt as ae,S as oe,Sn as z,T as se,Tn as ce,U as B,Ur as V,Ut as le,Vr as H,Wr as ue,Xn as de,Yn as U,_ as fe,_n as W,ar as G,ct as pe,f as me,in as he,ir as K,kn as q,lt as ge,m as _e,mr as ve,nr as ye,s as be,x as xe,y as Se,zr as J}from"./router-02wxpyS2.js";import{i as Ce,t as we}from"./FormItem-DxDFUXMN.js";import{n as Te}from"./DataTable-XybOxN16.js";import{n as Ee,t as De}from"./Space-rybSX4Ej.js";import{t as Oe}from"./Alert-V1rUyZGe.js";import{n as ke,t as Ae}from"./iso-status-tag-VAxNIF0M.js";import{t as je}from"./Spin-Dun24B_r.js";import{s as Me,t as Ne}from"./index-Bn76MJ2O.js";import{a as Pe,c as Fe,l as Ie,t as Le}from"./format-EVqN22hl.js";import{t as Re}from"./copy-text-PkfqpHzp.js";import{t as ze}from"./host-summary-table-CcjGQbqL.js";var Be=R([J(`list`,`
 --n-merged-border-color: var(--n-border-color);
 --n-merged-color: var(--n-color);
 --n-merged-color-hover: var(--n-color-hover);
 margin: 0;
 font-size: var(--n-font-size);
 transition:
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 padding: 0;
 list-style-type: none;
 color: var(--n-text-color);
 background-color: var(--n-merged-color);
 `,[H(`show-divider`,[J(`list-item`,[R(`&:not(:last-child)`,[T(`divider`,`
 background-color: var(--n-merged-border-color);
 `)])])]),H(`clickable`,[J(`list-item`,`
 cursor: pointer;
 `)]),H(`bordered`,`
 border: 1px solid var(--n-merged-border-color);
 border-radius: var(--n-border-radius);
 `),H(`hoverable`,[J(`list-item`,`
 border-radius: var(--n-border-radius);
 `,[R(`&:hover`,`
 background-color: var(--n-merged-color-hover);
 `,[T(`divider`,`
 background-color: transparent;
 `)])])]),H(`bordered, hoverable`,[J(`list-item`,`
 padding: 12px 20px;
 `),T(`header, footer`,`
 padding: 12px 20px;
 `)]),T(`header, footer`,`
 padding: 12px 0;
 box-sizing: border-box;
 transition: border-color .3s var(--n-bezier);
 `,[R(`&:not(:last-child)`,`
 border-bottom: 1px solid var(--n-merged-border-color);
 `)]),J(`list-item`,`
 position: relative;
 padding: 12px 0; 
 box-sizing: border-box;
 display: flex;
 flex-wrap: nowrap;
 align-items: center;
 transition:
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[T(`prefix`,`
 margin-right: 20px;
 flex: 0;
 `),T(`suffix`,`
 margin-left: 20px;
 flex: 0;
 `),T(`main`,`
 flex: 1;
 `),T(`divider`,`
 height: 1px;
 position: absolute;
 bottom: 0;
 left: 0;
 right: 0;
 background-color: transparent;
 transition: background-color .3s var(--n-bezier);
 pointer-events: none;
 `)])]),ue(J(`list`,`
 --n-merged-color-hover: var(--n-color-hover-modal);
 --n-merged-color: var(--n-color-modal);
 --n-merged-border-color: var(--n-border-color-modal);
 `)),j(J(`list`,`
 --n-merged-color-hover: var(--n-color-hover-popover);
 --n-merged-color: var(--n-color-popover);
 --n-merged-border-color: var(--n-border-color-popover);
 `))]),Ve=Object.assign(Object.assign({},q.props),{size:{type:String,default:`medium`},bordered:Boolean,clickable:Boolean,hoverable:Boolean,showDivider:{type:Boolean,default:!0}}),He=ve(`n-list`),Ue=r({name:`List`,props:Ve,slots:Object,setup(e){let{mergedClsPrefixRef:t,inlineThemeDisabled:n,mergedRtlRef:r}=P(e),i=I(`List`,r,t),a=q(`List`,`-list`,Be,Me,e,t);s(He,{showDividerRef:h(e,`showDivider`),mergedClsPrefixRef:t});let o=m(()=>{let{common:{cubicBezierEaseInOut:e},self:{fontSize:t,textColor:n,color:r,colorModal:i,colorPopover:o,borderColor:s,borderColorModal:c,borderColorPopover:l,borderRadius:u,colorHover:d,colorHoverModal:f,colorHoverPopover:p}}=a.value;return{"--n-font-size":t,"--n-bezier":e,"--n-text-color":n,"--n-color":r,"--n-border-radius":u,"--n-border-color":s,"--n-border-color-modal":c,"--n-border-color-popover":l,"--n-color-modal":i,"--n-color-popover":o,"--n-color-hover":d,"--n-color-hover-modal":f,"--n-color-hover-popover":p}}),c=n?A(`list`,void 0,o,e):void 0;return{mergedClsPrefix:t,rtlEnabled:i,cssVars:n?void 0:o,themeClass:c?.themeClass,onRender:c?.onRender}},render(){let{$slots:e,mergedClsPrefix:t,onRender:n}=this;return n?.(),_(`ul`,{class:[`${t}-list`,this.rtlEnabled&&`${t}-list--rtl`,this.bordered&&`${t}-list--bordered`,this.showDivider&&`${t}-list--show-divider`,this.hoverable&&`${t}-list--hoverable`,this.clickable&&`${t}-list--clickable`,this.themeClass],style:this.cssVars},e.header?_(`div`,{class:`${t}-list__header`},e.header()):null,e.default?.call(e),e.footer?_(`div`,{class:`${t}-list__footer`},e.footer()):null)}}),We=r({name:`ListItem`,slots:Object,setup(){let e=g(He,null);return e||G(`list-item`,"`n-list-item` must be placed in `n-list`."),{showDivider:e.showDividerRef,mergedClsPrefix:e.mergedClsPrefixRef}},render(){let{$slots:e,mergedClsPrefix:t}=this;return _(`li`,{class:`${t}-list-item`},e.prefix?_(`div`,{class:`${t}-list-item__prefix`},e.prefix()):null,e.default?_(`div`,{class:`${t}-list-item__main`},e):null,e.suffix?_(`div`,{class:`${t}-list-item__suffix`},e.suffix()):null,this.showDivider&&_(`div`,{class:`${t}-list-item__divider`}))}}),Ge=J(`steps`,`
 width: 100%;
 display: flex;
`,[J(`step`,`
 position: relative;
 display: flex;
 flex: 1;
 `,[H(`disabled`,`cursor: not-allowed`),H(`clickable`,`
 cursor: pointer;
 `),R(`&:last-child`,[J(`step-splitor`,`display: none;`)])]),J(`step-splitor`,`
 background-color: var(--n-splitor-color);
 margin-top: calc(var(--n-step-header-font-size) / 2);
 height: 1px;
 flex: 1;
 align-self: flex-start;
 margin-left: 12px;
 margin-right: 12px;
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `),J(`step-content`,`flex: 1;`,[J(`step-content-header`,`
 color: var(--n-header-text-color);
 margin-top: calc(var(--n-indicator-size) / 2 - var(--n-step-header-font-size) / 2);
 line-height: var(--n-step-header-font-size);
 font-size: var(--n-step-header-font-size);
 position: relative;
 display: flex;
 font-weight: var(--n-step-header-font-weight);
 margin-left: 9px;
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `,[T(`title`,`
 white-space: nowrap;
 flex: 0;
 `)]),T(`description`,`
 color: var(--n-description-text-color);
 margin-top: 12px;
 margin-left: 9px;
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `)]),J(`step-indicator`,`
 background-color: var(--n-indicator-color);
 box-shadow: 0 0 0 1px var(--n-indicator-border-color);
 height: var(--n-indicator-size);
 width: var(--n-indicator-size);
 border-radius: 50%;
 display: flex;
 align-items: center;
 justify-content: center;
 transition:
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
 `,[J(`step-indicator-slot`,`
 position: relative;
 width: var(--n-indicator-icon-size);
 height: var(--n-indicator-icon-size);
 font-size: var(--n-indicator-icon-size);
 line-height: var(--n-indicator-icon-size);
 `,[T(`index`,`
 display: inline-block;
 text-align: center;
 position: absolute;
 left: 0;
 top: 0;
 white-space: nowrap;
 font-size: var(--n-indicator-index-font-size);
 width: var(--n-indicator-icon-size);
 height: var(--n-indicator-icon-size);
 line-height: var(--n-indicator-icon-size);
 color: var(--n-indicator-text-color);
 transition: color .3s var(--n-bezier);
 `,[W()]),J(`icon`,`
 color: var(--n-indicator-text-color);
 transition: color .3s var(--n-bezier);
 `,[W()]),J(`base-icon`,`
 color: var(--n-indicator-text-color);
 transition: color .3s var(--n-bezier);
 `,[W()])])]),H(`vertical`,`flex-direction: column;`,[M(`show-description`,[R(`>`,[J(`step`,`padding-bottom: 8px;`)])]),R(`>`,[J(`step`,`margin-bottom: 16px;`,[R(`&:last-child`,`margin-bottom: 0;`),R(`>`,[J(`step-indicator`,[R(`>`,[J(`step-splitor`,`
 position: absolute;
 bottom: -8px;
 width: 1px;
 margin: 0 !important;
 left: calc(var(--n-indicator-size) / 2);
 height: calc(100% - var(--n-indicator-size));
 `)])]),J(`step-content`,[T(`description`,`margin-top: 8px;`)])])])])]),H(`content-bottom`,[M(`vertical`,[R(`>`,[J(`step`,`flex-direction: column`,[R(`>`,[J(`step-line`,`display: flex;`,[R(`>`,[J(`step-splitor`,`
 margin-top: 0;
 align-self: center;
 `)])])]),R(`>`,[J(`step-content`,`margin-top: calc(var(--n-indicator-size) / 2 - var(--n-step-header-font-size) / 2);`,[J(`step-content-header`,`
 margin-left: 0;
 `),J(`step-content__description`,`
 margin-left: 0;
 `)])])])])])])]);function Ke(e,t){return typeof e!=`object`||!e||Array.isArray(e)?null:(e.props||={},e.props.internalIndex=t+1,e)}function qe(e){return e.map((e,t)=>Ke(e,t))}var Je=Object.assign(Object.assign({},q.props),{current:Number,status:{type:String,default:`process`},size:{type:String,default:`medium`},vertical:Boolean,contentPlacement:{type:String,default:`right`},"onUpdate:current":[Function,Array],onUpdateCurrent:[Function,Array]}),Y=ve(`n-steps`),Ye=r({name:`Steps`,props:Je,slots:Object,setup(e,{slots:t}){let{mergedClsPrefixRef:n,mergedRtlRef:r}=P(e),i=I(`Steps`,r,n);return s(Y,{props:e,mergedThemeRef:q(`Steps`,`-steps`,Ge,Ne,e,n),mergedClsPrefixRef:n,stepsSlots:t}),{mergedClsPrefix:n,rtlEnabled:i}},render(){let{mergedClsPrefix:e}=this;return _(`div`,{class:[`${e}-steps`,this.rtlEnabled&&`${e}-steps--rtl`,this.vertical&&`${e}-steps--vertical`,this.contentPlacement===`bottom`&&`${e}-steps--content-bottom`]},qe(ye(Ee(this))))}}),Xe=r({name:`Step`,props:{status:String,title:String,description:String,disabled:Boolean,internalIndex:{type:Number,default:0}},slots:Object,setup(e){let t=g(Y,null);t||G(`step`,"`n-step` must be placed inside `n-steps`.");let{inlineThemeDisabled:n}=P(),{props:r,mergedThemeRef:i,mergedClsPrefixRef:a,stepsSlots:o}=t,s=h(r,`vertical`),c=h(r,`contentPlacement`),l=m(()=>{let{status:t}=e;if(t)return t;{let{internalIndex:t}=e,{current:n}=r;if(n===void 0)return`process`;if(t<n)return`finish`;if(t===n)return r.status||`process`;if(t>n)return`wait`}return`process`}),u=m(()=>{let{value:e}=l,{size:t}=r,{common:{cubicBezierEaseInOut:n},self:{stepHeaderFontWeight:a,[V(`stepHeaderFontSize`,t)]:o,[V(`indicatorIndexFontSize`,t)]:s,[V(`indicatorSize`,t)]:c,[V(`indicatorIconSize`,t)]:u,[V(`indicatorTextColor`,e)]:d,[V(`indicatorBorderColor`,e)]:f,[V(`headerTextColor`,e)]:p,[V(`splitorColor`,e)]:m,[V(`indicatorColor`,e)]:h,[V(`descriptionTextColor`,e)]:g}}=i.value;return{"--n-bezier":n,"--n-description-text-color":g,"--n-header-text-color":p,"--n-indicator-border-color":f,"--n-indicator-color":h,"--n-indicator-icon-size":u,"--n-indicator-index-font-size":s,"--n-indicator-size":c,"--n-indicator-text-color":d,"--n-splitor-color":m,"--n-step-header-font-size":o,"--n-step-header-font-weight":a}}),d=n?A(`step`,m(()=>{let{value:e}=l,{size:t}=r;return`${e[0]}${t[0]}`}),u,r):void 0;return{stepsSlots:o,mergedClsPrefix:a,vertical:s,mergedStatus:l,handleStepClick:m(()=>{if(e.disabled)return;let{onUpdateCurrent:t,"onUpdate:current":n}=r;return t||n?()=>{t&&K(t,e.internalIndex),n&&K(n,e.internalIndex)}:void 0}),cssVars:n?void 0:u,themeClass:d?.themeClass,onRender:d?.onRender,contentPlacement:c}},render(){let{mergedClsPrefix:e,onRender:t,handleStepClick:n,disabled:r,contentPlacement:i,vertical:a}=this,o=de(this.$slots.default,t=>{let n=t||this.description;return n?_(`div`,{class:`${e}-step-content__description`},n):null}),s=_(`div`,{class:`${e}-step-splitor`}),c=_(`div`,{class:`${e}-step-indicator`,key:i},_(`div`,{class:`${e}-step-indicator-slot`},_(k,null,{default:()=>de(this.$slots.icon,t=>{let{mergedStatus:n,stepsSlots:r}=this;return n===`finish`||n===`error`?n===`finish`?_(D,{clsPrefix:e,key:`finish`},{default:()=>U(r[`finish-icon`],()=>[_(ce,null)])}):n===`error`?_(D,{clsPrefix:e,key:`error`},{default:()=>U(r[`error-icon`],()=>[_(z,null)])}):null:t||_(`div`,{key:this.internalIndex,class:`${e}-step-indicator-slot__index`},this.internalIndex)})})),a?s:null),l=_(`div`,{class:`${e}-step-content`},_(`div`,{class:`${e}-step-content-header`},_(`div`,{class:`${e}-step-content-header__title`},U(this.$slots.title,()=>[this.title])),!a&&i===`right`?s:null),o),u;return u=!a&&i===`bottom`?_(p,null,_(`div`,{class:`${e}-step-line`},c,s),l):_(p,null,c,l),t?.(),_(`div`,{class:[`${e}-step`,r&&`${e}-step--disabled`,!r&&n&&`${e}-step--clickable`,this.themeClass,o&&`${e}-step--show-description`,`${e}-step--${this.mergedStatus}-status`],style:this.cssVars,onClick:n},u)}}),X={class:`flex flex-col gap-12px`},Ze={class:`flex flex-wrap items-center justify-between gap-8px`},Z={class:`font-mono`},Qe={class:`font-mono`},Q={class:`flex flex-wrap items-center gap-8px`},$={class:`error-text font-mono font-500`},$e={key:0,class:`error-text font-mono text-12px`},et={class:`error-text`},tt={key:7,class:`flex flex-col gap-8px`},nt={class:`text-13px text-gray-500`},rt={class:`font-mono`},it=500,at=`hosts:
  - sn: SN0000000
    hostname: node-01
    network:
      ip: 172.16.0.11
      netmask: 255.255.255.0
      gateway: 172.16.0.1
      mac: 00:11:22:33:44:55
    bmc_ip: 10.0.0.11
  - sn: SN0000001
    hostname: node-02
    network:
      ip: 172.16.0.12
      prefix: 24
      bond:
        mode: 4
        slaves:
          - 00:11:22:33:44:66
          - 00:11:22:33:44:67`,ot=be(r({__name:`hosts-editor`,props:{value:{required:!0},valueModifiers:{},format:{required:!0},formatModifiers:{}},emits:a([`validate`],[`update:value`,`update:format`]),setup(t,{emit:r}){let a=l(t,`value`),s=l(t,`format`),h=r,g=y(null),_=y(!1),v=y(``),w=y(!1),T=y(null),D=0,O=m(()=>a.value.trim().length>0),k=m(()=>`${s.value}\u0000${a.value}`),ee=m(()=>O.value&&!_.value&&k.value!==v.value),A=m(()=>g.value?.errors.length??0);function j(){D+=1,v.value=``,_.value=!1,w.value=!1,h(`validate`,null)}async function M(){let e=a.value;if(!e.trim()){g.value=null,j();return}let t=k.value;if(t===v.value&&g.value){h(`validate`,g.value);return}let n=++D;_.value=!0;let{data:r,error:i}=await E(e,s.value);if(n===D){if(_.value=!1,i){v.value=``,g.value=null,w.value=!0,h(`validate`,null);return}v.value=t,g.value=r,h(`validate`,r)}}let te=L(M,it);u([a,s],()=>{if(!O.value){g.value=null,j();return}j(),te()},{immediate:!0});function N(){T.value?.click()}async function ne(e){let t=e.target,n=t.files?.[0];if(t.value=``,n)try{a.value=await n.text(),window.$message?.success(`已读入 ${n.name}`)}catch{window.$message?.error(`读取本地文件失败，请重试。`)}}function P(){a.value=``}function F(e){return e.index>=0?`第 ${e.index+1} 台`:`清单级`}return(t,r)=>{let l=ke,u=Te,m=_e,h=ae,v=De,y=le,E=Oe,D=We,k=Ue,j=ze;return i(),f(`div`,X,[d(`div`,Ze,[S(u,{value:s.value,"onUpdate:value":r[0]||=e=>s.value=e,size:`small`},{default:e(()=>[(i(!0),f(p,null,c(x(Fe),t=>(i(),b(l,{key:t.value,value:t.value},{default:e(()=>[n(o(t.label),1)]),_:2},1032,[`value`]))),128))]),_:1},8,[`value`]),S(v,{size:8,wrap:!0},{default:e(()=>[S(h,{size:`small`,onClick:N},{icon:e(()=>[S(m,{icon:`mdi:file-upload-outline`})]),default:e(()=>[r[2]||=n(` 读入本地文件 `,-1)]),_:1}),S(h,{size:`small`,disabled:!O.value,onClick:P},{icon:e(()=>[S(m,{icon:`mdi:eraser`})]),default:e(()=>[r[3]||=n(` 清空 `,-1)]),_:1},8,[`disabled`]),S(h,{size:`small`,type:`primary`,loading:_.value,disabled:!O.value,onClick:M},{icon:e(()=>[S(m,{icon:`mdi:check-decagram-outline`})]),default:e(()=>[r[4]||=n(` 立即校验 `,-1)]),_:1},8,[`loading`,`disabled`])]),_:1})]),d(`input`,{ref_key:`fileInputRef`,ref:T,type:`file`,accept:`.yaml,.yml,.json,.txt`,class:`hidden`,onChange:ne},null,544),S(y,{value:a.value,"onUpdate:value":r[1]||=e=>a.value=e,type:`textarea`,rows:14,class:`font-mono`,"input-props":{class:`font-mono`,spellcheck:`false`},placeholder:at},null,8,[`value`]),O.value?_.value?(i(),b(E,{key:1,size:`small`,type:`info`,bordered:!1},{default:e(()=>[...r[6]||=[n(` 正在校验主机清单… `,-1)]]),_:1})):w.value?(i(),b(E,{key:2,size:`small`,type:`error`,bordered:!1},{default:e(()=>[...r[7]||=[n(` 校验请求失败，请检查清单格式后重试。 `,-1)]]),_:1})):ee.value?(i(),b(E,{key:3,size:`small`,type:`warning`,bordered:!1},{default:e(()=>[...r[8]||=[n(` 输入已改动，等待自动校验完成后再进入下一步。 `,-1)]]),_:1})):g.value?(i(),b(E,{key:4,size:`small`,type:g.value.ok?`success`:`error`,bordered:!1},{default:e(()=>[d(`span`,Z,o(g.value.hosts_count),1),r[9]||=n(` 台主机 · 识别格式 `,-1),d(`span`,Qe,o(g.value.format.toUpperCase()),1),n(` · `+o(g.value.ok?`校验通过`:`校验未通过，共 ${A.value} 处问题`),1)]),_:1},8,[`type`])):(i(),b(E,{key:5,size:`small`,type:`warning`,bordered:!1},{default:e(()=>[...r[10]||=[n(` 主机清单尚未校验。 `,-1)]]),_:1})):(i(),b(E,{key:0,size:`small`,type:`default`,bordered:!1,"show-icon":!1},{default:e(()=>[...r[5]||=[n(` 粘贴或读入主机清单（YAML / JSON），停止输入 500 毫秒后自动校验。 `,-1)]]),_:1})),g.value&&A.value?(i(),b(k,{key:6,bordered:``,size:`small`},{default:e(()=>[(i(!0),f(p,null,c(g.value.errors,(t,n)=>(i(),b(D,{key:`${t.index}-${t.field}-${n}`},{default:e(()=>[d(`div`,Q,[d(`span`,$,o(F(t)),1),t.field?(i(),f(`code`,$e,o(t.field),1)):C(``,!0),d(`span`,et,o(t.message),1)])]),_:2},1024))),128))]),_:1})):C(``,!0),g.value&&g.value.hosts_summary.length?(i(),f(`div`,tt,[d(`div`,nt,[r[11]||=n(` 解析结果（共 `,-1),d(`span`,rt,o(g.value.hosts_summary.length),1),r[12]||=n(` 台） `,-1)]),S(j,{hosts:g.value.hosts_summary,errors:g.value.errors,loading:_.value,"max-height":360},null,8,[`hosts`,`errors`,`loading`])])):C(``,!0)])}}}),[[`__scopeId`,`data-v-102496b2`]]),st={class:`flex flex-col gap-16px`},ct={class:`flex flex-wrap items-center justify-between gap-12px`},lt={class:`text-16px font-500`},ut={class:`mt-4px text-13px text-gray-500`},dt={class:`overflow-x-auto pb-4px`},ft={key:1,class:`flex flex-col gap-12px`},pt={class:`flex flex-wrap items-center justify-between gap-8px`},mt={key:1,class:`py-24px text-center`},ht={key:4,class:`grid grid-cols-1 gap-12px md:grid-cols-2 xl:grid-cols-3`},gt=[`aria-checked`,`aria-disabled`,`onClick`],_t={class:`flex items-start justify-between gap-8px`},vt={class:`min-w-0`},yt={class:`truncate text-14px font-500`},bt={class:`truncate text-12px font-mono text-gray-500`},xt={class:`flex flex-wrap items-center gap-8px text-12px text-gray-500`},St={class:`font-mono`},Ct={class:`font-mono`},wt={key:0,class:`text-12px text-warning`},Tt={class:`flex flex-wrap items-center justify-between gap-12px`},Et={class:`mt-4px text-12px text-gray-500`},Dt={key:2,class:`flex flex-col gap-12px`},Ot={key:0,class:`py-24px text-center`},kt={key:2,class:`grid grid-cols-1 gap-12px md:grid-cols-2`},At=[`aria-checked`,`aria-disabled`,`onClick`],jt={class:`flex items-start justify-between gap-8px`},Mt={class:`min-w-0`},Nt={class:`truncate text-14px font-500`},Pt={class:`mt-4px flex flex-wrap items-center gap-6px text-12px text-gray-500`},Ft={class:`font-mono`},It={key:0,class:`flex flex-wrap items-center gap-6px`},Lt={key:1,class:`text-12px text-gray-400`},Rt={key:2,class:`text-12px text-warning`},zt={key:3,class:`flex flex-col gap-12px`},Bt={class:`flex flex-wrap items-center justify-between gap-12px`},Vt={class:`min-h-240px`},Ht={key:0,class:`kickstart-preview font-mono text-12px`},Ut={key:0,class:`flex flex-col gap-12px`},Wt={class:`flex flex-col gap-8px`},Gt={class:`flex w-full flex-wrap items-center gap-8px`},Kt={class:`font-mono text-13px text-gray-500`},qt=64*1024*1024,Jt=be(r({__name:`project-wizard`,props:{projectId:{}},emits:[`load-error`],setup(r,{emit:a}){let s=r,l=a,{routerPushByKey:u}=N(),h=me(),g=[{key:`basic`,title:`基础信息`,description:`项目名称与目标架构`},{key:`iso`,title:`基础镜像`,description:`选用一个已就绪的 ISO`},{key:`kickstart`,title:`应答模板`,description:`选用 kickstart 模板`},{key:`hosts`,title:`主机清单`,description:`粘贴并校验主机信息`},{key:`advanced`,title:`高级项`,description:`ISO 卷标与产物文件名`}],_=[`.zip`,`.tar.gz`,`.tgz`,`.tar`],T=m(()=>!!s.projectId),E=y(0),D=m(()=>g[E.value]||g[0]),k=m(()=>h.isMobile?`100%`:720),A=y(!1),j=y(!1),M=y({name:``,description:``,arch:`x86_64`,iso_id:``,kickstart_id:``,iso_label:``,output_name:``}),P=y(``),I=y(`auto`),L=y(null),R=y([]),z=y([]),ce=y(!1),V=y(!1),H=y(!1),ue=m(()=>R.value.find(e=>e.id===M.value.iso_id)||null),de=m(()=>!!M.value.iso_id&&!ce.value&&!R.value.some(e=>e.id===M.value.iso_id)),U=m(()=>H.value?R.value:R.value.filter(e=>e.arch===M.value.arch||e.id===M.value.iso_id)),W=e=>e.status===`ready`,G=e=>e.arch===`any`||e.arch===M.value.arch,K=m(()=>P.value.trim()?L.value?L.value.ok?``:`主机清单存在 ${L.value.errors.length} 处问题，请修正后再继续。`:`主机清单尚未校验通过，请等待自动校验完成后重试。`:`请填写主机清单。`),q=m(()=>{switch(E.value){case 0:return!!M.value.name.trim();case 1:return!!M.value.iso_id&&(!ue.value||W(ue.value));case 2:return!!M.value.kickstart_id;case 3:return!K.value;default:return!0}}),ve=m(()=>!!M.value.name.trim()&&!!M.value.iso_id&&!!M.value.kickstart_id&&!K.value),ye=m(()=>{switch(E.value){case 0:return M.value.name.trim()?``:`请填写项目名称。`;case 1:return M.value.iso_id?``:`请选择一个状态为「可用」的基础镜像。`;case 2:return M.value.kickstart_id?``:`请选择一个 kickstart 模板。`;case 3:case 4:return K.value;default:return``}});function be(e){switch(e.status){case`uploading`:return`镜像仍在上传中，上传完成并校验通过后才能选用。`;case`hashing`:return`正在计算 SHA256，校验通过后才能选用。`;case`error`:return`镜像异常：${e.error||`原因未知`}`;default:return``}}function J(e){W(e)&&(M.value.iso_id=e.id)}function Ee(e){G(e)&&(M.value.kickstart_id=e.id)}function Me(e){L.value=e}function Ne(){E.value>0&&--E.value}function Fe(){!q.value||E.value>=g.length-1||(E.value+=1)}function ze(){u(`project-list`)}async function Be(){ce.value=!0;let{data:e,error:t}=await F();ce.value=!1,t||(R.value=e.isos)}async function Ve(){V.value=!0;let{data:e,error:t}=await O();V.value=!1,t||(z.value=e.kickstarts)}async function He(e){A.value=!0;let{data:t,error:n}=await Se(e);if(A.value=!1,n){l(`load-error`,n);return}let r=t.project;M.value={name:r.name,description:r.description,arch:r.arch,iso_id:r.iso_id,kickstart_id:r.kickstart_id,iso_label:r.iso_label,output_name:r.output_name},P.value=r.hosts_raw,I.value=r.hosts_format}async function Ue(){if(!ve.value)return;j.value=!0;let e={name:M.value.name.trim(),description:M.value.description.trim(),arch:M.value.arch,iso_id:M.value.iso_id,kickstart_id:M.value.kickstart_id,iso_label:M.value.iso_label.trim(),output_name:M.value.output_name.trim(),hosts_raw:P.value,hosts_format:I.value},{data:t,error:n}=s.projectId?await oe(s.projectId,e):await fe(e);j.value=!1,!n&&(window.$message?.success(T.value?`项目已保存`:`项目已创建`),u(`project-detail`,{params:{id:t.project.id}}))}let We=y(!1),Ge=y(null),Ke=y(``),qe=y(!1);async function Je(e){Ge.value=e,Ke.value=``,We.value=!0,qe.value=!0;let{data:t,error:n}=await se(e.id);qe.value=!1,!n&&(Ke.value=t.content)}let Y=y(!1),X=y(null),Ze=y(null),Z=y({name:``,description:``,arch:`x86_64`,iso_id:``,kickstart_id:``}),Qe=y(!1),Q=y(``),$=y(null),$e=y(null),et=m(()=>R.value.filter(e=>e.status===`ready`).map(e=>({label:`${e.name} · ${e.arch}`,value:e.id}))),tt=m(()=>z.value.map(e=>({label:`${e.name} · ${e.arch}`,value:e.id})));function nt(){let e=M.value;Z.value={name:e.name,description:e.description,arch:e.arch,iso_id:e.iso_id,kickstart_id:e.kickstart_id},X.value=null,Q.value=``,$.value=null,$e.value=null,Y.value=!0}function rt(){Ze.value?.click()}function it(e){let t=e.target,n=t.files?.[0];if(t.value=``,!n)return;let r=n.name.toLowerCase();if(!_.some(e=>r.endsWith(e))){window.$message?.error(`仅支持 .zip / .tar.gz / .tgz / .tar 格式的项目包。`);return}if(n.size>qt){window.$message?.error(`项目包不能超过 ${Le(qt)}（当前 ${Le(n.size)}）。`);return}X.value=n,Q.value=``}async function at(){if(!X.value){window.$message?.warning(`请先选择项目包文件。`);return}let e={},t=Z.value;t.name.trim()&&(e.name=t.name.trim()),t.description.trim()&&(e.description=t.description.trim()),t.arch&&(e.arch=t.arch),t.iso_id&&(e.iso_id=t.iso_id),t.kickstart_id&&(e.kickstart_id=t.kickstart_id),Qe.value=!0,Q.value=``;let{data:n,error:r}=await xe(X.value,e);if(Qe.value=!1,r){let e=B(r),t=ee(r);e===`archive_no_hosts`?Q.value=`项目包中没有找到可解析的主机清单（hosts*.yaml / hosts*.json）。${t}`:e===`invalid_archive`?Q.value=`项目包无法解析：${t}`:Q.value=t;return}$.value=n.project,$e.value=n.detected,window.$message?.success(`项目包导入成功`)}function Jt(){$.value&&u(`project-detail`,{params:{id:$.value.id}})}return t(async()=>{await Promise.all([Be(),Ve()]),s.projectId&&await He(s.projectId)}),(t,r)=>{let a=ae,s=Xe,l=Ye,m=le,h=we,_=ke,y=Te,O=Ce,ee=re,N=Oe,F=je,L=he,oe=De,se=Ae,B=_e,ue=te,fe=ne,me=ot,K=pe,xe=ge,Se=Re,Be=ie;return i(),f(p,null,[S(F,{show:A.value},{default:e(()=>[d(`div`,st,[d(`div`,ct,[d(`div`,null,[d(`div`,lt,`第 `+o(E.value+1)+` / `+o(g.length)+` 步 · `+o(D.value.title),1),d(`div`,ut,o(D.value.description),1)]),S(a,{onClick:ze},{default:e(()=>[...r[20]||=[n(`取消`,-1)]]),_:1})]),d(`div`,dt,[S(l,{current:E.value+1,size:`small`,style:{"min-width":`620px`}},{default:e(()=>[(i(),f(p,null,c(g,e=>S(s,{key:e.key,title:e.title,description:e.description},null,8,[`title`,`description`])),64))]),_:1},8,[`current`])]),E.value===0?(i(),b(O,{key:0,"label-placement":`top`},{default:e(()=>[S(h,{label:`项目名称`,required:``},{default:e(()=>[S(m,{value:M.value.name,"onUpdate:value":r[0]||=e=>M.value.name=e,placeholder:`例如：京西机房 perf 批次`,maxlength:`100`,"show-count":``},null,8,[`value`])]),_:1}),S(h,{label:`描述`},{default:e(()=>[S(m,{value:M.value.description,"onUpdate:value":r[1]||=e=>M.value.description=e,type:`textarea`,rows:3,placeholder:`选填，说明这批主机的用途、机房或批次`,maxlength:`500`},null,8,[`value`])]),_:1}),S(h,{label:`目标架构`,required:``},{default:e(()=>[S(y,{value:M.value.arch,"onUpdate:value":r[2]||=e=>M.value.arch=e},{default:e(()=>[(i(!0),f(p,null,c(x(Pe),t=>(i(),b(_,{key:String(t.value),value:t.value},{default:e(()=>[n(o(t.label),1)]),_:2},1032,[`value`]))),128))]),_:1},8,[`value`])]),_:1})]),_:1})):E.value===1?(i(),f(`div`,ft,[d(`div`,pt,[r[22]||=d(`span`,{class:`text-14px text-gray-500`},` 仅状态为「可用」的镜像可以被选用，其它状态会置灰并说明原因。 `,-1),S(ee,{checked:H.value,"onUpdate:checked":r[3]||=e=>H.value=e},{default:e(()=>[...r[21]||=[n(`显示全部架构`,-1)]]),_:1},8,[`checked`])]),de.value?(i(),b(N,{key:0,type:`warning`,size:`small`,bordered:!1},{default:e(()=>[...r[23]||=[n(` 当前项目绑定的基础镜像已从镜像库删除，请重新选择一个镜像。 `,-1)]]),_:1})):C(``,!0),ce.value?(i(),f(`div`,mt,[S(F,{size:`small`}),r[24]||=d(`div`,{class:`mt-8px text-13px text-gray-500`},`正在加载镜像库…`,-1)])):R.value.length?U.value.length?(i(),f(`div`,ht,[(i(!0),f(p,null,c(U.value,e=>(i(),f(`div`,{key:e.id,class:w([`select-card`,{"select-card--selected":e.id===M.value.iso_id,"select-card--disabled":!W(e)}]),role:`radio`,"aria-checked":e.id===M.value.iso_id,"aria-disabled":!W(e),onClick:t=>J(e)},[d(`div`,_t,[d(`div`,vt,[d(`div`,yt,o(e.name),1),d(`div`,bt,o(e.filename),1)]),S(se,{status:e.status,animated:!1},null,8,[`status`])]),d(`div`,xt,[d(`span`,St,o(e.arch),1),d(`span`,Ct,o(x(Le)(e.size_bytes)),1),d(`span`,null,o(x(Ie)[e.source].label),1)]),W(e)?C(``,!0):(i(),f(`div`,wt,o(be(e)),1))],10,gt))),128))])):(i(),b(L,{key:3,description:`镜像库中没有 ${M.value.arch} 架构的镜像。`},{extra:e(()=>[S(oe,{size:8},{default:e(()=>[S(a,{onClick:r[5]||=e=>H.value=!0},{default:e(()=>[...r[26]||=[n(`显示全部架构`,-1)]]),_:1}),S(a,{type:`primary`,onClick:r[6]||=e=>x(u)(`iso-library`)},{default:e(()=>[...r[27]||=[n(`去镜像库上传`,-1)]]),_:1})]),_:1})]),_:1},8,[`description`])):(i(),b(L,{key:2,description:`镜像库中还没有基础镜像，请先上传或登记一个 ISO。`},{extra:e(()=>[S(a,{type:`primary`,onClick:r[4]||=e=>x(u)(`iso-library`)},{default:e(()=>[...r[25]||=[n(`去镜像库上传`,-1)]]),_:1})]),_:1})),S(ue,{size:`small`,bordered:!0},{default:e(()=>[d(`div`,Tt,[d(`div`,null,[r[28]||=d(`div`,{class:`text-14px font-500`},`从项目包导入`,-1),d(`div`,Et,` 支持 .zip / .tar.gz / .tgz / .tar，单个文件不超过 `+o(x(Le)(qt))+`。服务端会自动识别包内的 kickstart 与主机清单。 `,1)]),S(a,{type:`primary`,secondary:``,onClick:nt},{icon:e(()=>[S(B,{icon:`mdi:package-variant-closed`})]),default:e(()=>[r[29]||=n(` 从项目包导入 `,-1)]),_:1})])]),_:1})])):E.value===2?(i(),f(`div`,Dt,[r[35]||=d(`span`,{class:`text-14px text-gray-500`},` 装机时 kickstart 会被写入 ISO，其中的占位符由构建阶段替换为主机清单。 `,-1),V.value?(i(),f(`div`,Ot,[S(F,{size:`small`}),r[30]||=d(`div`,{class:`mt-8px text-13px text-gray-500`},`正在加载模板库…`,-1)])):z.value.length?(i(),f(`div`,kt,[(i(!0),f(p,null,c(z.value,t=>(i(),f(`div`,{key:t.id,class:w([`select-card`,{"select-card--selected":t.id===M.value.kickstart_id,"select-card--disabled":!G(t)}]),role:`radio`,"aria-checked":t.id===M.value.kickstart_id,"aria-disabled":!G(t),onClick:e=>Ee(t)},[d(`div`,jt,[d(`div`,Mt,[d(`div`,Nt,o(t.name),1),d(`div`,Pt,[S(fe,{size:`tiny`,bordered:!1,class:`font-mono`},{default:e(()=>[n(o(t.arch),1)]),_:2},1024),t.builtin?(i(),b(fe,{key:0,size:`tiny`,type:`info`,bordered:!1},{default:e(()=>[...r[32]||=[n(`内置`,-1)]]),_:1})):C(``,!0),d(`span`,Ft,o(x(Le)(t.size_bytes)),1)])]),S(a,{size:`tiny`,quaternary:``,onClick:v(e=>Je(t),[`stop`])},{icon:e(()=>[S(B,{icon:`mdi:file-eye-outline`})]),default:e(()=>[r[33]||=n(` 预览 `,-1)]),_:1},8,[`onClick`])]),t.placeholders.length?(i(),f(`div`,It,[r[34]||=d(`span`,{class:`text-12px text-gray-500`},`占位符`,-1),(i(!0),f(p,null,c(t.placeholders,t=>(i(),b(fe,{key:t,size:`tiny`,bordered:!1,class:`font-mono`},{default:e(()=>[n(o(t),1)]),_:2},1024))),128))])):(i(),f(`div`,Lt,`无占位符`)),G(t)?C(``,!0):(i(),f(`div`,Rt,` 该模板仅适用于 `+o(t.arch)+` 架构。 `,1))],10,At))),128))])):(i(),b(L,{key:1,description:`模板库中还没有 kickstart 模板。`},{extra:e(()=>[S(a,{type:`primary`,onClick:r[7]||=e=>x(u)(`kickstart-library`)},{default:e(()=>[...r[31]||=[n(`去模板库上传`,-1)]]),_:1})]),_:1}))])):E.value===3?(i(),f(`div`,zt,[S(me,{value:P.value,"onUpdate:value":r[8]||=e=>P.value=e,format:I.value,"onUpdate:format":r[9]||=e=>I.value=e,onValidate:Me},null,8,[`value`,`format`])])):(i(),b(O,{key:4,"label-placement":`top`},{default:e(()=>[S(h,{label:`ISO 卷标（iso_label）`},{default:e(()=>[S(m,{value:M.value.iso_label,"onUpdate:value":r[10]||=e=>M.value.iso_label=e,placeholder:`留空则构建时自动探测原版 ISO 的卷标`},null,8,[`value`])]),_:1}),S(h,{label:`产物文件名（output_name）`},{default:e(()=>[S(m,{value:M.value.output_name,"onUpdate:value":r[11]||=e=>M.value.output_name=e,placeholder:`留空则构建时自动生成，例如 xxx-auto.iso`},null,8,[`value`])]),_:1}),S(N,{type:`info`,bordered:!1,"show-icon":!0},{default:e(()=>[...r[36]||=[d(`div`,{class:`flex flex-col gap-4px text-13px`},[d(`span`,null,[d(`span`,{class:`font-mono`},`iso_label`),n(` 留空时，构建阶段会挂载原版 ISO 并自动探测其卷标（volume label）后使用。 `)]),d(`span`,null,[d(`span`,{class:`font-mono`},`output_name`),n(` 留空时，构建阶段会按项目与架构自动生成产物文件名；填写时建议以 `),d(`span`,{class:`font-mono`},`.iso`),n(` 结尾。 `)])],-1)]]),_:1})]),_:1})),ye.value?(i(),b(N,{key:5,type:`warning`,size:`small`,bordered:!1,"show-icon":!0},{default:e(()=>[n(o(ye.value),1)]),_:1})):C(``,!0),d(`div`,Bt,[S(a,{disabled:E.value===0,onClick:Ne},{icon:e(()=>[S(B,{icon:`mdi:chevron-left`})]),default:e(()=>[r[37]||=n(` 上一步 `,-1)]),_:1},8,[`disabled`]),E.value<g.length-1?(i(),b(a,{key:0,type:`primary`,disabled:!q.value,onClick:Fe},{icon:e(()=>[S(B,{icon:`mdi:chevron-right`})]),default:e(()=>[r[38]||=n(` 下一步 `,-1)]),_:1},8,[`disabled`])):(i(),b(a,{key:1,type:`primary`,loading:j.value,disabled:!ve.value,onClick:Ue},{icon:e(()=>[S(B,{icon:`mdi:content-save-outline`})]),default:e(()=>[n(` `+o(T.value?`保存修改`:`创建项目`),1)]),_:1},8,[`loading`,`disabled`]))])])]),_:1},8,[`show`]),S(xe,{show:We.value,"onUpdate:show":r[12]||=e=>We.value=e,width:k.value,placement:`right`},{default:e(()=>[S(K,{title:`预览：${Ge.value?.name||``}`,"native-scrollbar":!1,closable:``},{default:e(()=>[S(F,{show:qe.value},{default:e(()=>[d(`div`,Vt,[Ke.value?(i(),f(`pre`,Ht,o(Ke.value),1)):qe.value?C(``,!0):(i(),b(L,{key:1,description:`模板内容加载失败`}))])]),_:1},8,[`show`])]),_:1},8,[`title`])]),_:1},8,[`show`,`width`]),S(xe,{show:Y.value,"onUpdate:show":r[19]||=e=>Y.value=e,width:k.value,placement:`right`},{default:e(()=>[S(K,{title:`从项目包导入`,"native-scrollbar":!1,closable:``},{default:e(()=>[$.value?(i(),f(`div`,Ut,[S(N,{type:`success`,bordered:!1,"show-icon":!0},{default:e(()=>[n(` 项目「`+o($.value.name)+`」导入完成。 `,1)]),_:1}),d(`div`,Wt,[r[39]||=d(`div`,{class:`text-13px text-gray-500`},`识别到的 kickstart（包内路径）`,-1),S(Se,{text:$e.value?.kickstart||``},null,8,[`text`]),r[40]||=d(`div`,{class:`text-13px text-gray-500`},`识别到的主机清单（包内路径）`,-1),S(Se,{text:$e.value?.hosts||``},null,8,[`text`])]),S(oe,null,{default:e(()=>[S(a,{type:`primary`,onClick:Jt},{default:e(()=>[...r[41]||=[n(`进入项目`,-1)]]),_:1}),S(a,{onClick:r[13]||=e=>Y.value=!1},{default:e(()=>[...r[42]||=[n(`关闭`,-1)]]),_:1})]),_:1})])):(i(),b(O,{key:1,"label-placement":`top`},{default:e(()=>[d(`input`,{ref_key:`importFileInputRef`,ref:Ze,type:`file`,accept:`.zip,.tar.gz,.tgz,.tar`,class:`hidden`,onChange:it},null,544),S(h,{label:`项目包`,required:``},{default:e(()=>[d(`div`,Gt,[S(a,{onClick:rt},{icon:e(()=>[S(B,{icon:`mdi:folder-open-outline`})]),default:e(()=>[r[43]||=n(` 选择文件 `,-1)]),_:1}),d(`span`,Kt,o(X.value?.name||`尚未选择文件`),1)])]),_:1}),S(N,{type:`info`,size:`small`,bordered:!1,"show-icon":!1,class:`mb-12px`},{default:e(()=>[...r[44]||=[n(` 未填写的字段会沿用服务端的默认值；下方字段已预填当前向导中的选择。 `,-1)]]),_:1}),S(h,{label:`项目名称`},{default:e(()=>[S(m,{value:Z.value.name,"onUpdate:value":r[14]||=e=>Z.value.name=e,placeholder:`留空则使用包内/服务端默认名称`},null,8,[`value`])]),_:1}),S(h,{label:`描述`},{default:e(()=>[S(m,{value:Z.value.description,"onUpdate:value":r[15]||=e=>Z.value.description=e,placeholder:`选填`},null,8,[`value`])]),_:1}),S(h,{label:`目标架构`},{default:e(()=>[S(Be,{value:Z.value.arch,"onUpdate:value":r[16]||=e=>Z.value.arch=e,options:x(Pe)},null,8,[`value`,`options`])]),_:1}),S(h,{label:`基础镜像`},{default:e(()=>[S(Be,{value:Z.value.iso_id,"onUpdate:value":r[17]||=e=>Z.value.iso_id=e,options:et.value,clearable:``,placeholder:`选填，仅列出「可用」的镜像`},null,8,[`value`,`options`])]),_:1}),S(h,{label:`kickstart 模板`},{default:e(()=>[S(Be,{value:Z.value.kickstart_id,"onUpdate:value":r[18]||=e=>Z.value.kickstart_id=e,options:tt.value,clearable:``,placeholder:`选填`},null,8,[`value`,`options`])]),_:1}),Q.value?(i(),b(N,{key:0,type:`error`,bordered:!1,"show-icon":!0,class:`mb-12px`},{default:e(()=>[n(o(Q.value),1)]),_:1})):C(``,!0),S(a,{type:`primary`,loading:Qe.value,onClick:at},{icon:e(()=>[S(B,{icon:`mdi:package-variant-closed`})]),default:e(()=>[r[45]||=n(` 开始导入 `,-1)]),_:1},8,[`loading`])]),_:1}))]),_:1})]),_:1},8,[`show`,`width`])],64)}}}),[[`__scopeId`,`data-v-61f30719`]]);export{Jt as t};