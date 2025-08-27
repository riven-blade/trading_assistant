import React from 'react';
import { 
  Card, 
  Row, 
  Col, 
  Spin,
  Statistic
} from 'antd';
import { 
  DollarOutlined,
  RiseOutlined,
  FallOutlined
} from '@ant-design/icons';
import useAccountData from '../hooks/useAccountData';

const Balances = () => {
  // 使用全局账户数据
  const { 
    accountValue, 
    loading, 
    lastUpdate,
    error,
    wsConnected
  } = useAccountData();

  // 简化显示，只保留核心财务指标

  return (
    <div style={{ padding: '24px' }}>
      {/* 核心财务指标 */}
      <Row gutter={[24, 24]} justify="center">
        <Col xs={24} sm={8}>
          <Card style={{ textAlign: 'center', height: '160px', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <Statistic
              title={<div style={{ fontSize: '16px', marginBottom: '8px' }}>账户净值</div>}
              value={accountValue.net_value || 0}
              precision={2}
              prefix={<DollarOutlined />}
              suffix="USDT"
              valueStyle={{ color: '#3f8600', fontSize: '28px' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card style={{ textAlign: 'center', height: '160px', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <Statistic
              title={<div style={{ fontSize: '16px', marginBottom: '8px' }}>可用余额</div>}
              value={accountValue.usdt_free || 0}
              precision={2}
              prefix={<DollarOutlined />}
              suffix="USDT"
              valueStyle={{ color: '#1890ff', fontSize: '28px' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card style={{ textAlign: 'center', height: '160px', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <Statistic
              title={<div style={{ fontSize: '16px', marginBottom: '8px' }}>总盈亏</div>}
              value={accountValue.total_pnl || 0}
              precision={4}
              prefix={(accountValue.total_pnl || 0) >= 0 ? <RiseOutlined /> : <FallOutlined />}
              suffix="USDT"
              valueStyle={{ 
                color: (accountValue.total_pnl || 0) >= 0 ? '#3f8600' : '#cf1322',
                fontSize: '28px'
              }}
            />
          </Card>
        </Col>
      </Row>

      {/* 状态信息 */}
      {lastUpdate && (
        <div style={{ 
          marginTop: 24,
          padding: '16px',
          backgroundColor: '#f6ffed',
          borderRadius: 8,
          border: '1px solid #b7eb8f',
          textAlign: 'center'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8 }}>
            <div style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: wsConnected ? '#52c41a' : error ? '#ff4d4f' : '#faad14',
            }} />
            <span style={{ color: '#52c41a', fontWeight: 500 }}>
              实时更新 · {lastUpdate.toLocaleTimeString()}
            </span>
            {error && <span style={{ color: '#ff4d4f', marginLeft: 16 }}>({error})</span>}
          </div>
        </div>
      )}

      {loading && (
        <div style={{ textAlign: 'center', padding: '40px 0', marginTop: 24 }}>
          <Spin size="large" />
          <div style={{ marginTop: 16, color: '#666' }}>
            正在加载数据...
          </div>
        </div>
      )}
    </div>
  );
};

export default Balances;