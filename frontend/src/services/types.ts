/** 实体与请求参数类型，逐条对应 docs/API.md（v1 冻结契约）。 */

export type Arch = 'x86_64' | 'aarch64';
export type KickstartArch = Arch | 'any';

export type IsoStatus = 'downloading' | 'hashing' | 'ready' | 'error';
export type IsoSource = 'registered' | 'url';

export type BuildStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled';
export type BuildStage = 'mount' | 'inject' | 'boot' | 'build' | 'seal';

export type PayloadMode = 'off' | 'label' | 'auto';
export type HostsFormat = 'auto' | 'yaml' | 'json';

export interface Iso {
  id: string;
  name: string;
  filename: string;
  arch: Arch;
  size_bytes: number;
  /** 校验中/失败时为空串 */
  sha256: string;
  status: IsoStatus;
  source: IsoSource;
  path: string;
  created_at: string;
  error: string;
}

export interface Kickstart {
  id: string;
  name: string;
  arch: KickstartArch;
  builtin: boolean;
  placeholders: string[];
  size_bytes: number;
  created_at: string;
}

export interface Script {
  id: string;
  name: string;
  remark: string;
  content: string;
  size_bytes: number;
  created_at: string;
  updated_at: string;
}

export interface HostSummary {
  sn: string;
  hostname: string;
  ip: string;
  /** 未配置网关时为空串 */
  gateway: string;
  /** 未配置 disk_target_size 时为 0 */
  disk_gb: number;
  network_mode: 'single' | 'bond';
  mac_count: number;
  bmc_ip: string;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  arch: Arch;
  iso_id: string;
  /** 内联展开；镜像被删除后为 null */
  iso: Iso | null;
  kickstart_id: string;
  kickstart: Kickstart | null;
  script_id: string;
  script: Pick<Script, 'id' | 'name' | 'remark' | 'size_bytes'> | null;
  iso_label: string;
  output_name: string;
  post_script: string;
  payload_mode: PayloadMode;
  hosts_format: 'yaml' | 'json';
  /** 列表接口不返回 */
  hosts_raw?: string;
  hosts_count: number;
  hosts_summary: HostSummary[];
  created_at: string;
  updated_at: string;
}

export interface Artifact {
  id: string;
  build_id: string;
  filename: string;
  size_bytes: number;
  sha256: string;
  storage: 's3' | 'local';
  url: string;
  /** storage=local 时为空 */
  url_expires_at: string;
  created_at: string;
}

export interface Build {
  id: string;
  project_id: string;
  project_name: string;
  status: BuildStatus;
  /** 未开始时为空串 */
  stage: BuildStage | '';
  stage_index: number;
  stage_total: number;
  progress: number;
  arch: Arch;
  iso_name: string;
  iso_label: string;
  output_name: string;
  hosts_count: number;
  log_bytes: number;
  error: string;
  artifact: Artifact | null;
  created_at: string;
  started_at: string;
  finished_at: string;
}

export interface StorageSettings {
  endpoint: string;
  region: string;
  bucket: string;
  prefix: string;
  path_style: boolean;
  insecure_tls: boolean;
  public_base: string;
  presign_ttl_seconds: number;
  access_key_set: boolean;
  secret_key_set: boolean;
  enabled: boolean;
  source: 'db' | 'env';
}

export interface Settings {
  storage: StorageSettings;
  /** 配置页是否已写入过 */
  storage_raw: boolean;
}

export interface SystemConfig {
  version: string;
  auth_required: boolean;
  archs: Arch[];
  max_upload_bytes: number;
  max_concurrent_builds: number;
  storage: {
    enabled: boolean;
    driver: string;
    endpoint: string;
    bucket: string;
    prefix: string;
    presign_ttl_seconds: number;
  };
  disk: {
    data_dir: string;
    free_bytes: number;
    iso_dir: string;
    work_dir: string;
    builds_dir: string;
  };
}

export interface HostIssue {
  index: number;
  field: string;
  message: string;
}

export interface HostsValidation {
  ok: boolean;
  format: 'yaml' | 'json';
  hosts_count: number;
  hosts_summary: HostSummary[];
  errors: HostIssue[];
}

/** 项目创建/更新 body：省略字段表示不变。 */
export interface ProjectInput {
  name?: string;
  description?: string;
  arch?: Arch;
  iso_id?: string;
  kickstart_id?: string;
  iso_label?: string;
  output_name?: string;
  script_id?: string;
  post_script?: string;
  payload_mode?: PayloadMode;
  hosts_raw?: string;
  hosts_format?: HostsFormat;
}
