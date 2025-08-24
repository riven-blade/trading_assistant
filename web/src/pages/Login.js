import React, { useState } from 'react';
import { Form, Input, Button, Card, Layout, Typography } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';

const { Content } = Layout;
const { Text } = Typography;

const Login = ({ onLogin }) => {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (values) => {
    setLoading(true);
    await onLogin(values.username, values.password);
    setLoading(false);
  };

  return (
    <Layout style={{ minHeight: '100vh', background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' }}>
      <Content style={{ display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
        <Card
          style={{
            width: 400,
            padding: '20px',
            borderRadius: '12px',
            boxShadow: '0 8px 32px rgba(0, 0, 0, 0.1)',
          }}
        >
          <div style={{ textAlign: 'center', marginBottom: '32px' }}>
            <div className="login-title-clean">
              交易助手
            </div>
            <Text type="secondary">
              智能化的加密货币交易管理平台
            </Text>
          </div>
          
          <Form
            name="login"
            onFinish={handleSubmit}
            autoComplete="off"
            size="large"
          >
            <Form.Item
              name="username"
              rules={[{ required: true, message: '请输入用户名' }]}
            >
              <Input
                prefix={<UserOutlined />}
                placeholder="用户名"
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: '请输入密码' }]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="密码"
              />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                style={{ width: '100%', height: '44px' }}
              >
                登录
              </Button>
            </Form.Item>
          </Form>

          <div style={{ textAlign: 'center', marginTop: '16px' }}>
            <Text type="secondary" style={{ fontSize: '12px' }}>
              请联系管理员获取登录凭据
            </Text>
          </div>
        </Card>
      </Content>
    </Layout>
  );
};

export default Login;
