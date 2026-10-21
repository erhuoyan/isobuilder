/**
 * @name 路由
 * @description 只声明业务页面，布局与菜单由 ProLayout 依据 name/icon 生成。
 * @doc https://umijs.org/docs/guides/routes
 */
export default [
  {
    path: '/login',
    layout: false,
    component: './user/login',
  },
  {
    path: '/',
    redirect: '/projects',
  },
  {
    path: '/projects',
    name: '装机项目',
    icon: 'ProjectOutlined',
    component: './Projects',
  },
  {
    path: '/projects/new',
    name: '新建项目',
    component: './ProjectWizard',
    hideInMenu: true,
  },
  {
    path: '/projects/:id',
    name: '项目详情',
    component: './ProjectDetail',
    hideInMenu: true,
  },
  {
    path: '/projects/:id/edit',
    name: '编辑项目',
    component: './ProjectWizard',
    hideInMenu: true,
  },
  {
    path: '/builds/:id',
    name: '构建详情',
    component: './BuildDetail',
    hideInMenu: true,
  },
  {
    path: '/isos',
    name: '镜像库',
    icon: 'CloudServerOutlined',
    component: './Isos',
  },
  {
    path: '/kickstarts',
    name: '应答模板',
    icon: 'FileTextOutlined',
    component: './Kickstarts',
  },
  {
    path: '/scripts',
    name: '自定义脚本',
    icon: 'CodeOutlined',
    component: './Scripts',
  },
  {
    path: '/settings',
    name: '系统配置',
    icon: 'SettingOutlined',
    component: './Settings',
  },
  {
    path: '/403',
    component: './exception/403',
  },
  {
    path: '/404',
    component: './exception/404',
  },
  {
    path: '/500',
    component: './exception/500',
  },
  {
    path: '*',
    component: './exception/404',
  },
];
