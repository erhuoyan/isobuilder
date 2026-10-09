import { request } from '../request';

/** get all kickstart templates */
export function fetchGetKickstarts() {
  return request<{ kickstarts: Api.IsoBuilder.Kickstart[] }>({ url: '/api/kickstarts' });
}

/** upload a kickstart template */
export function fetchUploadKickstart(payload: { file: File; name?: string; arch: Api.IsoBuilder.KickstartArch }) {
  const formData = new FormData();

  formData.append('file', payload.file);
  formData.append('arch', payload.arch);

  if (payload.name) {
    formData.append('name', payload.name);
  }

  return request<{ kickstart: Api.IsoBuilder.Kickstart }>({
    url: '/api/kickstarts',
    method: 'post',
    data: formData,
    timeout: 0
  });
}

/** get the content of a kickstart template */
export function fetchGetKickstartContent(id: string) {
  return request<{ content: string }>({ url: `/api/kickstarts/${id}/content` });
}

/** delete a kickstart template */
export function fetchDeleteKickstart(id: string) {
  return request<null>({ url: `/api/kickstarts/${id}`, method: 'delete' });
}
