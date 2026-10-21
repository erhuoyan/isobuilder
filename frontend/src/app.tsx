import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { history } from '@umijs/max';
import { App } from 'antd';
import React from 'react';
import { AvatarDropdown } from '@/components';
import { errorConfig } from './requestErrorConfig';
import { getToken } from './services/auth';
import { fetchConfig } from './services/isobuilder';
import type { SystemConfig } from './services/types';
import defaultSettings from '../config/defaultSettings';

const LOGIN_PATH = '/login';

export interface InitialState {
  currentUser?: { name: string };
  systemConfig?: SystemConfig;
  authRequired: boolean;
  settings: Partial<LayoutSettings>;
}

/**
 * 全局启动数据：拉取 /api/config 决定是否需要登录。
 *
 * /api/config 永远不要求认证，所以即使未登录也能拿到 auth_required；
 * 只有它说需要认证且本地没有 token 时，才把用户赶回登录页。
 */
export async function getInitialState(): Promise<InitialState> {
  const base: InitialState = {
    authRequired: false,
    settings: defaultSettings as Partial<LayoutSettings>,
  };

  let systemConfig: SystemConfig | undefined;
  try {
    systemConfig = await fetchConfig();
  } catch {
    // 后端不可达时仍然渲染外壳，页面内的请求会各自报错。
  }

  const authRequired = systemConfig?.auth_required ?? false;
  const hasToken = getToken().length > 0;

  if (authRequired && !hasToken && history.location.pathname !== LOGIN_PATH) {
    history.push(LOGIN_PATH);
  }

  return {
    ...base,
    currentUser: hasToken || !authRequired ? { name: '运维' } : undefined,
    systemConfig,
    authRequired,
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

/**
 * ProLayout 运行时配置。
 *
 * 内部运维平台：去掉文档、多语言、全局搜索等与业务无关的入口，
 * 只留标题、面包屑和账户菜单。
 */
export const layout: RunTimeLayoutConfig = ({ initialState }) => {
  return {
    ...initialState?.settings,
    logo: '/logo.svg',
    title: 'ISO Builder',
    menu: { locale: false },
    avatarProps: {
      render: () => <AvatarDropdown />,
    },
    // 未登录（不需要认证的口子）时不渲染账户菜单
    rightContentRender: initialState?.currentUser ? undefined : () => null,
    onPageChange: () => {
      const { location } = history;
      if (initialState?.authRequired && !getToken() && location.pathname !== LOGIN_PATH) {
        history.push(LOGIN_PATH);
      }
    },
    childrenRender: (children) => <App>{children}</App>,
  };
};

export const request: RequestConfig = errorConfig;

export function rootContainer(container: React.ReactNode) {
  return container;
}
