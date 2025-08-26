import React, { useState, useEffect } from 'react';
import { 
  Card, 
  Row, 
  Col, 
  Spin,
  Empty,
  Statistic,
  Tag
} from 'antd';
import { 
  DollarOutlined,
  TrophyOutlined,
  RiseOutlined,
  FallOutlined
} from '@ant-design/icons';
import useAccountData from '../hooks/useAccountData';
import api from '../services/api';

const Balances = () => {
  const [balances, setBalances] = useState([]);
  
  // 使用全局账户数据
  const { 
    accountValue, 
    loading, 
    lastUpdate,
    error 
  } = useAccountData();

  // 获取资产详情数据
  useEffect(() => {
    const fetchBalances = async () => {
      try {
        const response = await api.get('/monitor/balances');
        if (response.data && response.data.data) {
          const data = response.data.data;
          // 新的数据结构中，余额详情在 asset_details 中
          setBalances(data.asset_details || []);
        }
      } catch (error) {
        console.error('获取余额详情失败');
        setBalances([]);
      }
    };

    fetchBalances();
    // 每30秒刷新一次资产详情
    const interval = setInterval(fetchBalances, 30000);
    return () => clearInterval(interval);
  }, []);



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
        {lastUpdate && (
          <div style={{ 
            fontSize: '12px', 
            color: '#666', 
            display: 'flex', 
            alignItems: 'center', 
            gap: '8px' 
          }}>
            <div style={{ 
              width: '8px', 
              height: '8px', 
              borderRadius: '50%', 
              backgroundColor: error ? '#ff4d4f' : '#52c41a'
            }} />
            <span>实时更新 · {lastUpdate.toLocaleTimeString()}</span>
          </div>
        )}
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
              value={accountValue.positions || 0}
              valueStyle={{ color: '#1890ff' }}
              prefix={<TrophyOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="持仓盈亏明细"
              value={accountValue.total_pnl || 0}
              precision={4}
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
              value={accountValue.asset_count || balances.length}
              valueStyle={{ color: '#ff7a45' }}
            />
          </Card>
        </Col>
      </Row>

      {/* 保证金和锁定资产信息 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="已锁定USDT"
              value={accountValue.usdt_locked || 0}
              precision={2}
              valueStyle={{ color: '#fa8c16' }}
              prefix={<DollarOutlined />}
              suffix="USDT"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="已用保证金"
              value={accountValue.margin_used || 0}
              precision={2}
              valueStyle={{ color: '#eb2f96' }}
              prefix={<DollarOutlined />}
              suffix="USDT"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={8}>
          <Card>
            <Statistic
              title="保证金使用率"
              value={accountValue.margin_ratio || 0}
              precision={1}
              valueStyle={{ 
                color: (accountValue.margin_ratio || 0) > 80 ? '#cf1322' : 
                       (accountValue.margin_ratio || 0) > 60 ? '#fa8c16' : '#3f8600' 
              }}
              suffix="%"
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
                        
                        {/* 显示可用/锁定余额 */}
                        {(balance.free || balance.locked) && (
                          <div className="balance-breakdown">
                            <div className="balance-free">
                              可用: {formatBalance(balance.free || 0)}
                            </div>
                            {balance.locked > 0 && (
                              <div className="balance-locked">
                                锁定: {formatBalance(balance.locked)}
                              </div>
                            )}
                          </div>
                        )}
                        
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


    </div>
  );
};

export default Balances;
