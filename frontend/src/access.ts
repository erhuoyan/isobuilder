import type { InitialState } from './app';

/**
 * 权限判定。
 *
 * 平台只有单一共享口令，没有角色体系，所以这里没有细粒度权限；
 * 保留该文件是因为 Umi 的 layout 插件要求存在 access 配置。
 */
export default function access(initialState: InitialState | undefined) {
  return {
    canAccess: Boolean(initialState?.currentUser) || !initialState?.authRequired,
  };
}
