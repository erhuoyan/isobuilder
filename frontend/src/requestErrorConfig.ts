import type { RequestConfig } from '@umijs/max';
import { history } from '@umijs/max';
import { message as antdMessage } from 'antd';
import type { AxiosResponse } from 'axios';
import { clearToken, getToken } from '@/services/auth';

/** 后端统一响应信封，见 docs/API.md「通用约定」。 */
interface Envelope<T = unknown> {
  code: string;
  message: string;
  data: T;
  details?: Record<string, unknown>;
}

/** 业务错误：HTTP 200 但 code !== "200"。 */
export class BusinessError extends Error {
  code: string;
  details?: Record<string, unknown>;

  constructor(code: string, msg: string, details?: Record<string, unknown>) {
    super(msg);
    this.name = 'BusinessError';
    this.code = code;
    this.details = details;
  }
}

function isEnvelope(value: unknown): value is Envelope {
  return !!value && typeof value === 'object' && 'code' in value && 'data' in value;
}

export const errorConfig: RequestConfig = {
  errorConfig: {
    errorHandler: (error: any) => {
      // 业务错误已在响应拦截器里转成 BusinessError，文案由调用方决定是否展示。
      if (error instanceof BusinessError) throw error;

      const status = error?.response?.status;

      if (status === 401) {
        clearToken();
        if (window.location.pathname !== '/login') {
          history.push(`/login?redirect=${encodeURIComponent(window.location.pathname)}`);
        }
        throw error;
      }

      const detail = error?.response?.data ?? error?.message ?? '请求失败';
      antdMessage.error(typeof detail === 'string' ? detail : '请求失败');
      throw error;
    },
  },

  requestInterceptors: [
    (config) => {
      const token = getToken();
      if (token) {
        // RequestOptions['headers'] is a loose record; merging keeps caller headers.
        config.headers = { ...config.headers, Authorization: `Bearer ${token}` };
      }
      return config;
    },
  ],

  responseInterceptors: [
    (response: AxiosResponse) => {
      const body = response.data;

      if (!isEnvelope(body)) return response;

      if (body.code === '200') {
        // 调用方直接拿 data，不必层层解包。
        response.data = body.data;
        return response;
      }

      throw new BusinessError(body.code, body.message || body.code, body.details);
    },
  ],
};
