/**
 * The type of the isobuilder backend API
 *
 * The contract is frozen in `isobuilder/docs/API.md`.
 */
declare namespace Api {
  namespace IsoBuilder {
    /** the payload of `PUT /api/kickstarts/{id}`（字段省略＝不变，content 传入即更新正文） */
    interface KickstartUpdatePayload {
      name?: string;
      remark?: string;
      content?: string;
    }

    /** The response envelope every isobuilder endpoint returns (`{code, message, data, details?}`). */
    interface Envelope<T> {
      code: string;
      message: string;
      data: T;
      details?: unknown;
    }

    /** the cpu architecture of an iso / kickstart / project */
    type Arch = 'x86_64' | 'aarch64';

    /** the kickstart can be applied to any architecture */
    type KickstartArch = Arch | 'any';

    /** U 盘 payload 注入模式：off 不注入 / label 按卷标 / auto 卷标优先+扫描 */
    type PayloadMode = 'off' | 'label' | 'auto';
    /** the format of the hosts inventory */
    type HostsFormat = 'yaml' | 'json';

    /** the format which is sent to the backend, `auto` lets the backend detect it */
    type HostsFormatInput = HostsFormat | 'auto';

    /** the network mode of a host */
    type NetworkMode = 'single' | 'bond';

    /** the status of a base iso */
    type IsoStatus = 'downloading' | 'hashing' | 'ready' | 'error';

    /**
     * How the iso entered the library.
     *
     * `registered` adopts a file already on the server disk, `url` makes the server download an
     * arbitrary HTTP(S) link — object storage presigned/public links included.
     */
    type IsoSource = 'registered' | 'url';

    /** the status of a build */
    type BuildStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled';

    /** the stage of a build, the stage is empty when the build has not started */
    type BuildStage = 'mount' | 'inject' | 'boot' | 'build' | 'seal' | '';

    /** the artifact storage driver */
    type StorageDriver = 's3' | 'local' | '';

    /** `GET /api/health` */
    interface Health {
      status: string;
      version: string;
      time: string;
    }

