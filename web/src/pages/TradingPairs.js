import React, { useState, useEffect, useCallback } from 'react';
import { 
  Row, 
  Col, 
  Button, 
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
} from 'antd';
import { 
  PlusOutlined, 
  ReloadOutlined
} from '@ant-design/icons';
import api, { getBatchCoinPrecision } from '../services/api';
import { 
  validatePriceComplete, 
  validateQuantityComplete, 
  autoFixPriceAndQuantity,
  formatPrice
} from '../utils/precision';

// 通用组件和Hooks
import PageHeader from '../components/Common/PageHeader';
import TradingPairCard from '../components/Trading/TradingPairCard';
import TradeDrawer from '../components/Trading/TradeDrawer';
import useAccountData from '../hooks/useAccountData';
import useEstimates from '../hooks/useEstimates';
import { DEFAULT_CONFIG } from '../utils/constants';

const { Text } = Typography;

const { Option } = Select;

const TradingPairs = () => {
  // 移动端检测
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 768);
  
  // 基础状态
  const [selectedPairs, setSelectedPairs] = useState([]);
  const [allPairs, setAllPairs] = useState([]);
  const [priceData, setPriceData] = useState({});
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
  const [precisionInfo, setPrecisionInfo] = useState({});
  
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
  const [usdtAmount, setUsdtAmount] = useState(10);
  const [coinQuantity, setCoinQuantity] = useState(0);

  // 使用自定义Hooks
  const { 
    accountValue, 
    positionsMap, 
    hasPosition, 
    hasAnyPosition
  } = useAccountData();

  const { 
    symbolEstimates, 
    hasAnyEstimate,
    fetchSymbolEstimates 
  } = useEstimates();

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
      
      // 获取精度信息
      if (pairs.length > 0) {
        const symbols = pairs.map(pair => pair.symbol);
        const precision = await getBatchCoinPrecision(symbols);
        setPrecisionInfo(precision);
      }
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

  // 获取价格数据
  const fetchPriceData = useCallback(async () => {
    if (selectedPairs.length === 0) return;

    try {
      const pricePromises = selectedPairs.map(async (pair) => {
        try {
          const response = await api.get(`/monitor/orderbook/${pair.symbol}`);
          return {
            symbol: pair.symbol,
            orderbook: response.data.data
          };
        } catch (error) {
          console.error(`获取 ${pair.symbol} 订单薄失败:`, error);
          return {
            symbol: pair.symbol,
            orderbook: null
          };
        }
      });

      const results = await Promise.all(pricePromises);
      const newPriceData = {};
      
      results.forEach(result => {
        if (result) {
          const { symbol, orderbook } = result;
          
          let bestBid = 0;
          let bestAsk = 0;
          let midPrice = 0;
          let hasValidData = false;
          
          if (orderbook && orderbook.bids && orderbook.asks) {
            if (orderbook.bids.length > 0 && orderbook.asks.length > 0) {
              bestBid = parseFloat(orderbook.bids[0].price);
              bestAsk = parseFloat(orderbook.asks[0].price);
              if (bestBid > 0 && bestAsk > 0) {
                midPrice = (bestBid + bestAsk) / 2;
                hasValidData = true;
              }
            }
          }
          
          if (hasValidData) {
            newPriceData[symbol] = {
              currentPrice: midPrice,
              bestBid,
              bestAsk,
              lastUpdate: new Date(),
              orderbook: orderbook,
              hasValidData: hasValidData
            };
          }
        }
      });
      
      setPriceData(prev => ({ ...prev, ...newPriceData }));
    } catch (error) {
      console.error('批量获取价格数据失败:', error);
    }
  }, [selectedPairs]);

  // 定期更新价格数据
  useEffect(() => {
    if (selectedPairs.length > 0) {
      fetchPriceData();
      const interval = setInterval(fetchPriceData, DEFAULT_CONFIG.refreshInterval.price);
      return () => clearInterval(interval);
    }
  }, [selectedPairs, fetchPriceData]);

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
      setTimeout(fetchPriceData, 1000);
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
    
    // 计算初始币数量
    const defaultUsdtAmount = Math.min(getMaxUsdtAmount() * 0.2, 100);
    const initialQuantity = calculateCoinQuantity(defaultUsdtAmount, initialPrice, DEFAULT_CONFIG.leverage);
    setCoinQuantity(initialQuantity);
    setUsdtAmount(defaultUsdtAmount);
    
    // 重置状态到默认值
    setSelectedLeverage(DEFAULT_CONFIG.leverage);
    setOrderType(DEFAULT_CONFIG.orderType);
    setTargetPrice(initialPrice);
    
    setTradeModalVisible(true);
  };

  // 计算相关函数
  const calculateCoinQuantity = (usdtAmount, price, leverage) => {
    if (price > 0 && leverage > 0) {
      const quantity = (usdtAmount * leverage) / price;
      return parseFloat(quantity.toFixed(6));
    }
    return 0;
  };

  const calculateUsdtAmount = (coinQuantity, price, leverage) => {
    if (price > 0 && leverage > 0) {
      const usdt = (coinQuantity * price) / leverage;
      return parseFloat(usdt.toFixed(2));
    }
    return 0;
  };

  const getMaxUsdtAmount = () => {
    return Math.floor(accountValue.usdt_free * 0.5);
  };

  const getMaxCoinQuantity = () => {
    const currentPrice = getCurrentPrice();
    const maxUsdtAmount = getMaxUsdtAmount();
    
    if (currentPrice > 0 && maxUsdtAmount > 0 && selectedLeverage > 0) {
      const maxQuantity = (maxUsdtAmount * selectedLeverage) / currentPrice;
      return parseFloat(maxQuantity.toFixed(6));
    }
    return 1000;
  };

  const getCurrentPrice = () => {
    if (orderType === 'market') {
      return priceData[selectedTradeSymbol]?.currentPrice || 0;
    } else {
      return targetPrice || priceData[selectedTradeSymbol]?.currentPrice || 0;
    }
  };

  const getPriceRange = () => {
    const currentPrice = priceData[selectedTradeSymbol];
    if (!currentPrice?.hasValidData) return [0, 1000];
    
    const basePrice = currentPrice.currentPrice;
    const minPrice = basePrice * 0.8; // -20%
    const maxPrice = basePrice * 1.2; // +20%
    return [minPrice, maxPrice];
  };

  const calculateUsdtRatio = () => {
    const maxAmount = getMaxUsdtAmount();
    return maxAmount > 0 ? (usdtAmount / maxAmount) * 100 : 0;
  };

  // 处理函数
  const handlePriceChange = (value) => {
    const coinInfo = precisionInfo[selectedTradeSymbol];
    let finalPrice = value || 0;
    
    if (coinInfo && value) {
      const { price: adjustedPrice } = autoFixPriceAndQuantity(value, 0, coinInfo);
      finalPrice = adjustedPrice;
      
      if (Math.abs(adjustedPrice - value) > 1e-8) {
        message.info(`价格已自动调整为符合精度要求的 ${formatPrice(adjustedPrice, coinInfo.price_precision)}`);
      }
    }
    
    setTargetPrice(finalPrice);
    
    // 重新计算币数量和USDT金额
    const currentPrice = getCurrentPrice();
    if (currentPrice > 0) {
      const newMaxQuantity = getMaxCoinQuantity();
      let newCoinQuantity = coinQuantity;
      
      if (coinQuantity > newMaxQuantity) {
        newCoinQuantity = newMaxQuantity;
        setCoinQuantity(newCoinQuantity);
      }
      
      const usdt = calculateUsdtAmount(newCoinQuantity, currentPrice, selectedLeverage);
      setUsdtAmount(usdt);
    }
  };

  const handleQuantityChange = (value) => {
    let finalQuantity = value;
    
    if (precisionInfo[selectedTradeSymbol]) {
      const { quantity: adjustedQuantity } = autoFixPriceAndQuantity(0, value, precisionInfo[selectedTradeSymbol]);
      finalQuantity = adjustedQuantity;
    }
    
    setCoinQuantity(finalQuantity);
    
    const currentPrice = getCurrentPrice();
    if (currentPrice > 0) {
      const usdt = calculateUsdtAmount(finalQuantity, currentPrice, selectedLeverage);
      setUsdtAmount(usdt);
      // USDT金额已通过setUsdtAmount更新
    }
  };

  const handleSliderChange = (value) => {
    let finalPrice = value;
    
    if (precisionInfo[selectedTradeSymbol]) {
      const { price: adjustedPrice } = autoFixPriceAndQuantity(value, 0, precisionInfo[selectedTradeSymbol]);
      finalPrice = adjustedPrice;
    }
    
    setTargetPrice(finalPrice);
    // 目标价格已通过setState更新
    
    // 重新计算相关数值
    const currentPrice = getCurrentPrice();
    if (currentPrice > 0) {
      const newMaxQuantity = getMaxCoinQuantity();
      let newCoinQuantity = coinQuantity;
      
      if (coinQuantity > newMaxQuantity) {
        newCoinQuantity = newMaxQuantity;
        setCoinQuantity(newCoinQuantity);
      }
      
      const usdt = calculateUsdtAmount(newCoinQuantity, currentPrice, selectedLeverage);
      setUsdtAmount(usdt);
      // USDT金额已通过setUsdtAmount更新
    }
  };

  const handleLeverageChange = (value) => {
    setSelectedLeverage(value);
    const currentPrice = getCurrentPrice();
    if (currentPrice > 0) {
      const newMaxQuantity = getMaxCoinQuantity();
      let newCoinQuantity = coinQuantity;
      
      if (coinQuantity > newMaxQuantity) {
        newCoinQuantity = newMaxQuantity;
        setCoinQuantity(newCoinQuantity);
      }
      
      const usdt = calculateUsdtAmount(newCoinQuantity, currentPrice, value);
      setUsdtAmount(usdt);
    }
  };

  const handleOrderTypeChange = (value) => {
    setOrderType(value);
    const currentPrice = getCurrentPrice();
    if (currentPrice > 0) {
      const newMaxQuantity = getMaxCoinQuantity();
      let newCoinQuantity = coinQuantity;
      
      if (coinQuantity > newMaxQuantity) {
        newCoinQuantity = newMaxQuantity;
        setCoinQuantity(newCoinQuantity);
      }
      
      const usdt = calculateUsdtAmount(newCoinQuantity, currentPrice, selectedLeverage);
      setUsdtAmount(usdt);
    }
  };

  // 创建交易
  const createTrade = async (values) => {
    try {
      const price = priceData[selectedTradeSymbol];
      if (!price?.hasValidData) {
        message.error('价格数据不可用，请稍后再试');
        return;
      }

      const coinInfo = precisionInfo[selectedTradeSymbol];
      
      // 根据订单类型确定价格
      let orderPrice;
      if (values.orderType === 'market') {
        orderPrice = tradeSide === 'long' ? price.bestAsk : price.bestBid;
      } else {
        orderPrice = targetPrice;
        if (!orderPrice || orderPrice <= 0) {
          message.error('请设置有效的限价价格');
          return;
        }
        
        if (coinInfo) {
          const priceValidation = validatePriceComplete(orderPrice, coinInfo);
          if (!priceValidation.valid) {
            message.error(`价格精度错误: ${priceValidation.error}`);
            return;
          }
        }
      }

      if (!coinQuantity || coinQuantity <= 0) {
        message.error('币数量计算异常，请重新操作');
        return;
      }
      
      // 在提交前再次修正精度，确保完全符合要求
      let finalQuantity = coinQuantity;
      if (coinInfo) {
        const { quantity: adjustedQuantity } = autoFixPriceAndQuantity(0, coinQuantity, coinInfo);
        finalQuantity = adjustedQuantity;
        
        const quantityValidation = validateQuantityComplete(finalQuantity, coinInfo);
        if (!quantityValidation.valid) {
          message.error(`数量精度错误: ${quantityValidation.error}`);
          return;
        }
      }

      const orderData = {
        symbol: selectedTradeSymbol,
        side: tradeSide,
        action_type: 'open',
        target_price: orderPrice,
        quantity: finalQuantity,
        leverage: values.leverage,
        margin_mode: values.marginMode || 'isolated',
        order_type: values.orderType,
        trigger_type: 'condition',
        created_by: 'open_position'
      };

      await api.post('/estimates', orderData);
      
      const actionText = tradeSide === 'long' ? '开多' : '开空';
      const orderTypeText = values.orderType === 'market' ? '市价' : '限价';
      
      message.success(`${actionText}预估价已创建 | ${orderTypeText}单 ${values.leverage}x杠杆 ${usdtAmount} USDT`);
      setTradeModalVisible(false);
      fetchSymbolEstimates();
    } catch (error) {
      message.error('创建订单失败: ' + (error.response?.data?.error || error.message));
    }
  };

  // 页面操作配置
  const headerActions = [
    {
      icon: <ReloadOutlined />,
      loading: refreshing,
      onClick: async () => {
        setRefreshing(true);
        try {
          await fetchSelectedPairs();
          setTimeout(() => fetchPriceData(), 500);
        } catch (error) {
          console.error('刷新失败:', error);
        } finally {
          setRefreshing(false);
        }
      },
      children: '刷新'
    },
    {
      icon: <PlusOutlined />,
      type: 'primary',
      onClick: () => setDrawerVisible(true),
      children: '添加交易对'
    }
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
            <Col xs={24} sm={12} md={8} lg={6} xl={4} key={pair.symbol}>
              <TradingPairCard
                pair={pair}
                priceData={priceData}
                positions={positionsMap}
                symbolEstimates={symbolEstimates}
                onDelete={removePair}
                onTrade={openTradeModal}
                hasPosition={hasPosition}
                canDeleteSymbol={canDeleteSymbol}
                getDeleteDisabledReason={getDeleteDisabledReason}
                hasAnyEstimate={hasAnyEstimate}
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
          <Button 
            icon={<ReloadOutlined />} 
            onClick={syncPairs}
            loading={syncing}
            type="dashed"
            size="small"
            block
            style={{ 
              marginBottom: '8px',
              height: '32px',
              fontSize: isMobile ? '12px' : '13px'
            }}
          >
            从交易所同步最新交易对
          </Button>
          
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
                    <Button 
                      type="primary" 
                      size="small"
                      onClick={() => addPair(record.symbol)}
                      style={{ 
                        fontSize: isMobile ? '10px' : '12px',
                        padding: isMobile ? '0 4px' : '0 8px'
                      }}
                    >
                      添加
                    </Button>
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
          setTimeout(() => {
            if (selectedPairs.length > 0) {
              fetchPriceData();
            }
          }, 100);
        }}
        symbol={selectedTradeSymbol}
        side={tradeSide}

        onSubmit={createTrade}
        priceData={priceData}
        precisionInfo={precisionInfo}
        accountValue={accountValue}
        targetPrice={targetPrice}
        onPriceChange={handlePriceChange}
        coinQuantity={coinQuantity}
        onQuantityChange={handleQuantityChange}
        usdtAmount={usdtAmount}
        orderType={orderType}
        onOrderTypeChange={handleOrderTypeChange}
        selectedLeverage={selectedLeverage}
        onLeverageChange={handleLeverageChange}
        getMaxCoinQuantity={getMaxCoinQuantity}
        getPriceRange={getPriceRange}
        handleSliderChange={handleSliderChange}
        calculateUsdtRatio={calculateUsdtRatio}
      />
    </div>
  );
};

export default TradingPairs;
