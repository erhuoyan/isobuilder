import { PlusOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Popconfirm, Space, message } from 'antd';
import React, { useEffect, useState } from 'react';
import {
  createScript,
  deleteScript,
  listScripts,
  updateScript,
} from '@/services/isobuilder';
import type { Script } from '@/services/types';
import { formatBytes, formatDateTime } from '@/utils/format';

/** 后端限制脚本正文 64 KiB，超出返回 413 too_large。 */
const MAX_SCRIPT_BYTES = 64 * 1024;

interface ScriptFormValues {
  name: string;
  remark?: string;
  content: string;
}

const ScriptsPage: React.FC = () => {
  const [scripts, setScripts] = useState<Script[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Script | null>(null);

  const load = async () => {
    const res = await listScripts();
    setScripts(res.scripts ?? []);
  };

  useEffect(() => {
    void load();
  }, []);

  const handleSubmit = async (values: ScriptFormValues): Promise<boolean> => {
    const content = values.content ?? '';

    if (new TextEncoder().encode(content).length > MAX_SCRIPT_BYTES) {
      message.error(`脚本不能超过 ${formatBytes(MAX_SCRIPT_BYTES)}`);
      return false;
    }

    try {
      if (editing) {
        await updateScript(editing.id, {
          name: values.name,
          remark: values.remark ?? '',
          content,
        });
        message.success('脚本已保存');
      } else {
        await createScript({ name: values.name, remark: values.remark, content });
        message.success('脚本已创建');
      }

      void load();
      return true;
    } catch {
      return false;
    }
  };

  const handleDelete = async (record: Script) => {
    try {
      await deleteScript(record.id);
      message.success('已删除');
      void load();
    } catch {
      // 请求层已提示；被项目引用时返回 409 script_in_use
    }
  };

  const columns: ProColumns<Script>[] = [
    {
      title: '名称',
      dataIndex: 'name',
      width: 220,
      render: (_, record) => <a onClick={() => { setEditing(record); setOpen(true); }}>{record.name}</a>,
    },
    {
      title: '备注',
      dataIndex: 'remark',
      width: 260,
      render: (_, r) => r.remark || <span style={{ color: 'rgba(0,0,0,0.35)' }}>—</span>,
    },
    {
      title: '行数',
      width: 90,
      render: (_, r) => <code>{(r.content ?? '').split('\n').length}</code>,
    },
    { title: '大小', dataIndex: 'size_bytes', width: 100, render: (_, r) => formatBytes(r.size_bytes) },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 170,
      render: (_, r) => formatDateTime(r.updated_at),
    },
    {
      title: '操作',
      width: 140,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Button
            type="link"
            size="small"
            onClick={() => {
              setEditing(record);
              setOpen(true);
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title={`删除脚本「${record.name}」？`}
            okText="删除"
            cancelText="取消"
            onConfirm={() => handleDelete(record)}
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
        title: '自定义脚本',
        subTitle: '可复用的安装后脚本库。项目引用后可再追加一段项目专属脚本',
        extra: [
          <Button
            key="create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null);
              setOpen(true);
            }}
          >
            新建脚本
          </Button>,
        ],
      }}
    >
      <ProTable<Script>
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        dataSource={scripts}
        columns={columns}
        scroll={{ x: 1000 }}
        locale={{ emptyText: '还没有脚本，先新建一个' }}
      />

      <ModalForm<ScriptFormValues>
        title={editing ? `编辑脚本：${editing.name}` : '新建脚本'}
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          if (!next) setEditing(null);
        }}
        onFinish={handleSubmit}
        modalProps={{ destroyOnClose: true, width: 760 }}
        initialValues={
          editing
            ? { name: editing.name, remark: editing.remark, content: editing.content }
            : { name: '', remark: '', content: '#!/bin/bash\nset -euo pipefail\n\n' }
        }
      >
        <ProFormText name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]} />
        <ProFormText name="remark" label="备注" placeholder="例如：所有机器基线" />
        <ProFormTextArea
          name="content"
          label="脚本内容"
          fieldProps={{ rows: 16, style: { fontFamily: 'monospace', fontSize: 12 } }}
          rules={[{ required: true, message: '请输入脚本内容' }]}
          extra="构建时以 base64 编码写入 ISO 的 %post 段，在 chroot 内执行；失败不影响安装结果"
        />
      </ModalForm>
    </PageContainer>
  );
};

export default ScriptsPage;
