import React, { useState, useEffect, useCallback, useRef } from 'react';
import { 
  Card, 
  Row, 
  Col, 
  Table, 
  Typography, 
  Tag, 
  message,
  Empty,
  Popconfirm,
  Tabs,
  Badge,
  Statistic,
  Space,
  Select
} from 'antd';
import { 
  ReloadOutlined,
  LineChartOutlined,
  BarChartOutlined
} from '@ant-design/icons';
import api, { getAllEstimates } from '../services/api';

// 通用组件
import PageHeader from '../components/Common/PageHeader';

import usePriceData from '../hooks/usePriceData';
import { ORDER_STATUS_MAP } from '../utils/constants';

const { Text } = Typography;

const Orders = () => {
  const [orders, setOrders] = useState([]);
  const [currentPrices, setCurrentPrices] = useState({});
  const [pricesLoading, setPricesLoading] = useState(false);
  const [ordersLoading, setOrdersLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('estimates');
  const [orderSymbol, setOrderSymbol] = useState('');
  const [estimateSymbol, setEstimateSymbol] = useState('');
  const [selectedCoins, setSelectedCoins] = useState([]);
  
  // 价格预估相关状态
  const [estimates, setEstimates] = useState([]);
  const [estimatesLoading, setEstimatesLoading] = useState(true);
  const isMountedRef = useRef(true); // 组件挂载状态

  // 使用全局价格数据
  const { 
    getPriceBySymbol
  } = usePriceData();

  // 获取所有价格预估数据
  const fetchAllEstimates = useCallback(async (symbol = estimateSymbol) => {
    try {
      if (!isMountedRef.current) return;
      setEstimatesLoading(true);
      
      const estimatesData = await getAllEstimates(symbol || null);
      
      if (!isMountedRef.current) return;
      
      // 按时间倒序排列
      const sortedEstimates = estimatesData.sort((a, b) => {
        return new Date(b.created_at) - new Date(a.created_at);
      });
      
      setEstimates(sortedEstimates);
      console.log(`[订单页面] 获取到 ${estimatesData.length} 条价格预估数据`);
    } catch (error) {
      console.error('获取价格预估数据失败:', error);
      if (isMountedRef.current) {
        message.error('获取价格预估数据失败');
      }
    } finally {
      if (isMountedRef.current) {
        setEstimatesLoading(false);
      }
    }
  }, [estimateSymbol]);

  // 获取已选中的币种列表
  const fetchSelectedCoins = useCallback(async () => {
    try {
      if (!isMountedRef.current) return;
      
      const response = await api.get('/coins/selected');
      
      if (!isMountedRef.current) return;
      
      setSelectedCoins(response.data.data || []);
    } catch (error) {
      console.error('获取选中币种失败:', error);
    }
  }, []);

  // 获取订单状态
  const fetchOrders = useCallback(async (symbol = orderSymbol) => {
    try {
      if (!isMountedRef.current) return;
      setOrdersLoading(true);
      
      const url = symbol ? `/monitor/orders?symbol=${symbol}` : '/monitor/orders';
      const response = await api.get(url);
      
      if (!isMountedRef.current) return;
      
      const orderData = response.data.data || [];
      const sortedOrders = orderData.sort((a, b) => {
        return new Date(b.created_at) - new Date(a.created_at);
      });
      
      setOrders(sortedOrders);
    } catch (error) {
      if (isMountedRef.current) {
        message.error('获取订单状态失败');
      }
    } finally {
      if (isMountedRef.current) {
        setOrdersLoading(false);
      }
    }
  }, [orderSymbol]);

  useEffect(() => {
    // 初始化时获取所有数据
    const initializeData = async () => {
      if (!isMountedRef.current) return;
      
      // 获取订单状态、价格预估和币种列表
      await fetchOrders();
      await fetchAllEstimates();
      await fetchSelectedCoins();
    };
    
    initializeData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // 只在组件挂载时运行一次，避免无限循环
  
  // 组件卸载时的清理
  useEffect(() => {
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  // 获取当前价格 - 使用全局价格数据
  const fetchCurrentPrices = useCallback((estimatesData) => {
    if (!isMountedRef.current) return;
    
    try {
      setPricesLoading(true);
      const symbols = [...new Set(estimatesData.map(estimate => estimate.symbol))];
      
      const pricesMap = {};
      symbols.forEach(symbol => {
        const priceData = getPriceBySymbol(symbol);
        pricesMap[symbol] = priceData?.markPrice || priceData?.currentPrice || 0;
      });
      
      setCurrentPrices(pricesMap);
    } catch (error) {
      console.error('获取价格数据失败:', error);
    } finally {
      if (isMountedRef.current) {
        setPricesLoading(false);
      }
    }
  }, [getPriceBySymbol]);

  // 当estimates数据变化时，获取当前价格
  useEffect(() => {
    if (estimates.length > 0) {
      fetchCurrentPrices(estimates);
    }
  }, [estimates, fetchCurrentPrices]);

  // 删除单个价格预估
  const handleDeleteEstimate = async (estimateId) => {
    try {
      setEstimatesLoading(true);
      // 这里假设有一个单个删除的API，我们先用现有的方式
      await api.delete(`/estimates/${estimateId}`);
      message.success('删除价格预估成功');
      
      // 刷新数据
      await fetchAllEstimates();
    } catch (error) {
      console.error('删除价格预估失败:', error);
      message.error('删除价格预估失败');
    } finally {
      setEstimatesLoading(false);
    }
  };

  // 取消订单
  const handleCancelOrder = async (orderId, exchangeId, symbol) => {
    try {
      setOrdersLoading(true);
      
      const response = await api.post('/monitor/orders/cancel', {
        order_id: orderId,
        exchange_id: exchangeId,
        symbol: symbol
      });
      
      if (response.data.success) {
        message.success('订单取消成功');
        // 刷新订单列表
        await fetchOrders(orderSymbol);
      } else {
        message.error(`取消订单失败: ${response.data.message || '未知错误'}`);
      }
    } catch (error) {
      console.error('取消订单失败:', error);
      message.error('取消订单失败');
    } finally {
      setOrdersLoading(false);
    }
  };

  // 表格列定义
  const estimateColumns = [
    {
      title: '交易对',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 100,
      render: (symbol) => <Text strong>{symbol}</Text>
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 60,
      render: (side) => <Tag color={side === 'long' ? 'green' : 'red'}>{side === 'long' ? '多头' : '空头'}</Tag>
    },
    {
      title: '操作类型',
      dataIndex: 'created_by',
      key: 'created_by',
      width: 80,
      render: (createdBy) => {
        const typeMap = {
          'add_position': '加仓',
          'take_profit': '止盈', 
          'stop_loss': '止损',
          'open_position': '开仓'
        };
        const colorMap = {
          'add_position': 'green',
          'take_profit': 'blue',
          'stop_loss': 'red',
          'open_position': 'orange'
        };
        return <Tag color={colorMap[createdBy]}>{typeMap[createdBy] || createdBy}</Tag>;
      }
    },
    {
      title: '目标价格',
      dataIndex: 'target_price',
      key: 'target_price',
      width: 100,
      render: (price) => <Text strong>${price.toFixed(4)}</Text>
    },
    {
      title: '当前价格',
      dataIndex: 'symbol',
      key: 'current_price',
      width: 120,
      render: (symbol, record) => {
        const currentPrice = currentPrices[symbol] || 0;
        const targetPrice = record.target_price || 0;
        
        // 计算百分比差异
        let percentage = 0;
        if (currentPrice > 0 && targetPrice > 0) {
          percentage = ((currentPrice - targetPrice) / targetPrice * 100);
        }
        
        return (
          <div>
            <Text strong style={{ color: '#1890ff', display: 'block' }}>
              ${currentPrice.toFixed(4)}
            </Text>
            {percentage !== 0 && (
              <Text 
                style={{ 
                  fontSize: '10px',
                  color: percentage >= 0 ? '#059669' : '#dc2626',
                  fontWeight: 500
                }}
              >
                {percentage >= 0 ? '+' : ''}{percentage.toFixed(2)}%
              </Text>
            )}
          </div>
        );
      }
    },
    {
      title: '交易数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 120,
      render: (quantity, record) => {
        const baseAsset = record.symbol?.replace('USDT', '') || '';
        return <Text>{quantity?.toFixed(6)} {baseAsset}</Text>
      }
    },
    {
      title: '预估金额',
      dataIndex: 'target_price',
      key: 'estimated_amount',
      width: 120,
      render: (targetPrice, record) => {
        const estimatedAmount = (record.quantity || 0) * (targetPrice || 0);
        return <Text>${estimatedAmount.toFixed(2)} USDT</Text>
      }
    },
    {
      title: '杠杆',
      dataIndex: 'leverage',
      key: 'leverage',
      width: 60,
      render: (leverage) => <Tag color="blue">{leverage}x</Tag>
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status) => {
        const statusMap = {
          'listening': { color: 'orange', text: '监听中' },
          'triggered': { color: 'blue', text: '已触发' },
          'failed': { color: 'red', text: '触发失败' }
        };
        const statusInfo = statusMap[status] || { color: 'default', text: status };
        return <Tag color={statusInfo.color}>{statusInfo.text}</Tag>;
      }
    },

    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (time) => new Date(time).toLocaleString()
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <Popconfirm
          title="确认删除此价格预估？"
          description="删除后监听将无法恢复"
          onConfirm={() => handleDeleteEstimate(record.id)}
          okText="确认删除"
          cancelText="取消"
          okType="danger"
        >
          <button 
            className="control-btn danger-btn orders-table-btn"
            disabled={estimatesLoading}
          >
            删除
          </button>
        </Popconfirm>
      )
    }

  ];

  const orderColumns = [
    {
      title: '订单ID',
      dataIndex: 'id',
      key: 'id',
      width: 120,
      render: (id) => <Text style={{ fontSize: '12px', fontFamily: 'monospace' }}>{String(id)}</Text>
    },
    {
      title: '交易对',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 100,
      render: (symbol) => <Text strong>{symbol}</Text>
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 60,
      render: (side) => <Tag color={side === 'BUY' ? 'green' : 'red'}>{side}</Tag>
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      render: (type) => <Tag color="blue">{type}</Tag>
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 120,
      render: (quantity) => <Text>{quantity.toFixed(6)}</Text>
    },
    {
      title: '已成交',
      dataIndex: 'executed_qty',
      key: 'executed_qty',
      width: 120,
      render: (executedQty) => <Text>{executedQty.toFixed(6)}</Text>
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 100,
      render: (price) => <Text strong>${price.toFixed(4)}</Text>
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => {
        const statusInfo = ORDER_STATUS_MAP[status] || { color: 'default', text: status };
        return <Tag color={statusInfo.color}>{statusInfo.text}</Tag>;
      }
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 120,
      render: (time) => new Date(time).toLocaleString()
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => {
        // 只有NEW状态的订单可以取消
        if (record.status === 'NEW') {
          return (
            <Popconfirm
              title="确认取消此订单？"
              description="取消后订单将无法恢复"
              onConfirm={() => handleCancelOrder(record.id, record.exchange_id, record.symbol)}
              okText="确认取消"
              cancelText="取消"
              okType="danger"
            >
              <button 
                className="control-btn danger-btn orders-table-btn"
                disabled={ordersLoading}
              >
                取消
              </button>
            </Popconfirm>
          );
        }
        return <Text type="secondary">-</Text>;
      },
    }
  ];

  // 计算统计信息
  const getEstimatesStats = () => {
    const total = estimates.length;
    const listening = estimates.filter(e => e.status === 'listening').length;
    const triggered = estimates.filter(e => e.status === 'triggered').length;
    const failed = estimates.filter(e => e.status === 'failed').length;
    
    return { total, listening, triggered, failed };
  };

  const getOrdersStats = () => {
    const total = orders.length;
    const newOrders = orders.filter(o => o.status === 'NEW').length;
    const filled = orders.filter(o => o.status === 'FILLED').length;
    const cancelled = orders.filter(o => o.status === 'CANCELED').length;
    
    return { total, newOrders, filled, cancelled };
  };

  const estimatesStats = getEstimatesStats();
  const ordersStats = getOrdersStats();

  // 页面操作配置
  const headerActions = [
    {
      icon: <ReloadOutlined />,
      loading: activeTab === 'estimates' ? estimatesLoading || pricesLoading : ordersLoading,
      onClick: () => {
        if (activeTab === 'estimates') {
          fetchAllEstimates();
        } else {
          fetchOrders();
        }
      },
      children: '刷新'
    }
  ];

  const headerExtra = activeTab === 'estimates' && (estimatesLoading || pricesLoading) && (
    <Text type="secondary" style={{ fontSize: '12px' }}>
      {estimatesLoading ? '正在加载数据...' : '正在更新价格...'}
    </Text>
  );

  return (
    <div>
      <PageHeader 
        title="订单管理" 
        actions={headerActions}
        extra={headerExtra}
      />

      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <Tabs.TabPane 
          tab={
            <Badge count={estimatesStats.listening} offset={[10, 0]}>
              <span>价格预估</span>
            </Badge>
          } 
          key="estimates"
        >
          {/* 统计信息 */}
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="总计"
                  value={estimatesStats.total}
                  prefix={<LineChartOutlined style={{ color: '#1890ff' }} />}
                />
              </Card>
            </Col>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="监听中"
                  value={estimatesStats.listening}
                  valueStyle={{ color: '#fa8c16' }}
                  prefix={<BarChartOutlined />}
                />
              </Card>
            </Col>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="已触发"
                  value={estimatesStats.triggered}
                  valueStyle={{ color: '#1890ff' }}
                  prefix={<BarChartOutlined />}
                />
              </Card>
            </Col>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="触发失败"
                  value={estimatesStats.failed}
                  valueStyle={{ color: '#ff4d4f' }}
                  prefix={<BarChartOutlined />}
                />
              </Card>
            </Col>
          </Row>

          {/* 筛选器 */}
          <Card style={{ marginBottom: 16 }}>
            <Row gutter={16} align="middle">
              <Col flex="auto">
                <Space size="middle" style={{ width: '100%' }}>
                  <Text strong>交易对筛选:</Text>
                  <Select
                    placeholder="选择交易对"
                    value={estimateSymbol}
                    onChange={(val) => {
                      setEstimateSymbol(val || '');
                      fetchAllEstimates(val || '');
                    }}
                    style={{ minWidth: 200 }}
                    allowClear
                  >
                    <Select.Option value="">
                      <Space>
                        <span>📊</span>
                        <span>所有价格监听</span>
                      </Space>
                    </Select.Option>
                    {selectedCoins.map(option => (
                      <Select.Option key={option.symbol} value={option.symbol}>
                        <Space>
                          <span>📈</span>
                          <Text strong style={{ color: '#1890ff' }}>
                            {option.symbol}
                          </Text>
                        </Space>
                      </Select.Option>
                    ))}
                  </Select>
                  <button 
                    className="control-btn primary-btn orders-action-btn"
                    onClick={() => fetchAllEstimates()}
                    disabled={estimatesLoading || pricesLoading}
                  >
                    <ReloadOutlined style={{ marginRight: 4 }} />
                    刷新
                  </button>
                </Space>
              </Col>
            </Row>
          </Card>

          <Card>
            <Table
              columns={estimateColumns}
              dataSource={estimateSymbol ? estimates.filter(e => e.symbol === estimateSymbol) : estimates}
              rowKey="id"
              loading={estimatesLoading}
              size="small"
              scroll={{ x: 1000 }}
              pagination={{
                showSizeChanger: true,
                showQuickJumper: true,
                showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条，共 ${total} 条记录`,
              }}
              locale={{
                emptyText: estimateSymbol ? (
                  <Empty 
                    description={`暂无 ${estimateSymbol} 的价格预估`}
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                  />
                ) : (
                  <Empty description="暂无价格预估" />
                )
              }}
            />
          </Card>
        </Tabs.TabPane>

        <Tabs.TabPane 
          tab={
            <Badge count={ordersStats.newOrders} offset={[10, 0]}>
              <span>订单状态</span>
            </Badge>
          } 
          key="orders"
        >
          {/* 统计信息 */}
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="总计"
                  value={ordersStats.total}
                  prefix={<LineChartOutlined style={{ color: '#1890ff' }} />}
                />
              </Card>
            </Col>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="新订单"
                  value={ordersStats.newOrders}
                  valueStyle={{ color: '#fa8c16' }}
                  prefix={<BarChartOutlined />}
                />
              </Card>
            </Col>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="已成交"
                  value={ordersStats.filled}
                  valueStyle={{ color: '#52c41a' }}
                  prefix={<BarChartOutlined />}
                />
              </Card>
            </Col>
            <Col xs={6} sm={6} md={6} lg={6}>
              <Card size="small">
                <Statistic
                  title="已取消"
                  value={ordersStats.cancelled}
                  valueStyle={{ color: '#ff4d4f' }}
                  prefix={<BarChartOutlined />}
                />
              </Card>
            </Col>
          </Row>

          {/* 筛选器 */}
          <Card style={{ marginBottom: 16 }}>
            <Row gutter={16} align="middle">
              <Col flex="auto">
                <Space size="middle" style={{ width: '100%' }}>
                  <Text strong>交易对筛选:</Text>
                  <Select
                    placeholder="选择交易对"
                    value={orderSymbol}
                    onChange={(val) => {
                      setOrderSymbol(val || '');
                      fetchOrders(val || '');
                    }}
                    style={{ minWidth: 200 }}
                    allowClear
                  >
                    <Select.Option value="">
                      <Space>
                        <span>🔄</span>
                        <span>所有活跃订单</span>
                      </Space>
                    </Select.Option>
                    {selectedCoins.map(option => (
                      <Select.Option key={option.symbol} value={option.symbol}>
                        <Space>
                          <span>📈</span>
                          <Text strong style={{ color: '#1890ff' }}>
                            {option.symbol}
                          </Text>
                        </Space>
                      </Select.Option>
                    ))}
                  </Select>
                  <button 
                    className="control-btn primary-btn orders-action-btn"
                    onClick={() => fetchOrders(orderSymbol)}
                    disabled={ordersLoading}
                  >
                    <ReloadOutlined style={{ marginRight: 4 }} />
                    刷新
                  </button>
                </Space>
              </Col>
            </Row>
          </Card>

          <Card>
            <Table
              columns={orderColumns}
              dataSource={orders}
              rowKey="id"
              loading={ordersLoading}
              size="small"
              scroll={{ x: 1000 }}
              pagination={{
                showSizeChanger: true,
                showQuickJumper: true,
                showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条，共 ${total} 条记录`,
              }}
              locale={{
                emptyText: orderSymbol ? (
                  <Empty 
                    description={`暂无 ${orderSymbol} 的历史订单`}
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                  />
                ) : (
                  <Empty description="暂无历史订单" />
                )
              }}
            />
          </Card>
        </Tabs.TabPane>
      </Tabs>
    </div>
  );
};

export default Orders;
