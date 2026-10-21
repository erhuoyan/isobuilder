/**
 * @name 代理配置
 * @description 仅在 `max dev` 生效；生产环境由 Go 二进制同源提供 SPA 与 /api。
 * @doc https://umijs.org/docs/guides/proxy
 */
export default {
  dev: {
    '/api/': {
      target: process.env.ISOBUILDER_API || 'http://isobuilder.baremetal.internal',
      changeOrigin: true,
    },
  },
};
