import type { RouteMeta } from 'vue-router';
import ElegantVueRouter from '@elegant-router/vue/vite';
import type { RouteKey } from '@elegant-router/types';

/** The route path of each page */
const routePaths: Partial<Record<RouteKey, string>> = {
  login: '/login',
  'project-list': '/projects',
  'project-create': '/projects/new',
  'project-detail': '/projects/:id',
  'project-edit': '/projects/:id/edit',
  'build-detail': '/builds/:id',
  'iso-library': '/isos',
  'kickstart-library': '/templates',
  settings: '/settings'
};

/** The menu meta of each page */
const routeMetas: Partial<Record<RouteKey, Partial<RouteMeta>>> = {
  'project-list': { icon: 'mdi:folder-multiple-outline', order: 1 },
  'iso-library': { icon: 'mdi:disc', order: 2 },
  'kickstart-library': { icon: 'mdi:file-code-outline', order: 3 },
  settings: { icon: 'mdi:cog-outline', order: 4 },
  'project-detail': { hideInMenu: true, activeMenu: 'project-list' },
  'project-edit': { hideInMenu: true, activeMenu: 'project-list' },
  'project-create': { hideInMenu: true, activeMenu: 'project-list' },
  'build-detail': { hideInMenu: true, activeMenu: 'project-list' },
  'iframe-page': { hideInMenu: true }
};

export function setupElegantRouter() {
  return ElegantVueRouter({
    layouts: {
      base: 'src/layouts/base-layout/index.vue',
      blank: 'src/layouts/blank-layout/index.vue'
    },
    routePathTransformer(routeName, routePath) {
      const key = routeName as RouteKey;

      if (key === 'login') {
        const modules: UnionKey.LoginModule[] = ['pwd-login', 'code-login', 'register', 'reset-pwd', 'bind-wechat'];

        const moduleReg = modules.join('|');

        return `/login/:module(${moduleReg})?`;
      }

      return routePaths[key] || routePath;
    },
    onRouteMetaGen(routeName) {
      const key = routeName as RouteKey;

      const constantRoutes: RouteKey[] = ['login', '403', '404', '500'];

      const isConstant = constantRoutes.includes(key);

      const meta: Partial<RouteMeta> = {
        title: key,
        i18nKey: `route.${key}` as App.I18n.I18nKey,
        // The 2.2.0 router plugin generates routes for the `_builtin` error pages
        // as well; without `hideInMenu` they would all show up in the sider.
        hideInMenu: isConstant,
        ...routeMetas[key]
      };

      if (isConstant) {
        meta.constant = true;
      }

      return meta;
    }
  });
}
