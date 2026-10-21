import { request } from '@umijs/max';
import type {
  Build,
  HostsValidation,
  Iso,
  Kickstart,
  Project,
  ProjectInput,
  Script,
  Settings,
  SystemConfig,
} from './types';

/* ---------------------------------------------------------------- 系统 */

export async function fetchHealth() {
  return request<{ status: string; version: string; time: string }>('/api/health', {
    method: 'GET',
    skipErrorHandler: true,
  });
}

/** 永远可访问；auth_required 决定前端是否需要登录。 */
export async function fetchConfig() {
  return request<SystemConfig>('/api/config', { method: 'GET', skipErrorHandler: true });
}

export async function fetchSettings() {
  return request<Settings>('/api/settings', { method: 'GET' });
}

export interface StorageInput {
  endpoint?: string;
  region?: string;
  bucket?: string;
  prefix?: string;
  path_style?: boolean;
  insecure_tls?: boolean;
  public_base?: string;
  presign_ttl_seconds?: number;
  /** 传空串表示保留原值 */
  access_key?: string;
  secret_key?: string;
}

export async function updateSettings(storage: StorageInput) {
  return request<Settings>('/api/settings', {
    method: 'PUT',
    data: { storage },
  });
}

/* ---------------------------------------------------------------- ISO */

export async function listIsos() {
  return request<{ isos: Iso[] }>('/api/isos', { method: 'GET' });
}

export interface RegisterIsoByPath {
  source: 'path';
  path: string;
  name?: string;
  arch: string;
}

export interface RegisterIsoByURL {
  source: 'url';
  url: string;
  name?: string;
  arch: string;
}

/**
 * 登记镜像。两种来源都由服务端自己取文件：
 * - path: 服务器磁盘上已有的文件（本地注册）
 * - url:  任意 HTTP(S) 下载链接（对象存储的公开/预签名链接也走这条）
 * 浏览器不再上传镜像本体。
 */
export async function registerIso(body: RegisterIsoByPath | RegisterIsoByURL) {
  return request<{ iso: Iso }>('/api/isos/register', { method: 'POST', data: body });
}

export async function deleteIso(id: string) {
  return request<unknown>(`/api/isos/${id}`, { method: 'DELETE' });
}

/* ---------------------------------------------------------- Kickstart */

export async function listKickstarts() {
  return request<{ kickstarts: Kickstart[] }>('/api/kickstarts', { method: 'GET' });
}

export async function uploadKickstart(file: File, name: string, arch: string) {
  const form = new FormData();
  form.append('file', file);
  form.append('name', name);
  form.append('arch', arch);

  return request<{ kickstart: Kickstart }>('/api/kickstarts', {
    method: 'POST',
    data: form,
  });
}

export async function getKickstartContent(id: string) {
  return request<{ content: string }>(`/api/kickstarts/${id}/content`, { method: 'GET' });
}

export async function deleteKickstart(id: string) {
  return request<unknown>(`/api/kickstarts/${id}`, { method: 'DELETE' });
}

/* ------------------------------------------------------------- Script */

export async function listScripts() {
  return request<{ scripts: Script[] }>('/api/scripts', { method: 'GET' });
}

export async function createScript(body: { name: string; remark?: string; content: string }) {
  return request<{ script: Script }>('/api/scripts', { method: 'POST', data: body });
}

export async function updateScript(
  id: string,
  body: { name?: string; remark?: string; content?: string },
) {
  return request<{ script: Script }>(`/api/scripts/${id}`, { method: 'PUT', data: body });
}

export async function deleteScript(id: string) {
  return request<unknown>(`/api/scripts/${id}`, { method: 'DELETE' });
}

/* ------------------------------------------------------------ Project */

export async function listProjects() {
  return request<{ projects: Project[] }>('/api/projects', { method: 'GET' });
}

export async function getProject(id: string) {
  return request<{ project: Project }>(`/api/projects/${id}`, { method: 'GET' });
}

export async function createProject(body: ProjectInput) {
  return request<{ project: Project }>('/api/projects', { method: 'POST', data: body });
}

export async function updateProject(id: string, body: ProjectInput) {
  return request<{ project: Project }>(`/api/projects/${id}`, { method: 'PUT', data: body });
}

export async function deleteProject(id: string) {
  return request<unknown>(`/api/projects/${id}`, { method: 'DELETE' });
}

export async function validateHosts(content: string, format: string = 'auto') {
  return request<HostsValidation>('/api/validate/hosts', {
    method: 'POST',
    data: { content, format },
  });
}

/* -------------------------------------------------------------- Build */

export async function createBuild(projectId: string) {
  return request<{ build: Build }>(`/api/projects/${projectId}/builds`, { method: 'POST' });
}

export async function listBuilds(params: { project_id?: string; limit?: number } = {}) {
  return request<{ builds: Build[] }>('/api/builds', { method: 'GET', params });
}

export async function getBuild(id: string) {
  return request<{ build: Build }>(`/api/builds/${id}`, { method: 'GET' });
}

export async function cancelBuild(id: string) {
  return request<{ build: Build }>(`/api/builds/${id}/cancel`, { method: 'POST' });
}

export async function retryBuild(id: string) {
  return request<{ build: Build }>(`/api/builds/${id}/retry`, { method: 'POST' });
}

/** 日志全文；SSE 流由 `openBuildLog` 单独处理。 */
export async function getBuildLogText(id: string, offset = 0) {
  return request<string>(`/api/builds/${id}/log`, {
    method: 'GET',
    params: { offset, format: 'text' },
    responseType: 'text',
  });
}
