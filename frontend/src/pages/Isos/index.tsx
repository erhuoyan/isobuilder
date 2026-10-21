import { LinkOutlined, PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Popconfirm, Space, Tag, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import { ARCH_OPTIONS, ISO_SOURCE, ISO_STATUS } from '@/constants';
import { deleteIso, listIsos, registerIso } from '@/services/isobuilder';
import type { Iso } from '@/services/types';
import { formatBytes, formatDateTime } from '@/utils/format';

const POLL_INTERVAL = 5000;

/** 还在流转中的镜像需要靠轮询才能看到状态变化。 */
const TRANSITIONAL: Record<string, true> = { downloading: true, hashing: true };

type RegisterMode = 'url' | 'path';

const IsosPage: React.FC = () => {
  const actionRef = useRef<ActionType>(null);
  const [isos, setIsos] = useState<Iso[]>([]);
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<RegisterMode>('url');

  const loadIsos = async () => {
    const res = await listIsos();
    setIsos(res.isos ?? []);
  };

  useEffect(() => {
    void loadIsos();
  }, []);

  const hasTransitional = isos.some((iso) => TRANSITIONAL[iso.status]);

  // 只在有镜像下载/校验时轮询，状态稳定后自动停
  useEffect(() => {
    if (!hasTransitional) return;

    const timer = setInterval(() => void loadIsos(), POLL_INTERVAL);
    return () => clearInterval(timer);
  }, [hasTransitional]);

  const handleSubmit = async (values: Record<string, unknown>): Promise<boolean> => {
    try {
      if (mode === 'path') {
        await registerIso({
          source: 'path',
          path: String(values.path ?? ''),
          name: values.name ? String(values.name) : undefined,
          arch: String(values.arch),
        });
      } else {
        await registerIso({
          source: 'url',
          url: String(values.url ?? ''),
          name: values.name ? String(values.name) : undefined,
          arch: String(values.arch),
        });
      }

      message.success('已提交，服务端开始下载');
      void loadIsos();
      return true;
    } catch {
      return false;
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteIso(id);
      message.success('已删除');
      void loadIsos();
    } catch {
      // 请求层已提示；被项目引用时是 409 iso_in_use
    }
  };

  const columns: ProColumns<Iso>[] = [
    {
      title: '名称',
      dataIndex: 'name',
      width: 260,
      render: (_, record) => (
        <div>
          <div>{record.name}</div>
          <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12, fontFamily: 'monospace' }}>
            {record.filename}
          </div>
        </div>
      ),
    },
    { title: '架构', dataIndex: 'arch', width: 100, render: (_, r) => <code>{r.arch}</code> },
    { title: '大小', dataIndex: 'size_bytes', width: 110, render: (_, r) => formatBytes(r.size_bytes) },
    {
      title: '状态',
      dataIndex: 'status',
      width: 150,
      render: (_, record) => {
        const info = ISO_STATUS[record.status];
        return (
          <Space direction="vertical" size={0}>
            <Tag color={info?.color ?? 'default'}>{info?.label ?? record.status}</Tag>
            {record.error && (
              <span style={{ color: '#cf1322', fontSize: 12 }}>{record.error}</span>
            )}
          </Space>
        );
      },
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 110,
      render: (_, record) => {
        const info = ISO_SOURCE[record.source];
        return <Tag color={info?.color ?? 'default'}>{info?.label ?? record.source}</Tag>;
      },
    },
    {
      title: 'SHA256',
      dataIndex: 'sha256',
      width: 190,
      render: (_, record) =>
        record.sha256 ? (
          <code style={{ fontSize: 12 }}>{record.sha256.slice(0, 16)}…</code>
        ) : (
          <span style={{ color: 'rgba(0,0,0,0.35)' }}>—</span>
        ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 170,
      render: (_, r) => formatDateTime(r.created_at),
    },
    {
      title: '操作',
      width: 90,
      fixed: 'right',
      render: (_, record) => (
        <Popconfirm
          title={`删除镜像「${record.name}」？`}
          okText="删除"
          cancelText="取消"
          onConfirm={() => handleDelete(record.id)}
        >
          <Button type="link" size="small" danger>
            删除
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <PageContainer
      header={{
        title: '镜像库',
        subTitle: '提供一个下载链接，服务端自己拉取镜像；也可注册服务器上已有的文件',
        extra: [
          <Button
            key="path"
            icon={<PlusOutlined />}
            onClick={() => {
              setMode('path');
              setOpen(true);
            }}
          >
            本地注册
          </Button>,
          <Button
            key="url"
            type="primary"
            icon={<LinkOutlined />}
            onClick={() => {
              setMode('url');
              setOpen(true);
            }}
          >
            链接导入
          </Button>,
        ],
      }}
    >
      <ProTable<Iso>
        actionRef={actionRef}
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        dataSource={isos}
        columns={columns}
        scroll={{ x: 1200 }}
        locale={{ emptyText: '镜像库还是空的，先用链接导入一个基础 ISO' }}
      />

      <ModalForm
        title={mode === 'url' ? '链接导入镜像' : '本地注册镜像'}
        open={open}
        onOpenChange={setOpen}
        onFinish={handleSubmit}
        modalProps={{ destroyOnClose: true }}
        initialValues={{ arch: 'x86_64' }}
      >
        <ProFormSelect name="arch" label="架构" width="xs" options={ARCH_OPTIONS} />
        <ProFormText name="name" label="镜像名称" placeholder="留空则使用链接里的文件名" />

        {mode === 'url' ? (
          <ProFormText
            name="url"
            label="下载链接"
            placeholder="https://mirrors.example.com/Rocky-9.8-x86_64-dvd.iso"
            rules={[
              { required: true, message: '请输入下载链接' },
              { pattern: /^https?:\/\/.+/, message: '必须是 http:// 或 https:// 开头的完整地址' },
            ]}
            extra="对象存储的公开链接或预签名链接同样适用；服务端会在后台下载并计算 SHA256"
          />
        ) : (
          <ProFormText
            name="path"
            label="服务器上的绝对路径"
            placeholder="/var/lib/isobuilder/data/isos/Rocky-9.8-x86_64-dvd.iso"
            rules={[{ required: true, message: '请输入 ISO 绝对路径' }]}
            extra="文件必须已在数据目录内，服务端会直接读取并计算 SHA256"
          />
        )}
      </ModalForm>
    </PageContainer>
  );
};

export default IsosPage;
