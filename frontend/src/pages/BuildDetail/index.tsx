import { ArrowLeftOutlined, DownloadOutlined, ReloadOutlined, StopOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { Alert, Button, Descriptions, Popconfirm, Progress, Space, Tag, Tooltip, message } from 'antd';
import React, { useCallback, useEffect, useState } from 'react';
import BuildLogViewer from '@/components/BuildLogViewer';
import BuildStatusTag from '@/components/BuildStatusTag';
import { BUILD_STAGES, isBuildActive } from '@/constants';
import { cancelBuild, getBuild, retryBuild, fetchArtifact } from '@/services/isobuilder';
import type { Build } from '@/services/types';
import { formatBytes, formatDateTime, formatDuration } from '@/utils/format';

const BuildDetailPage: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>();
  const [build, setBuild] = useState<Build | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [acting, setActing] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await getBuild(id);
      setBuild(res.build);
    } catch {
      setNotFound(true);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const active = build ? isBuildActive(build.status) : false;

  // 构建期间轮询状态；日志本身由 BuildLogViewer 自己流式接收
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(() => void load(), 2000);
    return () => clearInterval(timer);
  }, [active, load]);

  const handleCancel = async () => {
    setActing(true);
    try {
      await cancelBuild(id);
      message.success('已取消构建');
      await load();
    } catch {
      // build_finished：已经结束了
    } finally {
      setActing(false);
    }
  };

  const handleRetry = async () => {
    setActing(true);
    try {
      const res = await retryBuild(id);
      message.success('已重新入队');
      history.push(`/builds/${res.build.id}`);
    } catch {
      // 请求层已提示
    } finally {
      setActing(false);
    }
  };

  /** 预签名链接会过期，重新签一次再跳转 */
  const handleDownload = async () => {
    if (!build?.artifact) return;
    try {
      const artifact = await fetchArtifact(build.artifact.id);
      window.location.href = artifact.url || `/api/artifacts/${build.artifact.id}/download`;
    } catch {
      // 请求层已提示
    }
  };

  if (notFound) {
    return (
      <PageContainer>
        <Alert
          type="error"
          showIcon
          message="构建不存在"
          action={
            <Button type="primary" onClick={() => history.push('/projects')}>
              返回项目列表
            </Button>
          }
        />
      </PageContainer>
    );
  }

  const stage = BUILD_STAGES.find((s) => s.key === build?.stage);

  return (
    <PageContainer
      header={{
        title: `构建 ${id}`,
        subTitle: build?.project_name,
        onBack: () => history.push(build ? `/projects/${build.project_id}` : '/projects'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Button key="refresh" icon={<ReloadOutlined />} onClick={() => void load()}>
            刷新
          </Button>,
          active ? (
            <Popconfirm
              key="cancel"
              title="取消这个构建？"
              okText="取消构建"
              cancelText="返回"
              onConfirm={handleCancel}
            >
              <Button danger icon={<StopOutlined />} loading={acting}>
                取消
              </Button>
            </Popconfirm>
          ) : (
            <Button key="retry" icon={<ReloadOutlined />} loading={acting} onClick={handleRetry}>
              重新构建
            </Button>
          ),
          build?.artifact && (
            <Button key="download" type="primary" icon={<DownloadOutlined />} onClick={handleDownload}>
              下载 ISO
            </Button>
          ),
        ],
      }}
    >
      <ProCard style={{ marginBottom: 16 }}>
        <div style={{ marginBottom: 16 }}>
          <Space size={12} align="center">
            <BuildStatusTag status={build?.status ?? 'queued'} />
            {build && (
              <span style={{ color: 'rgba(0,0,0,0.65)' }}>
                {build.stage_index}/{build.stage_total}
                {stage ? ` · ${stage.label}` : ' · 未开始'}
              </span>
            )}
          </Space>
        </div>

        <Progress
          percent={build?.progress ?? 0}
          status={build?.status === 'failed' ? 'exception' : undefined}
        />

        {build?.error && (
          <Alert type="error" showIcon style={{ marginTop: 16 }} message={build.error} />
        )}

        <Descriptions column={3} colon={false} style={{ marginTop: 16 }}>
          <Descriptions.Item label="项目">
            <a onClick={() => history.push(`/projects/${build?.project_id}`)}>{build?.project_name}</a>
          </Descriptions.Item>
          <Descriptions.Item label="架构">{build?.arch}</Descriptions.Item>
          <Descriptions.Item label="基础镜像">{build?.iso_name}</Descriptions.Item>
          <Descriptions.Item label="ISO 卷标">{build?.iso_label || '—'}</Descriptions.Item>
          <Descriptions.Item label="产物文件名">{build?.output_name || '—'}</Descriptions.Item>
          <Descriptions.Item label="主机台数">{build?.hosts_count ?? 0} 台</Descriptions.Item>
          <Descriptions.Item label="创建时间">{formatDateTime(build?.created_at ?? '')}</Descriptions.Item>
          <Descriptions.Item label="开始时间">{formatDateTime(build?.started_at ?? '')}</Descriptions.Item>
          <Descriptions.Item label="耗时">
            {formatDuration(build?.started_at ?? '', build?.finished_at ?? '')}
          </Descriptions.Item>
        </Descriptions>
      </ProCard>

      {build?.artifact && (
        <ProCard title="产物" style={{ marginBottom: 16 }}>
          <Descriptions column={2} colon={false}>
            <Descriptions.Item label="文件名">{build.artifact.filename}</Descriptions.Item>
            <Descriptions.Item label="大小">{formatBytes(build.artifact.size_bytes)}</Descriptions.Item>
            <Descriptions.Item label="SHA256">
              <code style={{ fontSize: 12, wordBreak: 'break-all' }}>{build.artifact.sha256}</code>
            </Descriptions.Item>
            <Descriptions.Item label="存储">
              <Tag color={build.artifact.storage === 's3' ? 'blue' : 'default'}>
                {build.artifact.storage === 's3' ? '对象存储' : '本地存储'}
              </Tag>
              {build.artifact.url_expires_at && (
                <Tooltip title="链接过期后点「下载 ISO」会自动续签">
                  <span style={{ marginLeft: 8, color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>
                    至 {formatDateTime(build.artifact.url_expires_at)}
                  </span>
                </Tooltip>
              )}
            </Descriptions.Item>
          </Descriptions>
        </ProCard>
      )}

      <ProCard
        title="构建日志"
        extra={<span style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>{formatBytes(build?.log_bytes ?? 0)}</span>}
      >
        <BuildLogViewer
          buildId={id}
          finished={!active}
          onStatus={() => {
            void load();
          }}
        />
      </ProCard>
    </PageContainer>
  );
};

export default BuildDetailPage;
