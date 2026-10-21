import { ArrowLeftOutlined, EditOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { PageContainer, ProCard, ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { Alert, Button, Descriptions, Popconfirm, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import BuildStatusTag from '@/components/BuildStatusTag';
import { BUILD_STAGES, ISO_STATUS, isBuildActive } from '@/constants';
import { createBuild, deleteProject, getProject, listBuilds } from '@/services/isobuilder';
import type { Build, HostSummary, Project } from '@/services/types';
import { formatBytes, formatDateTime, formatDuration } from '@/utils/format';

const ProjectDetailPage: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);
  const [builds, setBuilds] = useState<Build[]>([]);
  const [building, setBuilding] = useState(false);
  const [notFound, setNotFound] = useState(false);

  const load = async () => {
    try {
      const res = await getProject(id);
      setProject(res.project);
      const buildRes = await listBuilds({ project_id: id, limit: 50 });
      setBuilds(buildRes.builds ?? []);
    } catch {
      setNotFound(true);
    }
  };

  useEffect(() => {
    void load();
  }, [id]);

  const hasActiveBuild = builds.some((b) => isBuildActive(b.status));

  useEffect(() => {
    if (!hasActiveBuild) return;
    const timer = setInterval(() => void load(), 3000);
    return () => clearInterval(timer);
  }, [hasActiveBuild]);

  const handleBuild = async () => {
    setBuilding(true);
    try {
      const res = await createBuild(id);
      message.success('已入队，开始构建');
      history.push(`/builds/${res.build.id}`);
    } catch {
      // project_incomplete 会说明缺少什么
    } finally {
      setBuilding(false);
    }
  };

  const handleDelete = async () => {
    try {
      await deleteProject(id);
      message.success('项目已删除');
      history.push('/projects');
    } catch {
      // project_busy：还有构建在跑
    }
  };

  if (notFound) {
    return (
      <PageContainer>
        <Alert
          type="error"
          showIcon
          message="项目不存在"
          description="该项目可能已被删除。"
          action={
            <Button onClick={() => history.push('/projects')} type="primary">
              返回项目列表
            </Button>
          }
        />
      </PageContainer>
    );
  }

  const missing: string[] = [];
  if (project) {
    if (!project.iso) missing.push('基础镜像');
    if (!project.kickstart) missing.push('kickstart 模板');
    if (!project.hosts_count) missing.push('主机清单');
  }

  const hostColumns: ProColumns<HostSummary>[] = [
    { title: 'SN', dataIndex: 'sn', width: 160, render: (_, r) => <code>{r.sn}</code> },
    { title: 'hostname', dataIndex: 'hostname', width: 140 },
    { title: 'IP', dataIndex: 'ip', width: 140, render: (_, r) => <code>{r.ip}</code> },
    {
      title: '网关',
      dataIndex: 'gateway',
      width: 140,
      render: (_, r) =>
        r.gateway ? <code>{r.gateway}</code> : <span style={{ color: 'rgba(0,0,0,0.35)' }}>—</span>,
    },
    {
      title: '网络',
      dataIndex: 'network_mode',
      width: 110,
      render: (_, r) => (
        <Tag color={r.network_mode === 'bond' ? 'orange' : 'default'}>
          {r.network_mode === 'bond' ? `bond (${r.mac_count})` : '单口'}
        </Tag>
      ),
    },
    {
      title: '目标磁盘',
      dataIndex: 'disk_gb',
      width: 110,
      render: (_, r) => (r.disk_gb ? `${r.disk_gb} GB` : '—'),
    },
    {
      title: 'BMC',
      dataIndex: 'bmc_ip',
      width: 140,
      render: (_, r) =>
        r.bmc_ip ? <code>{r.bmc_ip}</code> : <span style={{ color: 'rgba(0,0,0,0.35)' }}>—</span>,
    },
  ];

  const buildColumns: ProColumns<Build>[] = [
    {
      title: '构建',
      dataIndex: 'id',
      width: 150,
      render: (_, r) => <a onClick={() => history.push(`/builds/${r.id}`)}>{r.id}</a>,
    },
    { title: '状态', dataIndex: 'status', width: 120, render: (_, r) => <BuildStatusTag status={r.status} /> },
    {
      title: '阶段',
      dataIndex: 'stage',
      width: 220,
      render: (_, r) => {
        if (!r.stage) return <span style={{ color: 'rgba(0,0,0,0.35)' }}>未开始</span>;
        const stage = BUILD_STAGES.find((s) => s.key === r.stage);
        return (
          <Space size={6}>
            <span>
              {r.stage_index}/{r.stage_total}
            </span>
            <span>{stage?.label ?? r.stage}</span>
          </Space>
        );
      },
    },
    { title: '进度', dataIndex: 'progress', width: 90, render: (_, r) => `${r.progress}%` },
    {
      title: '耗时',
      width: 120,
      render: (_, r) => formatDuration(r.started_at, r.finished_at),
    },
    {
      title: '开始时间',
      dataIndex: 'created_at',
      width: 170,
      render: (_, r) => formatDateTime(r.created_at),
    },
  ];

  return (
    <PageContainer
      header={{
        title: project?.name ?? '项目详情',
        subTitle: project?.description,
        onBack: () => history.push('/projects'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Button
            key="edit"
            icon={<EditOutlined />}
            onClick={() => history.push(`/projects/${id}/edit`)}
          >
            编辑
          </Button>,
          <Popconfirm
            key="del"
            title={`删除项目「${project?.name ?? ''}」及其全部构建记录？`}
            okText="删除"
            cancelText="取消"
            onConfirm={handleDelete}
          >
            <Button danger>删除</Button>
          </Popconfirm>,
          <Button
            key="build"
            type="primary"
            icon={<PlayCircleOutlined />}
            loading={building}
            disabled={missing.length > 0}
            onClick={handleBuild}
          >
            开始构建
          </Button>,
        ],
      }}
    >
      {missing.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={`项目尚不完整，缺少：${missing.join('、')}`}
          description="补齐后才能开始构建。"
        />
      )}

      <ProCard title="配置" style={{ marginBottom: 16 }}>
        <Descriptions column={3} colon={false}>
          <Descriptions.Item label="架构">{project?.arch}</Descriptions.Item>
          <Descriptions.Item label="基础镜像">
            {project?.iso ? (
              <Space size={6}>
                <span>{project.iso.name}</span>
                <Tag color={ISO_STATUS[project.iso.status]?.color}>
                  {ISO_STATUS[project.iso.status]?.label}
                </Tag>
              </Space>
            ) : (
              <Tag color="red">镜像已删除</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="kickstart">
            {project?.kickstart ? project.kickstart.name : <Tag color="red">未绑定</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="主机台数">{project?.hosts_count ?? 0} 台</Descriptions.Item>
          <Descriptions.Item label="ISO 卷标">
            {project?.iso_label || <span style={{ color: 'rgba(0,0,0,0.35)' }}>构建时自动探测</span>}
          </Descriptions.Item>
          <Descriptions.Item label="产物文件名">
            {project?.output_name || <span style={{ color: 'rgba(0,0,0,0.35)' }}>构建时自动生成</span>}
          </Descriptions.Item>
          <Descriptions.Item label="脚本库">
            {project?.script ? project.script.name : <span style={{ color: 'rgba(0,0,0,0.35)' }}>未引用</span>}
          </Descriptions.Item>
          <Descriptions.Item label="项目追加脚本">
            {project?.post_script ? (
              `${formatBytes(new TextEncoder().encode(project.post_script).length)}`
            ) : (
              <span style={{ color: 'rgba(0,0,0,0.35)' }}>无</span>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="U 盘文件注入">
            {project?.payload_mode === 'off' ? '不注入' : project?.payload_mode}
          </Descriptions.Item>
        </Descriptions>
      </ProCard>

      <ProCard title="构建记录" style={{ marginBottom: 16 }}>
        <ProTable<Build>
          rowKey="id"
          search={false}
          options={false}
          pagination={false}
          dataSource={builds}
          columns={buildColumns}
          scroll={{ x: 1000 }}
          locale={{ emptyText: '还没有构建记录' }}
        />
      </ProCard>

      <ProCard title={`主机清单（${project?.hosts_count ?? 0} 台）`}>
        <ProTable<HostSummary>
          rowKey="sn"
          search={false}
          options={false}
          pagination={false}
          dataSource={project?.hosts_summary ?? []}
          columns={hostColumns}
          scroll={{ x: 950 }}
          locale={{ emptyText: '还没有主机' }}
        />
      </ProCard>
    </PageContainer>
  );
};

export default ProjectDetailPage;
