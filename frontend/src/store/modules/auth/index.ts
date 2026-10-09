import { computed, reactive, ref } from 'vue';
import { useRoute } from 'vue-router';
import { defineStore } from 'pinia';
import { useLoading } from '@sa/hooks';
import { fetchGetIsos } from '@/service/api';
import { useRouterPush } from '@/hooks/common/router';
import { localStg } from '@/utils/storage';
import { SetupStoreId } from '@/enum';
import { useRouteStore } from '../route';
import { useTabStore } from '../tab';
import { clearAuthStorage } from './shared';

export const useAuthStore = defineStore(SetupStoreId.Auth, () => {
  const route = useRoute();
  const authStore = useAuthStore();
  const routeStore = useRouteStore();
  const tabStore = useTabStore();
  const { toLogin, redirectFromLogin } = useRouterPush(false);
  const { loading: loginLoading, startLoading, endLoading } = useLoading();

  const token = ref(localStg.get('token') || '');

  /**
   * The backend is guarded by a single shared token, so the user info is a constant.
   *
   * `roles` contains the super role so that every static route is authorized.
   */
  const userInfo: Api.Auth.UserInfo = reactive({
    userId: 'isobuilder',
    userName: 'isobuilder',
    roles: [import.meta.env.VITE_STATIC_SUPER_ROLE],
    buttons: []
  });

  /** is super role in static route */
  const isStaticSuper = computed(() => {
    const { VITE_AUTH_ROUTE_MODE, VITE_STATIC_SUPER_ROLE } = import.meta.env;

    return VITE_AUTH_ROUTE_MODE === 'static' && userInfo.roles.includes(VITE_STATIC_SUPER_ROLE);
  });

  /** Is login */
  const isLogin = computed(() => Boolean(token.value));

  /** Check if the token is accepted by the backend */
  async function verifyToken() {
    const { error } = await fetchGetIsos();

    return !error;
  }

  /** Reset auth store */
  async function resetStore() {
    clearAuthStorage();

    authStore.$reset();

    if (!route.meta.constant) {
      await toLogin();
    }

    tabStore.cacheTabs();
    routeStore.resetStore();
  }

  /**
   * Login
   *
   * @param loginToken the token of the isobuilder backend
   * @param [redirect=true] Whether to redirect after login. Default is `true`
   */
  async function login(loginToken: string, redirect = true) {
    startLoading();

    const tokenValue = loginToken.trim();
    localStg.set('token', tokenValue);
    token.value = tokenValue;

    const pass = await verifyToken();

    if (pass) {
      await redirectFromLogin(redirect);
    } else {
      resetStore();
    }

    endLoading();

    return pass;
  }

  async function initUserInfo() {
    const maybeToken = localStg.get('token');

    if (maybeToken) {
      token.value = maybeToken;
    }
  }

  return {
    token,
    userInfo,
    isStaticSuper,
    isLogin,
    loginLoading,
    login,
    verifyToken,
    resetStore,
    initUserInfo
  };
});
