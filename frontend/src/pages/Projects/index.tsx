import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { history, useRequest } from '@umijs/max';
import { Button, Popconfirm, Space, Tag, Tooltip, message } from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { BUILD_STATUS, ISO_STATUS, isBuildActive } from '@/constants';
import { deleteProject, listBuilds, listProjects } from '@/services/isobuilder';
import type { Build, Project } from '@/services/types';
import { formatDateTime } from '@/utils/format';
import BuildStatusTag from '@/components/BuildStatusTag';

const POLL_INTERVAL = 3000;

const ProjectsPage: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [projects, setProjects] = useState<Project[]>([]);
  const [latestBuilds, setLatestBuilds] = useState<Record<string, Build>>({});

  const load = async () => {
    const [projectRes, buildRes] = await Promise.all([
      listProjects(),
      listBuilds({ limit: 200 }),
    ]);

    setProjects(projectRes.projects ?? []);

    // 构建按创建时间倒序，所以每个项目的第一条就是它最近一次的构建
    const record: Record<string, Build> = {};
    (buildRes.builds ?? []).forEach((build) => {
      if (!record[build.project_id]) record[build.project_id] = build;
    });
    setLatestBuilds(record);
  };

  useRequest(load);

  const hasActiveBuild = useMemo(
    () => Object.values(latestBuilds).some((build) => isBuildActive(build.status)),
    [latestBuilds],
  );

  // 有构建在跑时轮询，全部结束后停掉，避免空转
  useEffect(() => {
    if (!hasActiveBuild) return;

    const timer = setInterval(() => {
      void load();
    }, POLL_INTERVAL);

    return () => clearInterval(timer);
  }, [hasActiveBuild]);

  const handleDelete = async (id: string) => {
    try {
      await deleteProject(id);
      message.success('项目已删除');
      await load();
    } catch {
      // 请求层已提示（被引用时是 409 project_busy）
    }
  };

  const columns: ProColumns<Project>[] = [
    {
      title: '项目',
      dataIndex: 'name',
      width: 240,
      render: (_, record) => (
        <div>
          <a onClick={() => history.push(`/projects/${record.id}`)}>{record.name}</a>
          <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>
            {record.description || '—'}
          </div>
        </div>
      ),
    },
    {
      title: '架构',
      dataIndex: 'arch',
      width: 100,
      render: (_, record) => <code>{record.arch}</code>,
    },
    {
      title: '基础镜像',
      dataIndex: 'iso_id',
      width: 240,
      render: (_, record) =>
        record.iso ? (
          <div>
            <div style={{ maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {record.iso.name}
            </div>
            <div
              style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12, fontFamily: 'monospace' }}
            >
              {record.iso.filename}
            </div>
          </div>
        ) : (
          <Tag color="red">镜像已删除</Tag>
        ),
    },
    {
      title: '主机台数',
      dataIndex: 'hosts_count',
      width: 110,
      render: (_, record) => <code>{record.hosts_count} 台</code>,
    },
    {
      title: '最近构建',
      width: 260,
      render: (_, record) => {
        const build = latestBuilds[record.id];
        if (!build) return <span style={{ color: 'rgba(0,0,0,0.35)' }}>尚未构建</span>;

        return (
          <Space size={8}>
            <BuildStatusTag status={build.status} />
            <Tooltip title={formatDateTime(build.created_at)}>
              <span style={{ color: 'rgba(0,0,0,0.45)', fontSize: 13 }}>
                {formatDateTime(build.created_at).slice(5, 16)}
              </span>
            </Tooltip>
            {build.status === 'running' && <code>{build.progress}%</code>}
          </Space>
        );
      },
    },
    {
      title: '操作',
      width: 180,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Button type="link" size="small" onClick={() => history.push(`/projects/${record.id}`)}>
            进入
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() => history.push(`/projects/${record.id}/edit`)}
          >
            编辑
          </Button>
          <Popconfirm
            title={`删除项目「${record.name}」及其全部构建记录？`}
            okText="删除"
            cancelText="取消"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      header={{
        title: '装机项目',
        subTitle: '每个项目对应一次 ISO 定制：绑定基础镜像与 kickstart 应答文件，并携带一份主机清单',
        extra: [
          <Button
            key="refresh"
            icon={<ReloadOutlined />}
            onClick={() => {
              void load();
              actionRef.current?.reload();
            }}
          >
            刷新
          </Button>,
          <Button
            key="create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => history.push('/projects/new')}
          >
            新建项目
          </Button>,
        ],
      }}
    >
      {hasActiveBuild && (
        <div style={{ marginBottom: 16 }}>
          <Tag color="processing">有构建正在进行，列表每 3 秒自动刷新</Tag>
        </div>
      )}

      <ProTable<Project>
        actionRef={actionRef}
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        dataSource={projects}
        columns={columns}
        scroll={{ x: 1140 }}
        locale={{ emptyText: '还没有项目，先创建一个吧' }}
      />
    </PageContainer>
  );
};

export default ProjectsPage;
