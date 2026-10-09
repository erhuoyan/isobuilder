import 'axios';

declare module 'axios' {
  interface AxiosRequestConfig {
    /**
     * Do not show the backend error message of this request
     *
     * Used by the requests which report their errors in the page, e.g. the iso chunk upload which
     * recovers from `offset_mismatch` on its own.
     */
    silentError?: boolean;
  }
}
