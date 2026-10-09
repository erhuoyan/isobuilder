import type { AxiosError, AxiosResponse } from 'axios';
import { useAuthStore } from '@/store/modules/auth';
import { getServiceBaseURL } from '@/utils/service';
import { createFlatRequest } from '@sa/axios';
import { getApiErrorMessage } from '../api/errors';
import { getAuthorization, showErrorMsg } from './shared';
import type { RequestInstanceState } from './type';

export { getAuthorization } from './shared';
export { getApiErrorCode, getApiErrorDetails, getApiErrorStatus } from '../api/errors';

export const isHttpProxy = import.meta.env.DEV && import.meta.env.VITE_HTTP_PROXY === 'Y';

/** base url of the isobuilder backend, exported for the hand-written SSE log reader */
export const serviceBaseURL = getServiceBaseURL(import.meta.env, isHttpProxy).baseURL;

/**
 * The isobuilder backend speaks the SoybeanAdmin envelope:
 *
 *   success: `{"code":"200","message":"ok","data": …}`
 *   failure: `{"code":"<biz code>","message":"<readable>","data":{},"details"?}` over HTTP 200
 *
 * Only transport boundaries (401) leave the envelope. With
 * `VITE_SERVICE_SUCCESS_CODE=200` in .env, this mapping needs no further
 * customization in the `@sa/axios` layer.
 */
export const request = createFlatRequest<Api.IsoBuilder.Envelope<any>, any, RequestInstanceState>(
  {
    baseURL: serviceBaseURL,
    timeout: 30 * 1000
  },
  {
    defaultState: {
      errMsgStack: [],
      refreshTokenPromise: null
    } as RequestInstanceState,
    transform(response: AxiosResponse<Api.IsoBuilder.Envelope<any>>) {
      return response.data.data;
    },
    async onRequest(config) {
      const Authorization = getAuthorization();

      if (Authorization) {
        config.headers.set('Authorization', Authorization);
      }

      return config;
    },
    isBackendSuccess(response) {
      return String(response.data?.code) === import.meta.env.VITE_SERVICE_SUCCESS_CODE;
    },
    async onBackendFail(_response) {
      return null;
    },
    async onError(error: AxiosError<Api.IsoBuilder.Envelope<any>>) {
      // an aborted request is a caller-side cancellation, not a failure
      if (error.code === 'ERR_CANCELED') return;

      // the shared token is invalid or missing: reset the auth store so the router
      // guard sends the user back to the token form when the backend requires one
      if (error.response?.status === 401) {
        useAuthStore().resetStore();
        return;
      }

      // callers mark requests with `silentError` when they render the failure themselves
      // (in-drawer forms, the chunked uploader)
      if (error.config?.silentError) return;

      showErrorMsg(request.state, getApiErrorMessage(error));
    }
  }
);
