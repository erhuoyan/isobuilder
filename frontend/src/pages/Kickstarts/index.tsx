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
  Tooltip,
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

interface Props {
  kickstart: Kickstart;
}

/** 后端唯一认的 U 盘注入替换点（builder/pipeline.go 里写死）。 */
const PAYLOAD_PLACEHOLDER = '%%PAYLOAD_SNIPPET%%';

/**
 * 后端会把 %%任意大写%% 都当成占位符列出来，但真正替换的只有两个名字。
 * 其余的原样留在 kickstart 里，装机时静默失效 —— 这种笔误值得当场提示。
 */
const KNOWN_PLACEHOLDERS: Record<string, true> = {
  '%%HOSTS_JSON_PLACEHOLDER%%': true,
  [PAYLOAD_PLACEHOLDER]: true,
};

const unknownPlaceholders = (kickstart: Kickstart) =>
  (kickstart.placeholders ?? []).filter((p) => !KNOWN_PLACEHOLDERS[p]);

/**
 * 模板能力。
 *
 * `%%HOSTS_JSON_PLACEHOLDER%%` 是准入门槛而非特性：模板缺了它上传就被拒、
 * 构建也会失败，所有可用模板必然都有，展示它没有区分度。真正要看的是有没有
 * payload 替换点 —— 有，项目的 payload_mode 才会生效。
 */
const CapabilityCell: React.FC<Props> = ({ kickstart }) => {
  const hasPayload = (kickstart.placeholders ?? []).includes(PAYLOAD_PLACEHOLDER);
  const unknown = unknownPlaceholders(kickstart);

  return (
    <Space size={6} wrap>
      {hasPayload ? (
        <Tag color="blue">支持 U 盘注入</Tag>
      ) : (
        <span style={{ color: 'rgba(0,0,0,0.35)' }}>—</span>
      )}
      {unknown.length > 0 && (
        <Tooltip
          title={`这些占位符构建时不会被替换，会原样留在 kickstart 里：${unknown.join('、')}`}
        >
          <Tag color="red">{unknown.length} 个未知占位符</Tag>
        </Tooltip>
      )}
    </Space>
  );
};

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
      width: 240,
      render: (_, record) => (
        <Space size={6}>
          <a onClick={() => handlePreview(record)}>{record.name}</a>
          {record.builtin && <Tag color="blue">内置</Tag>}
        </Space>
      ),
    },
    { title: '架构', dataIndex: 'arch', width: 100, render: (_, r) => <code>{r.arch}</code> },
    {
      title: '能力',
      dataIndex: 'placeholders',
      width: 220,
      render: (_, record) => <CapabilityCell kickstart={record} />,
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
          <Button
            key="upload"
            type="primary"
            icon={<UploadOutlined />}
            onClick={() => setUploadOpen(true)}
          >
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
        scroll={{ x: 970 }}
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
          含 <code>%%PAYLOAD_SNIPPET%%</code> 的模板才能使用项目的 U 盘文件注入。
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
