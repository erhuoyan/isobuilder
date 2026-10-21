import { PageContainer, ProCard, ProForm, ProFormSwitch, ProFormText } from '@ant-design/pro-components';
import { Alert, Col, Descriptions, Row, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import { fetchConfig, fetchSettings, updateSettings } from '@/services/isobuilder';
import type { Settings, SystemConfig } from '@/services/types';
import { formatBytes } from '@/utils/format';

interface StorageFormValues {
  endpoint?: string;
  region?: string;
  bucket?: string;
  prefix?: string;
  public_base?: string;
  presign_ttl_seconds?: number;
  path_style?: boolean;
  insecure_tls?: boolean;
  access_key?: string;
  secret_key?: string;
}

const SettingsPage: React.FC = () => {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [config, setConfig] = useState<SystemConfig | null>(null);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    const [settingsRes, configRes] = await Promise.all([fetchSettings(), fetchConfig()]);
    setSettings(settingsRes);
    setConfig(configRes);
  };

  useEffect(() => {
    void load();
  }, []);

  const handleSubmit = async (values: StorageFormValues): Promise<boolean> => {
    setSaving(true);
    try {
      await updateSettings({
        ...values,
        // 空串表示「保持不变」，而不是清空凭据
        access_key: values.access_key || undefined,
        secret_key: values.secret_key || undefined,
      });
      message.success('配置已保存并立即生效');
      await load();
      return true;
    } catch {
      // 保存前会用 HeadBucket 探活，连不上返回 400 storage_unreachable（配置已保存）
      await load();
      return false;
    } finally {
      setSaving(false);
    }
  };

  const storage = settings?.storage;

  return (
    <PageContainer
      header={{
        title: '系统配置',
        subTitle: '维护对象存储（S3 兼容）配置，保存后立即生效，无需重启服务',
      }}
    >
      <ProCard title="当前运行信息" style={{ marginBottom: 16 }}>
        <Descriptions column={3} colon={false}>
          <Descriptions.Item label="版本">{config?.version ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="对象存储">
            {config?.storage?.enabled ? (
              <Tag color="green">
                {config.storage.driver} · {config.storage.bucket}
              </Tag>
            ) : (
              <Tag>未启用（产物存本机）</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="最大并发构建">{config?.max_concurrent_builds ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="数据目录"><code>{config?.disk?.data_dir ?? '—'}</code></Descriptions.Item>
          <Descriptions.Item label="剩余空间">
            {config?.disk ? formatBytes(config.disk.free_bytes) : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="需要认证">
            {config?.auth_required ? <Tag color="orange">是</Tag> : <Tag>否</Tag>}
          </Descriptions.Item>
        </Descriptions>
      </ProCard>

      <ProCard title="对象存储">
        {storage && (
          <Alert
            type={storage.enabled ? 'success' : 'warning'}
            showIcon
            style={{ marginBottom: 16 }}
            message={
              storage.enabled
                ? `已启用：${storage.endpoint} / ${storage.bucket}（来源：${storage.source === 'db' ? '配置页' : '环境变量'}）`
                : '未启用：清空 Bucket 即关闭对象存储，构建产物改存本机'
            }
            description={
              storage.enabled
                ? `Access Key ${storage.access_key_set ? '已配置' : '未配置'} · Secret Key ${storage.secret_key_set ? '已配置' : '未配置'}`
                : undefined
            }
          />
        )}

        <ProForm<StorageFormValues>
          loading={!settings}
          submitter={{
            searchConfig: { submitText: '保存并生效' },
            submitButtonProps: { loading: saving },
            resetButtonProps: { style: { display: 'none' } },
          }}
          onFinish={handleSubmit}
          initialValues={
            storage
              ? {
                  endpoint: storage.endpoint,
                  region: storage.region,
                  bucket: storage.bucket,
                  prefix: storage.prefix,
                  public_base: storage.public_base,
                  presign_ttl_seconds: storage.presign_ttl_seconds,
                  path_style: storage.path_style,
                  insecure_tls: storage.insecure_tls,
                  access_key: '',
                  secret_key: '',
                }
              : {}
          }
        >
          <Row gutter={16}>
            <Col span={12}>
              <ProFormText
                name="endpoint"
                label="Endpoint"
                placeholder="https://oss.example.com"
                fieldProps={{ style: { fontFamily: 'monospace' } }}
              />
            </Col>
            <Col span={12}>
              <ProFormText
                name="region"
                label="Region"
                placeholder="us-east-1"
                fieldProps={{ style: { fontFamily: 'monospace' } }}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <ProFormText
                name="bucket"
                label="Bucket"
                placeholder="留空 = 关闭对象存储，产物存本机"
                fieldProps={{ style: { fontFamily: 'monospace' } }}
              />
            </Col>
            <Col span={12}>
              <ProFormText
                name="prefix"
                label="Prefix"
                placeholder="auto-iso"
                fieldProps={{ style: { fontFamily: 'monospace' } }}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <ProFormText
                name="public_base"
                label="Public base"
                placeholder="可选，CDN 加速域名"
                fieldProps={{ style: { fontFamily: 'monospace' } }}
              />
            </Col>
            <Col span={12}>
              <ProFormText
                name="presign_ttl_seconds"
                label="预签名有效期（秒）"
                placeholder="86400"
                fieldProps={{ style: { fontFamily: 'monospace' } }}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <ProFormText.Password
                name="access_key"
                label="Access Key"
                placeholder={storage?.access_key_set ? '已配置，留空保持不变' : '未配置'}
              />
            </Col>
            <Col span={12}>
              <ProFormText.Password
                name="secret_key"
                label="Secret Key"
                placeholder={storage?.secret_key_set ? '已配置，留空保持不变' : '未配置'}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <ProFormSwitch name="path_style" label="Path style 寻址" />
            </Col>
            <Col span={12}>
              <ProFormSwitch name="insecure_tls" label="跳过 TLS 校验" />
            </Col>
          </Row>
        </ProForm>
      </ProCard>
    </PageContainer>
  );
};

export default SettingsPage;
