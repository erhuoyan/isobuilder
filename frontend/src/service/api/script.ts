import { request } from '../request';

/** 装机后脚本库 */

/** get all scripts */
export function fetchGetScripts() {
  return request<{ scripts: Api.IsoBuilder.Script[] }>({ url: '/api/scripts' });
}

/** create a script */
export function fetchCreateScript(payload: { name: string; remark?: string; content: string }) {
  return request<{ script: Api.IsoBuilder.Script }>({
    url: '/api/scripts',
    method: 'post',
    data: payload,
    silentError: true
  });
}

/** update a script（省略字段表示不变） */
export function fetchUpdateScript(
  id: string,
  payload: { name?: string; remark?: string; content?: string }
) {
  return request<{ script: Api.IsoBuilder.Script }>({
    url: `/api/scripts/${id}`,
    method: 'put',
    data: payload,
    silentError: true
  });
}

/** delete a script（被项目引用时返回 409 script_in_use） */
export function fetchDeleteScript(id: string) {
  return request<null>({ url: `/api/scripts/${id}`, method: 'delete', silentError: true });
}
