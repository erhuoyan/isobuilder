import {
  PageContainer,
  ProCard,
  ProForm,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { Alert, Button, Input, Radio, Skeleton, Space, Steps, Tag, message } from 'antd';
import React, { useEffect, useMemo, useState } from 'react';
import { ARCH_OPTIONS, HOSTS_FORMAT_OPTIONS, PAYLOAD_MODE_OPTIONS } from '@/constants';
import {
  createProject,
  getKickstartContent,
  getProject,
  listIsos,
  listKickstarts,
  listScripts,
  updateProject,
  validateHosts,
} from '@/services/isobuilder';
import type {
  HostsValidation,
  Iso,
  Kickstart,
  ProjectInput,
  Script,
} from '@/services/types';

const STEPS = [
  { title: '基础信息', description: '项目名称与目标架构' },
  { title: '基础镜像', description: '选用一个已就绪的 ISO' },
  { title: '应答模板', description: '选用 kickstart 模板' },
  { title: '主机清单', description: '粘贴并校验主机信息' },
  { title: '自定义脚本', description: '安装完成后执行的脚本' },
  { title: '高级项', description: 'ISO 卷标与产物文件名' },
];

interface FormState {
  name: string;
  description: string;
  arch: 'x86_64' | 'aarch64';
  iso_id: string;
  kickstart_id: string;
  hosts_raw: string;
  hosts_format: 'auto' | 'yaml' | 'json';
  script_id: string;
  post_script: string;
  payload_mode: 'off' | 'label' | 'auto';
  iso_label: string;
  output_name: string;
}

const EMPTY: FormState = {
  name: '',
  description: '',
  arch: 'x86_64',
  iso_id: '',
  kickstart_id: '',
  hosts_raw: '',
  hosts_format: 'auto',
  script_id: '',
  post_script: '',
  payload_mode: 'off',
  iso_label: '',
  output_name: '',
};

const ProjectWizardPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const isEdit = Boolean(id);

  const [step, setStep] = useState(0);
  const [form, setForm] = useState<FormState>(EMPTY);
  const [isos, setIsos] = useState<Iso[]>([]);
  const [kickstarts, setKickstarts] = useState<Kickstart[]>([]);
  const [scripts, setScripts] = useState<Script[]>([]);
  const [validation, setValidation] = useState<HostsValidation | null>(null);
  const [validating, setValidating] = useState(false);
  const [saving, setSaving] = useState(false);
  /** 编辑模式下项目详情是异步的，加载完成前不能挂载表单 */
  const [loaded, setLoaded] = useState(!isEdit);
  const [preview, setPreview] = useState('');

  const patch = (next: Partial<FormState>) => setForm((prev) => ({ ...prev, ...next }));

  useEffect(() => {
    void (async () => {
      const [isoRes, ksRes, scriptRes] = await Promise.all([
        listIsos(),
        listKickstarts(),
        listScripts(),
      ]);
      setIsos(isoRes.isos ?? []);
      setKickstarts(ksRes.kickstarts ?? []);
      setScripts(scriptRes.scripts ?? []);

      if (id) {
        const res = await getProject(id);
        const p = res.project;
        setForm({
          name: p.name,
          description: p.description,
          arch: p.arch,
          iso_id: p.iso_id,
          kickstart_id: p.kickstart_id,
          hosts_raw: p.hosts_raw ?? '',
          hosts_format: p.hosts_format,
          script_id: p.script_id,
          post_script: p.post_script,
          payload_mode: p.payload_mode,
          iso_label: p.iso_label,
          output_name: p.output_name,
        });
      }

      setLoaded(true);
    })();
  }, [id]);

  const isoOptions = useMemo(
    () =>
      isos.map((iso) => ({
        label: `${iso.name} · ${iso.arch}${iso.status === 'ready' ? '' : `（${iso.status}）`}`,
        value: iso.id,
        disabled: iso.status !== 'ready',
      })),
    [isos],
  );

  const kickstartOptions = useMemo(
    () => kickstarts.map((k) => ({ label: `${k.name} · ${k.arch}`, value: k.id })),
    [kickstarts],
  );

  const scriptOptions = useMemo(
    () => scripts.map((s) => ({ label: `${s.name}${s.remark ? ` · ${s.remark}` : ''}`, value: s.id })),
    [scripts],
  );

  const handleValidate = async () => {
    if (!form.hosts_raw.trim()) {
      message.warning('请先粘贴主机清单');
      return;
    }
    setValidating(true);
    try {
      const res = await validateHosts(form.hosts_raw, form.hosts_format);
      setValidation(res);
      if (res.ok) message.success(`校验通过，共 ${res.hosts_count} 台主机`);
    } catch {
      setValidation(null);
    } finally {
      setValidating(false);
    }
  };

  const handlePreviewKickstart = async () => {
    if (!form.kickstart_id) return;
    try {
      const res = await getKickstartContent(form.kickstart_id);
      setPreview(res.content);
    } catch {
      setPreview('');
    }
  };

  const canSave = form.name.trim() !== '' && form.iso_id !== '' && form.kickstart_id !== '';

  const handleSave = async () => {
    setSaving(true);
    const payload: ProjectInput = {
      name: form.name.trim(),
      description: form.description,
      arch: form.arch,
      iso_id: form.iso_id,
      kickstart_id: form.kickstart_id,
      iso_label: form.iso_label,
      output_name: form.output_name,
      script_id: form.script_id,
      post_script: form.post_script,
      payload_mode: form.payload_mode,
      hosts_raw: form.hosts_raw,
      hosts_format: form.hosts_format,
    };

    try {
      if (isEdit && id) {
        await updateProject(id, payload);
        message.success('项目已保存');
        history.push(`/projects/${id}`);
      } else {
        const res = await createProject(payload);
        message.success('项目已创建');
        history.push(`/projects/${res.project.id}`);
      }
    } catch {
      // invalid_hosts 会在请求层提示，校验详情在主机清单步骤展示
    } finally {
      setSaving(false);
    }
  };

  return (
    <PageContainer
      header={{
        title: isEdit ? '编辑项目' : '新建项目',
        subTitle: `第 ${step + 1} / ${STEPS.length} 步 · ${STEPS[step].title}`,
        onBack: () => history.push(isEdit && id ? `/projects/${id}` : '/projects'),
      }}
    >
      <Steps
        current={step}
        size="small"
        style={{ marginBottom: 24 }}
        items={STEPS.map((s) => ({ title: s.title }))}
      />

      <ProCard>
        {!loaded && <Skeleton active paragraph={{ rows: 6 }} />}

        {loaded && step === 0 && (
          <ProForm submitter={false} initialValues={form} onValuesChange={(_, all) => patch(all)}>
            <ProFormText
              name="name"
              label="项目名称"
              rules={[{ required: true, message: '请输入项目名称' }]}
            />
            <ProFormText name="description" label="描述" />
            <ProFormRadio.Group
              name="arch"
              label="目标架构"
              options={ARCH_OPTIONS}
              rules={[{ required: true }]}
            />
          </ProForm>
        )}

        {loaded && step === 1 && (
          <ProForm submitter={false} initialValues={form} onValuesChange={(_, all) => patch(all)}>
            <ProFormSelect
              name="iso_id"
              label="基础镜像"
              options={isoOptions}
              rules={[{ required: true, message: '请选择一个可用的镜像' }]}
              extra="只列出「可用」状态的镜像；其余镜像需等下载/校验完成"
              placeholder="选择镜像"
            />
          </ProForm>
        )}

        {loaded && step === 2 && (
          <>
            <ProForm submitter={false} initialValues={form} onValuesChange={(_, all) => patch(all)}>
              <ProFormSelect
                name="kickstart_id"
                label="kickstart 模板"
                options={kickstartOptions}
                rules={[{ required: true, message: '请选择应答模板' }]}
                placeholder="选择模板"
              />
            </ProForm>
            <Space>
              <Button disabled={!form.kickstart_id} onClick={handlePreviewKickstart}>
                预览模板内容
              </Button>
            </Space>
            {preview && (
              <pre
                style={{
                  marginTop: 12,
                  padding: 12,
                  background: '#f6f8fa',
                  borderRadius: 6,
                  fontSize: 12,
                  maxHeight: 320,
                  overflow: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {preview}
              </pre>
            )}
          </>
        )}

        {loaded && step === 3 && (
          <div>
            <Space style={{ marginBottom: 12 }} wrap>
              <span>格式：</span>
              <Radio.Group
                value={form.hosts_format}
                onChange={(e) => patch({ hosts_format: e.target.value })}
                optionType="button"
                options={HOSTS_FORMAT_OPTIONS}
              />
              <Button type="primary" loading={validating} onClick={handleValidate}>
                校验主机清单
              </Button>
            </Space>

            <Input.TextArea
              value={form.hosts_raw}
              onChange={(e) => patch({ hosts_raw: e.target.value })}
              rows={14}
              style={{ fontFamily: 'monospace', fontSize: 12 }}
              placeholder={`hosts:\n  - sn: YS2B60105814\n    hostname: perf001\n    network:\n      ip: 172.100.7.1\n      netmask: 255.255.255.0`}
            />

            {validation && (
              <div style={{ marginTop: 12 }}>
                {validation.ok ? (
                  <Alert
                    type="success"
                    showIcon
                    message={`校验通过：${validation.hosts_count} 台主机（${validation.format}）`}
                  />
                ) : (
                  <Alert
                    type="error"
                    showIcon
                    message={`校验失败：${validation.errors.length} 处问题`}
                    description={
                      <ul style={{ margin: 0, paddingLeft: 18 }}>
                        {validation.errors.slice(0, 20).map((err, idx) => (
                          <li key={idx}>
                            第 {err.index + 1} 台 · <code>{err.field}</code>：{err.message}
                          </li>
                        ))}
                      </ul>
                    }
                  />
                )}

                {validation.hosts_count > 0 && (
                  <div style={{ marginTop: 12 }}>
                    <Space size={6} wrap>
                      {validation.hosts_summary.map((h) => (
                        <Tag key={h.sn}>
                          {h.hostname} · {h.ip}
                        </Tag>
                      ))}
                    </Space>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {loaded && step === 4 && (
          <ProForm submitter={false} initialValues={form} onValuesChange={(_, all) => patch(all)}>
            <ProFormSelect
              name="script_id"
              label="引用脚本库"
              options={scriptOptions}
              allowClear
              placeholder="可选"
              extra="库脚本在前，项目追加脚本在后，合并后写入独立的 %post 段"
            />
            <ProFormTextArea
              name="post_script"
              label="项目追加脚本"
              fieldProps={{ rows: 10, style: { fontFamily: 'monospace', fontSize: 12 } }}
              placeholder="可选，留空表示不追加"
            />
          </ProForm>
        )}

        {loaded && step === 5 && (
          <ProForm submitter={false} initialValues={form} onValuesChange={(_, all) => patch(all)}>
            <ProFormText
              name="iso_label"
              label="ISO 卷标"
              placeholder="留空则构建时从原版 ISO 自动探测"
            />
            <ProFormText name="output_name" label="产物文件名" placeholder="留空则自动生成" />
            <ProFormRadio.Group
              name="payload_mode"
              label="U 盘文件注入"
              options={PAYLOAD_MODE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))}
              extra="开启后模板的 %%PAYLOAD_SNIPPET%% 占位符会被替换为对应逻辑"
            />
          </ProForm>
        )}

        <div style={{ marginTop: 24, display: 'flex', justifyContent: 'space-between' }}>
          <Button disabled={step === 0} onClick={() => setStep((s) => s - 1)}>
            上一步
          </Button>

          <Space>
            <Button onClick={() => history.push('/projects')}>取消</Button>
            {step < STEPS.length - 1 ? (
              <Button type="primary" onClick={() => setStep((s) => s + 1)}>
                下一步
              </Button>
            ) : (
              <Button type="primary" loading={saving} disabled={!canSave} onClick={handleSave}>
                {isEdit ? '保存' : '创建项目'}
              </Button>
            )}
          </Space>
        </div>
      </ProCard>
    </PageContainer>
  );
};

export default ProjectWizardPage;
