import React, { useState, useEffect } from 'react';
import { 
  Card, 
  Row, 
  Col, 
  Typography, 
  Spin,
  Empty,
  Statistic,
  Button,
  Tag,
  Space
} from 'antd';
import { 
  ReloadOutlined,
  DollarOutlined,
  TrophyOutlined,
  RiseOutlined,
  FallOutlined
} from '@ant-design/icons';
import api from '../services/api';

const { Text } = Typography;

const Balances = () => {
  const [balances, setBalances] = useState([]);
  const [accountValue, setAccountValue] = useState({});
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false); // 刷新状态
  const [accountStatus, setAccountStatus] = useState({});

  useEffect(() => {
    fetchBalances();
    fetchAccountStatus();
    // 每30秒刷新一次数据
    const interval = setInterval(() => {
      fetchBalances();
      fetchAccountStatus();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchBalances = async () => {
    try {
      const response = await api.get('/monitor/balances');
      if (response.data && response.data.data) {
        const data = response.data.data;
        // 新的数据结构中，余额详情在 asset_details 中
        setBalances(data.asset_details || []);
        // 保存账户价值信息
        setAccountValue({
          total_value: data.total_value || 0,
          usdt_total: data.usdt_total || 0,
          usdt_free: data.usdt_free || 0,
          other_assets_value: data.other_assets_value || 0,
          total_pnl: data.total_pnl || 0,
          net_value: data.net_value || 0
        });
      }
    } catch (error) {
      console.error('获取余额数据失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchAccountStatus = async () => {
    try {
      const response = await api.get('/monitor/account');
      setAccountStatus(response.data || {});
    } catch (error) {
      console.error('获取账户状态失败');
    }
  };

  const getAssetIcon = (asset) => {
    // 根据资产类型返回不同颜色
    const iconMap = {
      'USDT': '#26a69a',
      'BUSD': '#f4b942',
      'BTC': '#f7931a',
      'ETH': '#627eea',
      'BNB': '#f3ba2f',
    };
    return iconMap[asset] || '#666';
  };

  const getAssetCategory = (asset) => {
    const stableCoins = ['USDT', 'BUSD', 'USDC', 'DAI'];
    const majorCoins = ['BTC', 'ETH', 'BNB'];
    
    if (stableCoins.includes(asset)) return 'stable';
    if (majorCoins.includes(asset)) return 'major';
    return 'alt';
  };

  const getCategoryName = (category) => {
    const nameMap = {
      'stable': '稳定币',
      'major': '主流币',
      'alt': '其他币种'
    };
    return nameMap[category] || '未知';
  };

  const getCategoryColor = (category) => {
    const colorMap = {
      'stable': 'green',
      'major': 'blue',
      'alt': 'purple'
    };
    return colorMap[category] || 'default';
  };

  const formatBalance = (balance) => {
    if (balance >= 1000000) {
      return `${(balance / 1000000).toFixed(2)}M`;
    } else if (balance >= 1000) {
      return `${(balance / 1000).toFixed(2)}K`;
    }
    return balance.toFixed(6);
  };

  const getTotalUSDTValue = () => {
    // 使用新的总价值数据
    return accountValue.total_value || 0;
  };

  if (loading) {
    return <Spin size="large" style={{ display: 'block', textAlign: 'center', padding: '50px' }} />;
  }

  const groupedBalances = balances.reduce((acc, balance) => {
    const category = getAssetCategory(balance.asset);
    if (!acc[category]) acc[category] = [];
    acc[category].push(balance);
    return acc;
  }, {});

  return (
    <div>
      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center', 
        marginBottom: 24,
        flexWrap: 'wrap',
        gap: '16px'
      }}>
        <div className="page-title-clean">余额资产</div>
        <Button 
          icon={<ReloadOutlined />} 
          loading={refreshing}
          onClick={async () => {
            setRefreshing(true);
            try {
              await Promise.all([
                fetchBalances(),
                fetchAccountStatus()
              ]);
            } catch (error) {
              console.error('刷新失败:', error);
            } finally {
              setRefreshing(false);
            }
          }}
          type="primary"
        >
          刷新
        </Button>
      </div>

      {/* 账户概览 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} lg={8}>
          <Card>
            <Statistic
              title="净资产价值"
              value={accountValue.net_value || 0}
              precision={2}
              valueStyle={{ color: '#3f8600' }}
              prefix={<DollarOutlined />}
              suffix="USDT"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card>
            <Statistic
              title="可用USDT"
              value={accountValue.usdt_free || 0}
              precision={2}
              valueStyle={{ color: '#1890ff' }}
              prefix={<DollarOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card>
            <Statistic
              title="其他资产价值"
              value={accountValue.other_assets_value || 0}
              precision={2}
              valueStyle={{ color: '#722ed1' }}
              prefix={<DollarOutlined />}
              suffix="USDT"
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="持仓数量"
              value={accountStatus.positions || 0}
              valueStyle={{ color: '#1890ff' }}
              prefix={<TrophyOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="总盈亏"
              value={accountValue.total_pnl || 0}
              precision={2}
              valueStyle={{ 
                color: (accountValue.total_pnl || 0) >= 0 ? '#3f8600' : '#cf1322' 
              }}
              prefix={
                (accountValue.total_pnl || 0) >= 0 ? 
                <RiseOutlined /> : <FallOutlined />
              }
              suffix="USDT"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="资产种类"
              value={balances.length}
              valueStyle={{ color: '#ff7a45' }}
            />
          </Card>
        </Col>
      </Row>

      {balances.length === 0 ? (
        <Empty description="暂无余额数据" />
      ) : (
        <div>
          {Object.entries(groupedBalances).map(([category, categoryBalances]) => (
            <div key={category} style={{ marginBottom: 32 }}>
              <div style={{ marginBottom: 16 }}>
                <Tag color={getCategoryColor(category)} style={{ fontSize: '14px', padding: '4px 12px' }}>
                  {getCategoryName(category)} ({categoryBalances.length})
                </Tag>
              </div>
              
              <Row gutter={[16, 16]}>
                {categoryBalances
                  .sort((a, b) => b.amount - a.amount) // 按余额排序
                  .map((balance, index) => (
                  <Col xs={24} sm={12} md={8} lg={6} xl={4} key={index}>
                    <div className="balance-card-clean">
                      {/* 头部 */}
                      <div className="balance-header">
                        <div 
                          className="balance-asset-name"
                          style={{ color: getAssetIcon(balance.asset) }}
                        >
                          {balance.asset}
                        </div>
                        <div className="balance-update-time">
                          {new Date(balance.updated_at).toLocaleTimeString()}
                        </div>
                      </div>

                      {/* 金额显示 */}
                      <div className="balance-amount-section">
                        <div 
                          className="balance-amount"
                          style={{ color: getAssetIcon(balance.asset) }}
                        >
                          {formatBalance(balance.amount)}
                        </div>
                        
                        {balance.value_usdt > 0 && (
                          <div className="balance-usdt-value">
                            ≈ ${balance.value_usdt.toFixed(2)}
                          </div>
                        )}
                      </div>

                      {/* 比例显示 */}
                      {balance.amount > 1000 && (
                        <div className="balance-percentage">
                          <div className="percentage-bar">
                            <div 
                              className="percentage-fill"
                              style={{ 
                                width: `${Math.min(100, (balance.value_usdt / getTotalUSDTValue()) * 100)}%`,
                                backgroundColor: getAssetIcon(balance.asset)
                              }}
                            />
                          </div>
                          <span className="percentage-text">
                            {((balance.value_usdt / getTotalUSDTValue()) * 100).toFixed(0)}%
                          </span>
                        </div>
                      )}
                    </div>
                  </Col>
                ))}
              </Row>
            </div>
          ))}
        </div>
      )}

      {/* 账户状态信息 */}
      <Card 
        title="账户状态" 
        style={{ marginTop: 24 }}
        size="small"
      >
        <Row gutter={16}>
          <Col xs={24} sm={8}>
            <Text strong>WebSocket状态: </Text>
            <Tag color={accountStatus.running ? 'green' : 'red'}>
              {accountStatus.running ? '运行中' : '已停止'}
            </Tag>
          </Col>
          <Col xs={24} sm={8}>
            <Text strong>监控模式: </Text>
            <Tag color="blue">{accountStatus.mode || 'unknown'}</Tag>
          </Col>
          <Col xs={24} sm={8}>
            <Text strong>连接状态: </Text>
            <Space>
              <Tag color={accountStatus.redis_connected ? 'green' : 'red'}>
                Redis
              </Tag>
              <Tag color={accountStatus.binance_connected ? 'green' : 'red'}>
                Binance
              </Tag>
            </Space>
          </Col>
        </Row>
      </Card>
    </div>
  );
};

export default Balances;
