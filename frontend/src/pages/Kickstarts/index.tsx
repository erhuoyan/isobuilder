import { UploadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import {
  Button,
  Drawer,
  Popconfirm,
  Space,
  Tag,
  Upload,
  message,
} from 'antd';
import type { UploadFile } from 'antd';
import React, { useEffect, useState } from 'react';
import { KICKSTART_ARCH_OPTIONS } from '@/constants';
import {
  deleteKickstart,
  getKickstartContent,
  listKickstarts,
  uploadKickstart,
} from '@/services/isobuilder';
import type { Kickstart } from '@/services/types';
import { formatBytes, formatDateTime } from '@/utils/format';

const KickstartsPage: React.FC = () => {
  const [kickstarts, setKickstarts] = useState<Kickstart[]>([]);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [preview, setPreview] = useState<{ name: string; content: string } | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const load = async () => {
    const res = await listKickstarts();
    setKickstarts(res.kickstarts ?? []);
  };

  useEffect(() => {
    void load();
  }, []);

  const handleUpload = async (values: Record<string, unknown>): Promise<boolean> => {
    const file = fileList[0]?.originFileObj;
    if (!file) {
      message.error('请选择 .ks 文件');
      return false;
    }

    try {
      await uploadKickstart(file, String(values.name || file.name), String(values.arch));
      message.success('模板已上传');
      setFileList([]);
      void load();
      return true;
    } catch {
      return false;
    }
  };

  const handlePreview = async (record: Kickstart) => {
    setPreview({ name: record.name, content: '' });
    setPreviewLoading(true);
    try {
      const res = await getKickstartContent(record.id);
      setPreview({ name: record.name, content: res.content });
    } catch {
      setPreview(null);
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleDelete = async (record: Kickstart) => {
    try {
      await deleteKickstart(record.id);
      message.success('已删除');
      void load();
    } catch {
      // 请求层已提示；内置模板或被项目引用时返回 409
    }
  };

  const columns: ProColumns<Kickstart>[] = [
    {
      title: '名称',
      dataIndex: 'name',
      width: 260,
      render: (_, record) => (
        <Space size={6}>
          <a onClick={() => handlePreview(record)}>{record.name}</a>
          {record.builtin && <Tag color="blue">内置</Tag>}
        </Space>
      ),
    },
    { title: '架构', dataIndex: 'arch', width: 110, render: (_, r) => <code>{r.arch}</code> },
    {
      title: '占位符',
      dataIndex: 'placeholders',
      width: 280,
      render: (_, record) =>
        record.placeholders?.length ? (
          <Space size={4} wrap>
            {record.placeholders.map((p) => (
              <Tag key={p} style={{ fontFamily: 'monospace', fontSize: 12 }}>
                {p}
              </Tag>
            ))}
          </Space>
        ) : (
          <span style={{ color: 'rgba(0,0,0,0.35)' }}>—</span>
        ),
    },
    {
      title: '大小',
      dataIndex: 'size_bytes',
      width: 100,
      render: (_, r) => formatBytes(r.size_bytes),
    },
    {
      title: '上传时间',
      dataIndex: 'created_at',
      width: 170,
      render: (_, r) => formatDateTime(r.created_at),
    },
    {
      title: '操作',
      width: 140,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Button type="link" size="small" onClick={() => handlePreview(record)}>
            预览
          </Button>
          <Popconfirm
            title={`删除模板「${record.name}」？`}
            okText="删除"
            cancelText="取消"
            onConfirm={() => handleDelete(record)}
            disabled={record.builtin}
          >
            <Button type="link" size="small" danger disabled={record.builtin}>
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
        title: '应答模板',
        subTitle: '内置与自定义的 kickstart 模板，构建时注入主机清单',
        extra: [
          <Button key="upload" type="primary" icon={<UploadOutlined />} onClick={() => setUploadOpen(true)}>
            上传模板
          </Button>,
        ],
      }}
    >
      <ProTable<Kickstart>
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        dataSource={kickstarts}
        columns={columns}
        scroll={{ x: 1060 }}
        locale={{ emptyText: '还没有模板，先上传一个 .ks 文件' }}
      />

      <ModalForm
        title="上传 kickstart 模板"
        open={uploadOpen}
        onOpenChange={(open) => {
          setUploadOpen(open);
          if (!open) setFileList([]);
        }}
        onFinish={handleUpload}
        modalProps={{ destroyOnClose: true }}
        initialValues={{ arch: 'any' }}
      >
        <ProFormText name="name" label="模板名称" placeholder="留空则使用文件名" />
        <ProFormSelect name="arch" label="适用架构" width="xs" options={KICKSTART_ARCH_OPTIONS} />
        <Upload
          accept=".ks"
          maxCount={1}
          fileList={fileList}
          beforeUpload={() => false}
          onChange={({ fileList: next }) => setFileList(next.slice(-1))}
        >
          <Button icon={<UploadOutlined />}>选择 .ks 文件</Button>
        </Upload>
        <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12, marginTop: 8 }}>
          模板必须包含 <code>%%HOSTS_JSON_PLACEHOLDER%%</code> 占位符，构建时会被替换为主机清单。
        </div>
      </ModalForm>

      <Drawer
        title={`预览：${preview?.name ?? ''}`}
        open={!!preview}
        onClose={() => setPreview(null)}
        width={720}
      >
        {previewLoading ? (
          <div>加载中…</div>
        ) : (
          <pre
            style={{
              fontFamily: 'monospace',
              fontSize: 12,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
              margin: 0,
            }}
          >
            {preview?.content}
          </pre>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default KickstartsPage;
