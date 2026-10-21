import { LockOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { history, useModel } from '@umijs/max';
import { message } from 'antd';
import React from 'react';
import { setToken } from '@/services/auth';
import { fetchConfig } from '@/services/isobuilder';

/**
 * 登录页。
 *
 * 后端没有用户体系，只有一个共享口令（ISOBUILDER_AUTH_TOKEN）。
 * 所以这里只有一个口令输入框：提交时用 GET /api/config 带上口令探活，
 * 通过即把口令存本地，之后由请求拦截器附加到 Authorization 头。
 */
const Login: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');

  const handleSubmit = async (values: { token: string }) => {
    const token = values.token.trim();
    if (!token) return false;

    setToken(token);

    try {
      await fetchConfig();
    } catch {
      // 口令错误时后端返回 401，请求层已清除 token 并提示。
      message.error('口令校验失败，请确认后重试');
      return false;
    }

    // 探活通过后，需要再打一次带认证的接口才能确认口令真的有效：
    // /api/config 本身免认证，所以这里用 /api/projects 验证。
    const { listProjects } = await import('@/services/isobuilder');
    try {
      await listProjects();
    } catch {
      message.error('口令无效');
      return false;
    }

    await setInitialState((s) => ({ ...s, currentUser: { name: '运维' } }) as NonNullable<typeof s>);

    const params = new URL(window.location.href).searchParams;
    history.push(params.get('redirect') || '/projects');
    return true;
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100vh',
        overflow: 'auto',
        background: '#f5f5f5',
      }}
    >
      <div style={{ flex: 1, padding: '32px 0' }}>
        <LoginForm
          logo={<img alt="logo" src="/logo.svg" />}
          title="ISO Builder"
          subTitle="自动装机镜像构建平台"
          initialValues={{ autoLogin: true }}
          onFinish={async (values) => handleSubmit(values as { token: string })}
        >
          <ProFormText
            name="token"
            fieldProps={{
              size: 'large',
              prefix: <LockOutlined />,
            }}
            placeholder="访问口令"
            rules={[{ required: true, message: '请输入访问口令' }]}
          />
          {initialState?.authRequired === false && (
            <div style={{ color: '#999', fontSize: 12, marginTop: -8 }}>
              当前后端未启用口令校验，可直接进入。
            </div>
          )}
        </LoginForm>
      </div>
    </div>
  );
};

export default Login;
