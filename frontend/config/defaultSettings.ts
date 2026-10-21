import type { ProLayoutProps } from '@ant-design/pro-components';

/**
 * @name 布局默认值
 * @description ProLayout 的初始外观，运行时可在 app.tsx 里按需覆盖。
 */
const Settings: ProLayoutProps & {
  logo?: string;
} = {
  navTheme: 'light',
  colorPrimary: '#1677ff',
  layout: 'side',
  contentWidth: 'Fluid',
  fixedHeader: true,
  fixSiderbar: true,
  colorWeak: false,
  title: 'ISO Builder',
  logo: '/logo.svg',
  iconfontUrl: '',
  token: {},
};

export default Settings;
