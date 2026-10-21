import { LogoutOutlined } from '@ant-design/icons';
import { history, useModel } from '@umijs/max';
import type { MenuProps } from 'antd';
import { Button, Dropdown } from 'antd';
import React from 'react';
import { clearToken } from '@/services/auth';

/**
 * 右上角账户菜单。系统只有单一共享口令，没有多用户，所以菜单只保留
 * 「退出登录」一项；点击后清除本地 token 并回登录页。
 */
export const AvatarDropdown: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');

  const onMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'logout') {
      clearToken();
      setInitialState((s) => ({ ...s, currentUser: undefined }));
      history.push('/login');
    }
  };

  const items: MenuProps['items'] = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
    },
  ];

  return (
    <Dropdown menu={{ items, onClick: onMenuClick }} placement="bottomRight">
      <Button type="text" style={{ paddingInline: 8 }}>
        {initialState?.currentUser?.name ?? '运维'}
      </Button>
    </Dropdown>
  );
};

export default AvatarDropdown;
