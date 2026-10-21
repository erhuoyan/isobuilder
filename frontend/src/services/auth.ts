/**
 * Token 存取。
 *
 * 后端只有一个共享口令（ISOBUILDER_AUTH_TOKEN），没有用户体系：
 * 登录就是把这个口令存到 localStorage，之后由请求拦截器带上。
 * GET /api/config 的 auth_required 决定是否真的需要登录。
 */
const STORAGE_KEY = 'isobuilder_token';

export function getToken(): string {
  return localStorage.getItem(STORAGE_KEY) ?? '';
}

export function setToken(token: string) {
  localStorage.setItem(STORAGE_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(STORAGE_KEY);
}
