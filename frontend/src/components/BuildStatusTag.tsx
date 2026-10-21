import { LoadingOutlined } from '@ant-design/icons';
import { Tag } from 'antd';
import React from 'react';
import { BUILD_STATUS, isBuildActive } from '@/constants';
import type { BuildStatus } from '@/services/types';

interface Props {
  status: BuildStatus;
  /** 排队/构建中时是否显示转圈 */
  animated?: boolean;
}

/**
 * 构建状态标签。全部页面共用，避免同一状态在不同页面显示成不同颜色。
 */
const BuildStatusTag: React.FC<Props> = ({ status, animated = true }) => {
  const record = BUILD_STATUS[status] ?? { label: status, color: 'default' };
  const showSpinner = animated && isBuildActive(status);

  return (
    <Tag
      color={record.color}
      icon={showSpinner ? <LoadingOutlined /> : undefined}
      style={{ marginInlineEnd: 0 }}
    >
      {record.label}
    </Tag>
  );
};

export default BuildStatusTag;
