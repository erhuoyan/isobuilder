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
