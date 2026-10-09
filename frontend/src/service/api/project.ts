import { request } from '../request';

/** get all projects, the latest build of each project is inlined */
export function fetchGetProjects() {
  return request<{ projects: Api.IsoBuilder.ProjectListItem[] }>({ url: '/api/projects' });
}

/** get one project */
export function fetchGetProject(id: string) {
  return request<{ project: Api.IsoBuilder.Project }>({ url: `/api/projects/${id}` });
}

/** create a project */
export function fetchCreateProject(payload: Api.IsoBuilder.ProjectPayload) {
  return request<{ project: Api.IsoBuilder.Project }>({ url: '/api/projects', method: 'post', data: payload });
}

/** update a project */
export function fetchUpdateProject(id: string, payload: Partial<Api.IsoBuilder.ProjectPayload>) {
  return request<{ project: Api.IsoBuilder.Project }>({ url: `/api/projects/${id}`, method: 'put', data: payload });
}

/** delete a project, its builds are deleted too */
export function fetchDeleteProject(id: string) {
  return request<null>({ url: `/api/projects/${id}`, method: 'delete' });
}

/** validate a hosts inventory without saving it */
export function fetchValidateHosts(content: string, format: Api.IsoBuilder.HostsFormatInput = 'auto') {
  return request<Api.IsoBuilder.ValidateHostsResult>({
    url: '/api/validate/hosts',
    method: 'post',
    data: { content, format }
  });
}

/** import a project package */
export function fetchImportProject(file: File, fields: Partial<Api.IsoBuilder.ProjectPayload> = {}) {
  const formData = new FormData();

  formData.append('file', file);

  if (fields.name) formData.append('name', fields.name);
  if (fields.description) formData.append('description', fields.description);
  if (fields.arch) formData.append('arch', fields.arch);
  if (fields.iso_id) formData.append('iso_id', fields.iso_id);
  if (fields.kickstart_id) formData.append('kickstart_id', fields.kickstart_id);

  return request<{ project: Api.IsoBuilder.Project; detected: Api.IsoBuilder.ImportDetected }>({
    url: '/api/projects/import',
    method: 'post',
    data: formData,
    timeout: 0
  });
}
