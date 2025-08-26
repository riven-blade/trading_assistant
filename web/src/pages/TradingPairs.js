import React, { useState, useEffect } from 'react';
import { 
  Drawer, 
  Input, 
  Typography, 
  message,
  Spin,
  Empty,
  Badge,
  Tag,
  Select,
  Table,
  Row,
  Col,
} from 'antd';
import { 
  PlusOutlined, 
  ReloadOutlined
} from '@ant-design/icons';
import api from '../services/api';

// 通用组件和Hooks
import PageHeader from '../components/Common/PageHeader';
import TradeDrawer from '../components/Trading/TradeDrawer';
import TradingPairCard from '../components/Trading/TradingPairCard';
import useAccountData from '../hooks/useAccountData';
import useEstimates from '../hooks/useEstimates';
import usePriceData from '../hooks/usePriceData';
import { DEFAULT_CONFIG } from '../utils/constants';

const { Text } = Typography;

const { Option } = Select;

const TradingPairs = () => {
  // 移动端检测
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 768);
  
  // 基础状态
  const [selectedPairs, setSelectedPairs] = useState([]);
  const [allPairs, setAllPairs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false); // 刷新状态
  const [drawerVisible, setDrawerVisible] = useState(false);
  
  // 监听窗口大小变化
  useEffect(() => {
    const handleResize = () => {
      setIsMobile(window.innerWidth <= 768);
    };
    
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);
  const [syncing, setSyncing] = useState(false);
  const [searchValue, setSearchValue] = useState('');
  const [filteredPairs, setFilteredPairs] = useState([]);
  
  // 排序相关状态
  const [sortBy, setSortBy] = useState('price_change_percent'); // 排序字段
  const [sortOrder, setSortOrder] = useState('desc'); // 排序顺序

  // 交易相关状态
  const [tradeModalVisible, setTradeModalVisible] = useState(false);
  const [selectedTradeSymbol, setSelectedTradeSymbol] = useState('');
  const [tradeSide, setTradeSide] = useState('long');
  const [selectedLeverage, setSelectedLeverage] = useState(DEFAULT_CONFIG.leverage);
  const [targetPrice, setTargetPrice] = useState(0);
  const [orderType, setOrderType] = useState(DEFAULT_CONFIG.orderType);
  const [quantity, setQuantity] = useState(0.001);
  const [marginMode, setMarginMode] = useState('isolated'); // 默认逐仓模式

  // 使用自定义Hooks
  const { 
    accountValue, 
    hasPosition, 
    hasAnyPosition,
    positionCount,
    lastUpdate: accountLastUpdate,
    error: accountError
  } = useAccountData();

  const { 
    symbolEstimates, 
    hasAnyEstimate,
    fetchSymbolEstimates 
  } = useEstimates();

  // 使用全局价格数据管理
  const { 
    priceData, 
    loading: priceLoading,
    lastUpdate: priceLastUpdate,
    refreshPriceData,
    getPriceBySymbol,
    priceCount,
    validPriceCount
  } = usePriceData();

  // 初始化
  useEffect(() => {
    fetchSelectedPairs();
    fetchAllPairs();
  }, []);

  // 搜索过滤和排序
  useEffect(() => {
    let filtered = allPairs;
    
    // 先进行搜索过滤
    if (searchValue) {
      filtered = allPairs.filter(pair => 
        pair.symbol.toLowerCase().includes(searchValue.toLowerCase()) ||
        pair.base_asset.toLowerCase().includes(searchValue.toLowerCase()) ||
        pair.quote_asset.toLowerCase().includes(searchValue.toLowerCase())
      );
    }
    
    // 再进行排序
    const sorted = [...filtered].sort((a, b) => {
      let aValue, bValue;
      
      switch (sortBy) {
        case 'symbol':
          aValue = a.symbol;
          bValue = b.symbol;
          break;
        case 'price_change_percent':
          aValue = parseFloat(a.price_change_percent || '0');
          bValue = parseFloat(b.price_change_percent || '0');
          break;
        case 'volume':
          aValue = parseFloat(a.volume || '0');
          bValue = parseFloat(b.volume || '0');
          break;
        case 'quote_volume':
          aValue = parseFloat(a.quote_volume || '0');
          bValue = parseFloat(b.quote_volume || '0');
          break;
        case 'price':
          aValue = parseFloat(a.price || '0');
          bValue = parseFloat(b.price || '0');
          break;
        default:
          aValue = a.symbol;
          bValue = b.symbol;
      }
      
      // 字符串比较
      if (typeof aValue === 'string' && typeof bValue === 'string') {
        return sortOrder === 'asc' ? aValue.localeCompare(bValue) : bValue.localeCompare(aValue);
      }
      
      // 数字比较
      if (sortOrder === 'asc') {
        return aValue - bValue;
      } else {
        return bValue - aValue;
      }
    });
    
    setFilteredPairs(sorted);
  }, [searchValue, allPairs, sortBy, sortOrder]);



  // 获取选中的交易对
  const fetchSelectedPairs = async () => {
    try {
      const response = await api.get('/coins/selected');
      const pairs = response.data.data || [];
      setSelectedPairs(pairs);
    } catch (error) {
      message.error('获取选中交易对失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchAllPairs = async () => {
    try {
      const response = await api.get('/coins');
      setAllPairs(response.data.data || []);
    } catch (error) {
      message.error('获取交易对列表失败');
    }
  };



  // 同步交易对
  const syncPairs = async () => {
    setSyncing(true);
    try {
      await api.post('/coins/sync');
      message.success('交易对同步成功');
      fetchAllPairs();
    } catch (error) {
      message.error('同步失败');
    } finally {
      setSyncing(false);
    }
  };

  // 添加交易对
  const addPair = async (symbol) => {
    try {
      await api.post('/coins/select', {
        symbol,
        is_selected: true
      });
      message.success(`${symbol} 添加成功`);
      await fetchSelectedPairs();
      // 价格数据会自动更新，无需手动刷新
      setDrawerVisible(false);
    } catch (error) {
      message.error(`${symbol} 添加失败`);
    }
  };

  // 删除交易对
  const removePair = async (symbol) => {
    if (!canDeleteSymbol(symbol)) {
      const reason = getDeleteDisabledReason(symbol);
      message.error(`${symbol} ${reason}`);
      return;
    }

    try {
      await api.post('/coins/select', {
        symbol,
        is_selected: false
      });
      message.success(`${symbol} 删除成功`);
      fetchSelectedPairs();
      fetchSymbolEstimates();
    } catch (error) {
      message.error(`${symbol} 删除失败`);
    }
  };

  // 检查交易对是否可以删除
  const canDeleteSymbol = (symbol) => {
    return !hasAnyPosition(symbol) && !hasAnyEstimate(symbol);
  };

  // 获取删除禁用原因
  const getDeleteDisabledReason = (symbol) => {
    const hasPos = hasAnyPosition(symbol);
    const hasEst = hasAnyEstimate(symbol);
    
    if (hasPos && hasEst) {
      return `存在持仓和监听，无法删除`;
    } else if (hasPos) {
      return `存在未平仓持仓，无法删除`;
    } else if (hasEst) {
      return `存在${symbolEstimates[symbol]}个价格监听，无法删除`;
    }
    return '';
  };

  const isSelected = (symbol) => {
    return selectedPairs.some(pair => pair.symbol === symbol);
  };

  // 打开交易模态框
  const openTradeModal = (symbol, side) => {
    if (hasPosition(symbol, side)) {
      const positionText = side === 'long' ? '多头' : '空头';
      message.warning(`${symbol} 已有${positionText}仓位 | 无法重复开仓`);
      return;
    }

    setSelectedTradeSymbol(symbol);
    setTradeSide(side);
    setSelectedLeverage(DEFAULT_CONFIG.leverage);
    setOrderType(DEFAULT_CONFIG.orderType);
    
    // 设置初始目标价格
    const price = priceData[symbol];
    let initialPrice = 0;
    if (price && price.hasValidData) {
      initialPrice = price.currentPrice;
    }
    setTargetPrice(initialPrice);
    
    // 计算默认开仓数量
    const defaultQuantity = getDefaultQuantity(symbol, initialPrice);
    setQuantity(defaultQuantity);
    
    // 重置状态到默认值
    setSelectedLeverage(DEFAULT_CONFIG.leverage);
    setOrderType(DEFAULT_CONFIG.orderType);
    setMarginMode('isolated');
    
    setTradeModalVisible(true);
  };

  // 计算默认开仓数量
  const getDefaultQuantity = (symbol, price) => {
    if (!price || !accountValue?.usdt_free) return 0.001;
    const maxUsdtAmount = accountValue.usdt_free * 0.2; // 使用20%的可用余额作为默认
    const defaultQuantity = (maxUsdtAmount * selectedLeverage) / price;
    return Math.max(0.001, parseFloat(defaultQuantity.toFixed(6)));
  };

  const getCurrentPrice = () => {
    const price = getPriceBySymbol(selectedTradeSymbol);
    return price?.markPrice || price?.currentPrice || 0;
  };

  // 统一的卡片操作处理
  const handleCardAction = (symbol, action) => {
    switch (action) {
      case 'long':
        openTradeModal(symbol, 'long');
        break;
      case 'short':
        openTradeModal(symbol, 'short');
        break;
      case 'delete':
        removePair(symbol);
        break;
      default:
        break;
    }
  };

  // 简化的处理函数
  const handlePriceChange = (value) => {
    setTargetPrice(value || 0);
  };

  const handleLeverageChange = (value) => {
    setSelectedLeverage(value);
  };

  const handleOrderTypeChange = (value) => {
    setOrderType(value);
  };

  // 创建交易
  const createTrade = async () => {
    try {
      const currentPrice = getCurrentPrice();
      if (!currentPrice || currentPrice <= 0) {
        message.error('价格数据不可用，请稍后再试');
        return;
      }

      // 检查开仓数量
      if (!quantity || quantity <= 0) {
        message.error('请设置有效的开仓数量');
        return;
      }
      
      // 根据订单类型确定价格
      let orderPrice = currentPrice;
      if (orderType === 'limit') {
        orderPrice = targetPrice || currentPrice;
        if (!orderPrice || orderPrice <= 0) {
          message.error('请设置有效的限价价格');
          return;
        }
      }

      const orderData = {
        symbol: selectedTradeSymbol,
        side: tradeSide,
        action_type: 'open',
        target_price: orderPrice,
        quantity: quantity,
        leverage: selectedLeverage,
        margin_mode: marginMode,
        order_type: orderType,
        trigger_type: 'condition',
        created_by: 'open_position'
      };

      await api.post('/estimates', orderData);
      
      const actionText = tradeSide === 'long' ? '开多' : '开空';
      const orderTypeText = orderType === 'market' ? '市价' : '限价';
      
      const baseAsset = selectedTradeSymbol.replace('USDT', '');
      const marginModeText = marginMode === 'cross' ? '全仓' : '逐仓';
      message.success(`${actionText}预估价已创建 | ${orderTypeText}单 ${selectedLeverage}x杠杆 ${marginModeText} ${quantity} ${baseAsset}`);
      setTradeModalVisible(false);
      fetchSymbolEstimates();
    } catch (error) {
      message.error('创建订单失败: ' + (error.response?.data?.error || error.message));
    }
  };

  // 页面操作配置
  const headerActions = [
    <button 
      key="refresh"
      className="control-btn primary-btn trading-pairs-header-btn"
      onClick={async () => {
        setRefreshing(true);
        try {
          await fetchSelectedPairs();
          refreshPriceData();
        } catch (error) {
          console.error('刷新失败:', error);
        } finally {
          setRefreshing(false);
        }
      }}
      disabled={refreshing}
    >
      <ReloadOutlined style={{ marginRight: 4 }} />
      刷新
    </button>,
    <button 
      key="add"
      className="control-btn success-btn trading-pairs-header-btn"
      onClick={() => setDrawerVisible(true)}
    >
      <PlusOutlined style={{ marginRight: 4 }} />
      添加交易对
    </button>
  ];

  if (loading) {
    return <Spin size="large" style={{ display: 'block', textAlign: 'center', padding: '50px' }} />;
  }

  return (
    <div>
      <PageHeader 
        title="币种" 
        actions={headerActions}
      />

      <div style={{ marginBottom: 16 }}>
        <Badge count={selectedPairs.length} offset={[10, 0]}>
          <Text strong>已选中的交易对</Text>
        </Badge>
      </div>

      {selectedPairs.length === 0 ? (
        <Empty description="暂无选中的交易对" />
      ) : (
        <Row gutter={[16, 16]}>
          {selectedPairs.map((pair) => (
            <Col 
              xs={12} 
              sm={12} 
              md={8} 
              lg={6} 
              xl={4} 
              key={pair.symbol}
            >
              <TradingPairCard
                pair={pair}
                priceInfo={getPriceBySymbol(pair.symbol)}
                onAction={handleCardAction}
                hasPosition={hasPosition}
                hasAnyPosition={hasAnyPosition}
                hasAnyEstimate={hasAnyEstimate}
                symbolEstimates={symbolEstimates}
                canDeleteSymbol={canDeleteSymbol}
                getDeleteDisabledReason={getDeleteDisabledReason}
                isMobile={isMobile}
              />
            </Col>
          ))}
        </Row>
      )}

      {/* 添加交易对抽屉 */}
      <Drawer
        title="添加交易对"
        open={drawerVisible}
        onClose={() => setDrawerVisible(false)}
        width={isMobile ? '100%' : 800}
        placement="right"
        styles={{
          body: { 
            padding: isMobile ? '12px' : '16px',
            height: '100%',
            overflow: 'hidden',
            display: 'flex',
            flexDirection: 'column'
          }
        }}
      >
        {/* 固定顶部控制区 */}
        <div style={{ 
          flexShrink: 0,
          marginBottom: '12px',
          paddingBottom: '12px',
          borderBottom: '1px solid #f0f0f0'
        }}>
          {/* 同步按钮 */}
          <button 
            className="control-btn primary-btn trading-pairs-header-btn"
            onClick={syncPairs}
            disabled={syncing}
            style={{ 
              marginBottom: '8px',
              height: '32px',
              fontSize: isMobile ? '12px' : '13px',
              width: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
          >
            <ReloadOutlined style={{ marginRight: 4 }} />
            从交易所同步最新交易对
          </button>
          
          {/* 搜索框 - 统一样式 */}
          <Input.Search
            placeholder="搜索交易对"
            value={searchValue}
            onChange={(e) => setSearchValue(e.target.value)}
            style={{ 
              width: '100%',
              marginBottom: '8px'
            }}
            size="default"
            allowClear
            bordered={true}
          />
          
          {/* 排序控制 - 强制水平布局 */}
          <div 
            style={{ 
              display: 'flex !important', 
              flexDirection: 'row !important',
              alignItems: 'center !important',
              gap: '6px',
              width: '100%',
              height: '32px',
              marginTop: '0px',
              marginBottom: '0px'
            }}
          >
            <div style={{ 
              fontSize: '12px', 
              color: '#888',
              width: '28px',
              flexShrink: 0,
              lineHeight: '32px',
              textAlign: 'left',
              display: 'inline-block'
            }}>
              排序
            </div>
            <div style={{ flex: 1, display: 'inline-block' }}>
              <Select
                value={sortBy}
                onChange={setSortBy}
                style={{ 
                  width: '100%',
                  height: '32px'
                }}
                size="small"
                placeholder="排序方式"
              >
                <Option value="price_change_percent">涨跌幅</Option>
                <Option value="volume">成交量</Option>
                <Option value="quote_volume">成交额</Option>
                <Option value="price">价格</Option>
                <Option value="symbol">名称</Option>
              </Select>
            </div>
            <div style={{ width: '75px', flexShrink: 0, display: 'inline-block' }}>
              <Select
                value={sortOrder}
                onChange={setSortOrder}
                style={{ 
                  width: '100%',
                  height: '32px'
                }}
                size="small"
              >
                <Option value="desc">降序</Option>
                <Option value="asc">升序</Option>
              </Select>
            </div>
          </div>
        </div>

        {/* 可滚动内容区 */}
        <div style={{ 
          flex: 1,
          overflow: 'auto',
          minHeight: 0
        }}>
          <Table
            dataSource={filteredPairs.slice(0, 500)}
            pagination={{ 
              pageSize: isMobile ? 15 : 20, 
              showSizeChanger: false,
              showQuickJumper: !isMobile,
              size: 'small',
              showTotal: (total, range) => `共 ${total} 条，显示 ${range[0]}-${range[1]} 条`
            }}
            size="small"
            scroll={{ y: 'calc(100vh - 400px)' }}
            columns={[
              {
                title: '交易对',
                dataIndex: 'symbol',
                key: 'symbol',
                width: isMobile ? 80 : 120,
                render: (text, record) => (
                  <div>
                    <div style={{ 
                      fontWeight: 'bold', 
                      fontSize: isMobile ? '12px' : '14px',
                      lineHeight: 1.2
                    }}>
                      {text}
                    </div>
                    <div style={{ 
                      fontSize: isMobile ? '10px' : '12px', 
                      color: '#666',
                      marginTop: '2px'
                    }}>
                      {record.base_asset}/{record.quote_asset}
                    </div>
                  </div>
                ),
              },
              {
                title: '价格',
                dataIndex: 'price',
                key: 'price',
                width: isMobile ? 70 : 100,
                align: 'right',
                render: (price) => (
                  <span style={{ fontSize: isMobile ? '11px' : '13px' }}>
                    {price ? parseFloat(price).toFixed(4) : '-'}
                  </span>
                ),
              },
              {
                title: '涨跌幅',
                dataIndex: 'price_change_percent',
                key: 'price_change_percent',
                width: isMobile ? 60 : 90,
                align: 'right',
                render: (percent) => {
                  if (!percent) return '-';
                  const value = parseFloat(percent);
                  return (
                    <span style={{ 
                      color: value >= 0 ? '#3f8600' : '#cf1322',
                      fontWeight: 'bold',
                      fontSize: isMobile ? '10px' : '12px'
                    }}>
                      {value >= 0 ? '+' : ''}{value.toFixed(2)}%
                    </span>
                  );
                },
              },
              ...(isMobile ? [] : [
                {
                  title: '成交量',
                  dataIndex: 'volume',
                  key: 'volume',
                  width: 100,
                  align: 'right',
                  render: (volume) => volume ? `${(parseFloat(volume) / 1000).toFixed(0)}K` : '-',
                },
                {
                  title: '成交额',
                  dataIndex: 'quote_volume', 
                  key: 'quote_volume',
                  width: 100,
                  align: 'right',
                  render: (quoteVolume) => quoteVolume ? `${(parseFloat(quoteVolume) / 1000000).toFixed(1)}M` : '-',
                }
              ]),
              {
                title: '操作',
                key: 'action',
                width: isMobile ? 50 : 80,
                align: 'center',
                render: (_, record) => 
                  isSelected(record.symbol) ? (
                    <Tag 
                      color="success" 
                      style={{ 
                        fontSize: isMobile ? '10px' : '12px',
                        margin: 0
                      }}
                    >
                      已选
                    </Tag>
                  ) : (
                    <button 
                      className="control-btn success-btn trading-pairs-action-btn"
                      onClick={() => addPair(record.symbol)}
                      style={{ 
                        fontSize: isMobile ? '10px' : '12px',
                        padding: isMobile ? '0 4px' : '0 8px'
                      }}
                    >
                      添加
                    </button>
                  )
              }
            ]}
          />
          
          {filteredPairs.length === 0 && searchValue && (
            <div style={{ padding: '40px 20px', textAlign: 'center' }}>
              <Empty 
                description="未找到匹配的交易对" 
                imageStyle={{ height: 60 }}
              />
            </div>
          )}
        </div>
      </Drawer>

      {/* 交易抽屉 */}
      <TradeDrawer
        visible={tradeModalVisible}
        onClose={() => {
          setTradeModalVisible(false);
          // 价格数据会自动更新，无需手动刷新
        }}
        symbol={selectedTradeSymbol}
        side={tradeSide}
        onSubmit={createTrade}
        priceData={priceData}
        accountValue={accountValue}
        targetPrice={targetPrice}
        onPriceChange={handlePriceChange}
        quantity={quantity}
        onQuantityChange={setQuantity}
        orderType={orderType}
        onOrderTypeChange={handleOrderTypeChange}
        selectedLeverage={selectedLeverage}
        onLeverageChange={handleLeverageChange}
        marginMode={marginMode}
        onMarginModeChange={setMarginMode}
      />
      
      {/* 全局状态显示 */}
      <div style={{ 
        position: 'fixed', 
        bottom: '20px', 
        right: '20px', 
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
        zIndex: 1000
      }}>
        {/* 价格数据状态 */}
        <div style={{ 
          backgroundColor: 'rgba(0, 0, 0, 0.8)', 
          color: 'white', 
          padding: '8px 12px', 
          borderRadius: '6px', 
          fontSize: '12px',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          boxShadow: '0 2px 8px rgba(0,0,0,0.15)'
        }}>
          <div style={{ 
            width: '8px', 
            height: '8px', 
            borderRadius: '50%', 
            backgroundColor: priceLoading ? '#ffa500' : (validPriceCount > 0 ? '#52c41a' : '#ff4d4f')
          }} />
          <span>
            价格: {validPriceCount}/{priceCount} 
            {priceLastUpdate && ` · ${priceLastUpdate.toLocaleTimeString()}`}
          </span>
        </div>
        
        {/* 账户数据状态 */}
        <div style={{ 
          backgroundColor: 'rgba(0, 0, 0, 0.8)', 
          color: 'white', 
          padding: '8px 12px', 
          borderRadius: '6px', 
          fontSize: '12px',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          boxShadow: '0 2px 8px rgba(0,0,0,0.15)'
        }}>
          <div style={{ 
            width: '8px', 
            height: '8px', 
            borderRadius: '50%', 
            backgroundColor: accountError ? '#ff4d4f' : (accountValue ? '#52c41a' : '#ffa500')
          }} />
          <span>
            账户: {positionCount}仓位 
            {accountLastUpdate && ` · ${accountLastUpdate.toLocaleTimeString()}`}
          </span>
        </div>
      </div>
    </div>
  );
};

export default TradingPairs;
