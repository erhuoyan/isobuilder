// https://umijs.org/config/

import { join } from 'node:path';
import { defineConfig } from '@umijs/max';
import defaultSettings from './defaultSettings';
import proxy from './proxy';
import routes from './routes';

const { UMI_ENV = 'dev' } = process.env;

/**
 * @name 公共路径
 * @description 生产环境由 Go 二进制同源提供 SPA，固定为根路径。
 */
const PUBLIC_PATH: string = '/';

export default defineConfig({
  hash: true,

  publicPath: PUBLIC_PATH,

  routes,

  /**
   * @name 产物目录
   * @description Go 二进制把 backend/web/dist 作为静态目录提供，
   * deploy/release.sh 与 Dockerfile 都从那里取产物。
   */
  outputPath: '../backend/web/dist',

  ignoreMomentLocale: true,

  proxy: proxy[UMI_ENV as keyof typeof proxy],

  fastRefresh: true,

  routePrefetch: {},

  manifest: {},

  model: {},

  initialState: {},

  title: 'ISO Builder',

  layout: {
    locale: false,
    ...defaultSettings,
  },

  moment2dayjs: {
    preset: 'antd',
    plugins: ['duration', 'relativeTime'],
  },

  /**
   * 界面固定中文：这是内部运维平台，不提供语言切换。
   */
  locale: {
    default: 'zh-CN',
    antd: true,
    baseNavigator: false,
  },

  antd: {
    appConfig: {},
    configProvider: {
      theme: {
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 6,
        },
      },
    },
  },

  request: {},

  access: {},

  headScripts: [
    // 解决首次加载时白屏的问题
    { src: join(PUBLIC_PATH, 'scripts/loading.js'), async: true },
  ],

  define: {
    'process.env.CI': process.env.CI,
    __APP_VERSION__: require('./../package.json').version,
  },
});