    /** `GET /api/config` */
    interface Config {
      version: string;
      git_commit: string;
      build_time: string;
      go_version: string;
      /** whether the backend requires a token, `GET /api/config` is always accessible */
      auth_required: boolean;
      archs: Arch[];
      max_upload_bytes: number;
      max_concurrent_builds: number;
      storage: {
        enabled: boolean;
        driver: StorageDriver;
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

    /** where the effective object storage configuration comes from */
    type StorageSettingsSource = 'db' | 'env';

    /** `GET /api/settings` — the effective object storage configuration, secrets masked */
    interface StorageSettings {
      endpoint: string;
      region: string;
      bucket: string;
      prefix: string;
      path_style: boolean;
      insecure_tls: boolean;
      public_base: string;
      presign_ttl_seconds: number;
      /** whether an access key is stored, the key itself is never returned */
      access_key_set: boolean;
      /** whether a secret key is stored, the key itself is never returned */
      secret_key_set: boolean;
      /** whether object storage is enabled, derived from a non-empty `bucket` */
      enabled: boolean;
      source: StorageSettingsSource;
    }

    /** `GET /api/settings` */
    interface Settings {
      storage: StorageSettings;
      /** whether the settings page has ever been saved */
      storage_raw: boolean;
    }

    /**
     * `PUT /api/settings` body
     *
     * Every field is optional, an omitted field keeps its current value. An empty
     * `access_key` / `secret_key` also keeps the stored secret.
     */
    interface StorageSettingsPayload {
      endpoint?: string;
      region?: string;
      bucket?: string;
      prefix?: string;
      path_style?: boolean;
      insecure_tls?: boolean;
      public_base?: string;
      presign_ttl_seconds?: number;
      access_key?: string;
      secret_key?: string;
    }

    /** `PUT /api/settings` */
    interface SettingsPayload {
      storage: StorageSettingsPayload;
    }

    /** the s3 branch of `POST /api/isos/register` */
    interface S3RegisterSource {
      /** `s3://bucket/key`, it wins over `bucket` + `key` when present */
      url?: string;
      endpoint?: string;
      region?: string;
      bucket?: string;
      key?: string;
      access_key?: string;
      secret_key?: string;
      path_style?: boolean;
    }

    /** `POST /api/isos/register` */
    interface RegisterIsoPayload {
      source: 'path' | 'url';
      /** the absolute path on the server, used by `source=path` */
      path?: string;
      /** the HTTP(S) download link, used by `source=url` */
      url?: string;
      name?: string;
      remark?: string;
      arch: Arch;
    }

    /** a base iso */
    interface Iso {
      id: string;
      name: string;
      filename: string;
      arch: Arch;
      size_bytes: number;
      /** empty while the hash is being computed or when it failed */
      sha256: string;
      /** 基础镜像的 MD5（装机侧校验习惯） */
      md5: string;
      /** 登记时填写的备注 */
      remark: string;
      status: IsoStatus;
      source: IsoSource;
      path: string;
      created_at: string;
      /** the reason of `status=error` */
      error: string;
    }

    /** a kickstart template */
    interface Kickstart {
      id: string;
      name: string;
      arch: KickstartArch;
      builtin: boolean;
      /** 备注说明（可编辑） */
      remark: string;
      placeholders: string[];
      size_bytes: number;
      created_at: string;
    }

    /** a reusable post-install script (script library) */
    interface Script {
      id: string;
      name: string;
      remark: string;
      /** the script body */
      content: string;
      size_bytes: number;
      created_at: string;
      updated_at: string;
    }

    /** the parsed summary of one host of the inventory */
    interface HostSummary {
      sn: string;
      hostname: string;
      ip: string;
      /** empty when the host has no gateway configured */
      gateway: string;
      /** 0 when disk_target_size is not configured */
      disk_gb: number;
      network_mode: NetworkMode;
      mac_count: number;
      /** 802.1Q VLAN tag; 0 when the address is untagged */
      vlan_id: number;
      bmc_ip: string;
    }

    /** one validation error of the hosts inventory */
    interface HostError {
      /** the index of the host in the inventory, the first host is 0 */
      index: number;
      /** the field path, e.g. `network.bond.slaves` */
      field: string;
      message: string;
    }

    /** `POST /api/validate/hosts` */
    interface ValidateHostsResult {
      ok: boolean;
      /** the detected format of the inventory */
      format: HostsFormat;
      hosts_count: number;
      hosts_summary: HostSummary[];
      errors: HostError[];
    }

    /** a project */
    interface Project {
      id: string;
      name: string;
      description: string;
      arch: Arch;
      iso_id: string;
      /** the inline iso, null when the iso has been deleted */
      iso: Iso | null;
      kickstart_id: string;
      /** the inline kickstart, null when the template has been deleted */
      kickstart: Kickstart | null;
      /** id of the bound script, empty when none */
      script_id: string;
      /** inline script metadata (body omitted), null when unbound or deleted */
      script: Script | null;
      iso_label: string;
      output_name: string;
      /** U 盘 payload 注入模式：off 不注入 / label 按卷标 / auto 卷标+扫描 */
      payload_mode: PayloadMode;
      /** the project-level post-install script, empty when not configured */
      post_script: string;
      hosts_format: HostsFormat;
      /** the raw inventory, it is not returned by `GET /api/projects` */
      hosts_raw: string;
      hosts_count: number;
      hosts_summary: HostSummary[];
      created_at: string;
      updated_at: string;
    }

    /** the request body of creating / updating a project */
    interface ProjectPayload {
      name: string;
      description: string;
      arch: Arch;
      iso_id: string;
      kickstart_id: string;
      iso_label: string;
      output_name: string;
      /** id of the bound script from the library */
      script_id?: string;
      /** U 盘 payload 注入模式 */
      payload_mode?: PayloadMode;
      post_script?: string;
      hosts_raw: string;
      hosts_format: HostsFormatInput;
    }

    /**
     * one entry of a project list
     *
     * `GET /api/projects` omits `hosts_raw`, the raw inventory must be fetched per project.
     */
    type ProjectListItem = Omit<Project, 'hosts_raw'>;

    /** the artifact of a succeeded build */
    interface Artifact {
      id: string;
      build_id: string;
      filename: string;
      size_bytes: number;
      sha256: string;
      storage: StorageDriver;
      /** the presigned url, it is empty when the storage driver is `local` */
      url: string;
      /** the expiration time of the presigned url, empty when the storage driver is `local` */
      url_expires_at: string;
      created_at: string;
    }

    /** a build task */
    interface Build {
      id: string;
      project_id: string;
      project_name: string;
      status: BuildStatus;
      stage: BuildStage;
      stage_index: number;
      stage_total: number;
      /** 0-100, derived from the stage */
      progress: number;
      arch: Arch;
      iso_name: string;
      iso_label: string;
      output_name: string;
      hosts_count: number;
      log_bytes: number;
      /** the reason of `status=failed` */
      error: string;
      artifact: Artifact | null;
      created_at: string;
      started_at: string;
      finished_at: string;
    }

    /** one line of the build log stream */
    interface LogLine {
      /** the byte offset to resume the stream from after this line */
      offset: number;
      line: string;
    }

    /** the payload of the `end` event of the build log stream */
    interface LogEnd {
      status: BuildStatus;
      next_offset: number;
    }
  }
}
