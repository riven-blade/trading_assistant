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

const { Text } = Typography;

const Orders = () => {
  const [markPrices, setMarkPrices] = useState({});
  const [pricesLoading, setPricesLoading] = useState(false);
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


  useEffect(() => {
    // 初始化时获取所有数据
    const initializeData = async () => {
      if (!isMountedRef.current) return;
      
      // 获取价格预估和币种列表
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
        pricesMap[symbol] = priceData?.markPrice || 0;
      });
      
      setMarkPrices(pricesMap);
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


  // 表格列定义
  const estimateColumns = [
    {
      title: '交易对',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 100,
      render: (symbol) => <Text strong>{symbol && symbol.length > 8 ? symbol.substring(0, 8) + '...' : symbol}</Text>
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
      dataIndex: 'action_type',
      key: 'action_type',
      width: 80,
      render: (actionType) => {
        const typeMap = {
          'open': '开仓',
          'addition': '加仓',
          'close': '平仓',
          'take_profit': '止盈', 
          'stop_loss': '止损'
        };
        const colorMap = {
          'open': 'green',
          'addition': 'green',
          'close': 'purple',
          'take_profit': 'blue',
          'stop_loss': 'red'
        };
        return <Tag color={colorMap[actionType]}>{typeMap[actionType] || actionType}</Tag>;
      }
    },
    {
      title: '目标价格',
      dataIndex: 'target_price',
      key: 'target_price',
      width: 100,
      render: (price) => <Text strong>${(price || 0).toFixed(4)}</Text>
    },
    {
      title: '当前价格',
      dataIndex: 'symbol',
      key: 'current_price',
      width: 120,
      render: (symbol, record) => {
        const markPrice = markPrices[symbol] || 0;
        const targetPrice = record.target_price || 0;
        
        // 计算百分比差异
        let percentage = 0;
        if (markPrice > 0 && targetPrice > 0) {
          percentage = ((markPrice - targetPrice) / targetPrice * 100);
        }
        
        return (
          <div>
            <Text strong style={{ color: '#1890ff', display: 'block' }}>
              ${(markPrice || 0).toFixed(4)}
            </Text>
            {percentage !== 0 && (
              <Text 
                style={{ 
                  fontSize: '10px',
                  color: percentage >= 0 ? '#059669' : '#dc2626',
                  fontWeight: 500
                }}
              >
                {percentage >= 0 ? '+' : ''}{(percentage || 0).toFixed(2)}%
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
        return <Text>{(quantity || 0).toFixed(6)} {baseAsset}</Text>
      }
    },
    {
      title: '预估金额',
      dataIndex: 'target_price',
      key: 'estimated_amount',
      width: 120,
      render: (targetPrice, record) => {
        const estimatedAmount = (record.quantity || 0) * (targetPrice || 0);
        return <Text>${(estimatedAmount || 0).toFixed(2)} USDT</Text>
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


  // 计算统计信息
  const getEstimatesStats = () => {
    const total = estimates.length;
    const listening = estimates.filter(e => e.status === 'listening').length;
    const triggered = estimates.filter(e => e.status === 'triggered').length;
    const failed = estimates.filter(e => e.status === 'failed').length;
    
    return { total, listening, triggered, failed };
  };

  const estimatesStats = getEstimatesStats();

  // 页面操作配置
  const headerActions = [
    {
      icon: <ReloadOutlined />,
      loading: estimatesLoading || pricesLoading,
      onClick: () => fetchAllEstimates(),
      children: '刷新'
    }
  ];

  const headerExtra = (estimatesLoading || pricesLoading) && (
    <Text type="secondary" style={{ fontSize: '12px' }}>
      {estimatesLoading ? '正在加载数据...' : '正在更新价格...'}
    </Text>
  );

  return (
    <div>
      <PageHeader 
        title="价格预估" 
        actions={headerActions}
        extra={headerExtra}
      />

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
                listHeight={200}
                virtual={false}
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
    </div>
  );
};

export default Orders;
