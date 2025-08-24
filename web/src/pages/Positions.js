import React, { useState, useEffect } from 'react';
import { 
  Row, 
  Col, 
  Drawer, 
  Typography, 
  message,
  Spin,
  Empty
} from 'antd';
import { 
  ReloadOutlined
} from '@ant-design/icons';
import api, { getCoinPrecision, toggleEstimateEnabled } from '../services/api';
import { autoFixPriceAndQuantity, validatePriceComplete, validateQuantityComplete } from '../utils/precision';

// 通用组件和Hooks
import PageHeader from '../components/Common/PageHeader';
import PositionCard from '../components/Position/PositionCard';
import PriceSlider from '../components/Trading/PriceSlider';
import QuantitySlider from '../components/Trading/QuantitySlider';
import MonitoringCard from '../components/Monitoring/MonitoringCard';
import useAccountData from '../hooks/useAccountData';
import useEstimates from '../hooks/useEstimates';
import { ACTIONS } from '../utils/constants';

const { Text } = Typography;

const Positions = () => {
  // 操作相关状态
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [refreshing, setRefreshing] = useState(false); // 刷新状态
  const [currentPosition, setCurrentPosition] = useState(null);
  const [actionType, setActionType] = useState('');
  const [currentPrice, setCurrentPrice] = useState(0);
  const [targetPrice, setTargetPrice] = useState(0);
  const [quantity, setQuantity] = useState(0);
  const [usdtAmount, setUsdtAmount] = useState(0);
  const [pricePercentage, setPricePercentage] = useState(0);
  const [coinPrecision, setCoinPrecision] = useState(null);

  // 详情抽屉相关状态
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [detailPosition, setDetailPosition] = useState(null);
  const [positionEstimates, setPositionEstimates] = useState([]);
  const [estimatesLoading, setEstimatesLoading] = useState(false);

  // 使用自定义Hooks
  const { 
    accountValue, 
    positions, 
    loading: positionsLoading,
    fetchPositions,
    fetchAccountInfo 
  } = useAccountData();

  // 监听数量状态
  const [positionsWithMonitors, setPositionsWithMonitors] = useState([]);

  const { deleteEstimate } = useEstimates();

  // 监听positions变化，获取监听数量
  useEffect(() => {
    if (positions.length > 0) {
      fetchPositionsWithMonitors(positions);
    } else {
      setPositionsWithMonitors([]);
    }
  }, [positions]);

  // 获取所有持仓的监听数量
  const fetchPositionsWithMonitors = async (positionsData) => {
    try {
      const positionsWithCount = await Promise.all(
        positionsData.map(async (position) => {
          try {
            const estimates = await api.get(`/estimates?symbol=${position.symbol}`);
            const filteredEstimates = (estimates.data.data || []).filter(estimate => 
              estimate.side === position.side.toLowerCase() && estimate.status === 'listening'
            );
            return {
              ...position,
              monitorCount: filteredEstimates.length
            };
          } catch (error) {
            console.error(`获取 ${position.symbol} 监听数量失败:`, error);
            return {
              ...position,
              monitorCount: 0
            };
          }
        })
      );
      setPositionsWithMonitors(positionsWithCount);
    } catch (error) {
      console.error('获取持仓监听数量失败:', error);
      setPositionsWithMonitors(positionsData.map(pos => ({ ...pos, monitorCount: 0 })));
    }
  };

  // 获取当前价格
  const fetchCurrentPrice = async (symbol) => {
    try {
      const response = await api.get(`/monitor/orderbook/${symbol}`);
      const orderbook = response.data.data;
      if (orderbook?.bids && orderbook?.asks) {
        const bestBid = parseFloat(orderbook.bids[0]?.price || 0);
        const bestAsk = parseFloat(orderbook.asks[0]?.price || 0);
        return (bestBid + bestAsk) / 2;
      }
      return 0;
    } catch (error) {
      message.error('获取当前价格失败');
      return 0;
    }
  };

  // 统一的操作处理器
  const handleAction = async (position, action) => {
    setCurrentPosition(position);
    setActionType(action);
    
    const price = await fetchCurrentPrice(position.symbol);
    setCurrentPrice(price);
    
    const precision = await getCoinPrecision(position.symbol);
    setCoinPrecision(precision);
    
    const config = ACTIONS[action];
    const entryPrice = position.entry_price || price;
    const isLong = position.side === 'LONG';
    
    // 设置默认价格百分比
    let defaultPercentage = 0;
    if (action === 'take_profit') {
      defaultPercentage = isLong ? 5 : -5;
    } else if (action === 'stop_loss') {
      defaultPercentage = isLong ? -3 : 3;
    }
    
    // 计算目标价格
    const basePrice = config.priceBase === 'current' ? price : entryPrice;
    let defaultTargetPrice = basePrice * (1 + defaultPercentage / 100);
    
    // 应用精度修正
    if (precision) {
      const { price: adjustedPrice } = autoFixPriceAndQuantity(defaultTargetPrice, 0, precision);
      defaultTargetPrice = adjustedPrice;
    }
    
    // 设置默认数量（持仓数量的50%）
    let defaultQuantity = Math.abs(position.size) * 0.5;
    if (precision) {
      const { quantity: adjustedQuantity } = autoFixPriceAndQuantity(0, defaultQuantity, precision);
      defaultQuantity = adjustedQuantity;
    }
    
    // 计算USDT金额 - 根据操作类型区分
    let defaultUsdtAmount;
    if (action === 'add_position') {
      // 加仓：显示需要投入的USDT（保证金）
      defaultUsdtAmount = (defaultQuantity * defaultTargetPrice) / position.leverage;
    } else {
      // 止盈/止损：显示盈利的USDT，需要考虑方向
      let profitPerCoin;
      if (isLong) {
        // 多头：目标价格 - 开仓价格
        profitPerCoin = defaultTargetPrice - entryPrice;
      } else {
        // 空头：开仓价格 - 目标价格
        profitPerCoin = entryPrice - defaultTargetPrice;
      }
      defaultUsdtAmount = profitPerCoin * defaultQuantity;
    }
    
    setPricePercentage(defaultPercentage);
    setTargetPrice(defaultTargetPrice);
    setQuantity(defaultQuantity);
    setUsdtAmount(defaultUsdtAmount);
    setDrawerVisible(true);
  };

  // 价格滑块变化处理
  const handlePriceSliderChange = (percentage) => {
    setPricePercentage(percentage);
    
    const config = ACTIONS[actionType];
    const basePrice = config.priceBase === 'current' ? currentPrice : currentPosition.entry_price;
    let newTargetPrice = basePrice * (1 + percentage / 100);
    
    if (coinPrecision) {
      const { price: adjustedPrice } = autoFixPriceAndQuantity(newTargetPrice, 0, coinPrecision);
      newTargetPrice = adjustedPrice;
    }
    
    setTargetPrice(newTargetPrice);
    
    // 对于加仓，价格变化时需要调整数量以保持在最大范围内
    if (actionType === 'add_position') {
      const maxUsdtAmount = accountValue.usdt_free * 0.6;
      const newMaxQuantity = (maxUsdtAmount * currentPosition.leverage) / newTargetPrice;
      
      // 如果当前数量超过了新的最大数量，调整到最大数量
      if (quantity > newMaxQuantity) {
        const adjustedQuantity = newMaxQuantity;
        setQuantity(adjustedQuantity);
        
        // 重新计算USDT金额
        const newUsdtAmount = (adjustedQuantity * newTargetPrice) / currentPosition.leverage;
        setUsdtAmount(newUsdtAmount);
      } else {
        // 重新计算USDT金额
        const newUsdtAmount = (quantity * newTargetPrice) / currentPosition.leverage;
        setUsdtAmount(newUsdtAmount);
      }
    } else {
      // 止盈/止损：显示盈利的USDT，需要考虑方向
      const entryPrice = currentPosition.entry_price;
      const isLong = currentPosition.side === 'LONG';
      let profitPerCoin;
      if (isLong) {
        // 多头：目标价格 - 开仓价格
        profitPerCoin = newTargetPrice - entryPrice;
      } else {
        // 空头：开仓价格 - 目标价格
        profitPerCoin = entryPrice - newTargetPrice;
      }
      const newUsdtAmount = profitPerCoin * quantity;
      setUsdtAmount(newUsdtAmount);
    }
  };

  // 数量滑块变化处理
  const handleQuantitySliderChange = (newQuantity) => {
    let finalQuantity = newQuantity;
    
    if (coinPrecision) {
      const { quantity: adjustedQuantity } = autoFixPriceAndQuantity(0, newQuantity, coinPrecision);
      finalQuantity = adjustedQuantity;
    }
    
    setQuantity(finalQuantity);
    
    // 重新计算USDT金额 - 根据操作类型区分
    let newUsdtAmount;
    if (actionType === 'add_position') {
      // 加仓：显示需要投入的USDT
      newUsdtAmount = (finalQuantity * targetPrice) / currentPosition.leverage;
    } else {
      // 止盈/止损：显示盈利的USDT，需要考虑方向
      const entryPrice = currentPosition.entry_price;
      const isLong = currentPosition.side === 'LONG';
      let profitPerCoin;
      if (isLong) {
        // 多头：目标价格 - 开仓价格
        profitPerCoin = targetPrice - entryPrice;
      } else {
        // 空头：开仓价格 - 目标价格
        profitPerCoin = entryPrice - targetPrice;
      }
      newUsdtAmount = profitPerCoin * finalQuantity;
    }
    setUsdtAmount(newUsdtAmount);
  };

  // 获取最大数量
  const getMaxQuantity = () => {
    if (!currentPosition) return 1;
    
    const positionSize = Math.abs(currentPosition.size);
    if (actionType === 'add_position') {
      // 加仓：基于可用余额和目标价格计算，随价格变化
      const maxUsdtAmount = accountValue.usdt_free * 0.8; // 使用80%的可用余额
      const priceToUse = targetPrice > 0 ? targetPrice : currentPrice; // 使用目标价格
      if (priceToUse > 0 && currentPosition.leverage > 0) {
        // 最大数量 = (最大USDT × 杠杆) ÷ 目标价格
        const maxQuantity = (maxUsdtAmount * currentPosition.leverage) / priceToUse;
        return parseFloat(maxQuantity.toFixed(6));
      }
      return positionSize;
    } else {
      // 止盈/止损：最多平掉所有仓位
      return positionSize;
    }
  };

  // 确认操作
  const handleConfirm = async () => {
    try {
      // 在提交前再次修正精度
      let finalPrice = targetPrice;
      let finalQuantity = quantity;
      
      if (coinPrecision) {
        const { price: adjustedPrice, quantity: adjustedQuantity } = autoFixPriceAndQuantity(targetPrice, quantity, coinPrecision);
        finalPrice = adjustedPrice;
        finalQuantity = adjustedQuantity;
        
        // 验证精度
        const priceValidation = validatePriceComplete(finalPrice, coinPrecision);
        if (!priceValidation.valid) {
          message.error(`价格精度错误: ${priceValidation.error}`);
          return;
        }
        
        const quantityValidation = validateQuantityComplete(finalQuantity, coinPrecision);
        if (!quantityValidation.valid) {
          message.error(`数量精度错误: ${quantityValidation.error}`);
          return;
        }
      }

      const orderData = {
        symbol: currentPosition.symbol,
        side: currentPosition.side.toLowerCase(),
        action_type: actionType === 'add_position' ? 'open' : 'close',
        target_price: finalPrice,
        quantity: finalQuantity,
        leverage: currentPosition.leverage,
        margin_mode: currentPosition.margin_mode || 'isolated',
        order_type: 'limit',
        trigger_type: 'condition',
        created_by: actionType
      };

      await api.post('/estimates', orderData);
      
      message.success(`${ACTIONS[actionType].title}监听已创建`);
      setDrawerVisible(false);
      
      // 刷新持仓监听数量
      fetchPositionsWithMonitors(positions);
      
      // 如果详情抽屉是打开的，刷新详情
      if (detailDrawerVisible && detailPosition) {
        fetchPositionDetails(detailPosition);
      }
    } catch (error) {
      message.error('创建订单失败');
    }
  };

  // 获取仓位详情
  const fetchPositionDetails = async (position) => {
    setEstimatesLoading(true);
    try {
      const data = await api.get(`/estimates?symbol=${position.symbol}`);
      const estimatesData = data.data.data || [];
      const filteredEstimates = estimatesData.filter(estimate => 
        estimate.side === position.side.toLowerCase() && estimate.status === 'listening'
      );
      
      // 获取当前价格并计算差异
      const currentPrice = await fetchCurrentPrice(position.symbol);
      
      const estimatesWithPrice = filteredEstimates.map(estimate => {
        const priceDiff = currentPrice > 0 ? 
          ((estimate.target_price - currentPrice) / currentPrice * 100) : 0;
        
        return {
          ...estimate,
          current_price: currentPrice,
          price_difference: priceDiff,
          is_close_to_trigger: Math.abs(priceDiff) <= 2
        };
      });
      
      setPositionEstimates(estimatesWithPrice);
    } catch (error) {
      message.error('获取仓位详情失败');
      setPositionEstimates([]);
    } finally {
      setEstimatesLoading(false);
    }
  };

  const openDetailDrawer = (position) => {
    setDetailPosition(position);
    setDetailDrawerVisible(true);
    fetchPositionDetails(position);
  };

  const closeDetailDrawer = () => {
    setDetailDrawerVisible(false);
    setDetailPosition(null);
    setPositionEstimates([]);
  };

  const handleDeleteEstimate = async (estimateId) => {
    const success = await deleteEstimate(estimateId);
    if (success) {
      // 刷新持仓监听数量
      fetchPositionsWithMonitors(positions);
      
      // 如果详情抽屉是打开的，刷新详情
      if (detailPosition) {
        fetchPositionDetails(detailPosition);
      }
    }
  };

  const handleToggleEstimate = async (estimateId, enabled) => {
    try {
      await toggleEstimateEnabled(estimateId, enabled);
      message.success(`监听已${enabled ? '开启' : '关闭'}`);
      
      // 刷新持仓监听数量
      fetchPositionsWithMonitors(positions);
      
      // 如果详情抽屉是打开的，刷新详情
      if (detailPosition) {
        fetchPositionDetails(detailPosition);
      }
    } catch (error) {
      message.error('切换监听状态失败');
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
          await Promise.all([
            fetchPositions(),
            fetchAccountInfo()
          ]);
          // 刷新后重新获取监听数量会通过useEffect自动触发
        } catch (error) {
          console.error('刷新失败:', error);
        } finally {
          setRefreshing(false);
        }
      },
      children: '刷新'
    }
  ];

  if (positionsLoading) {
    return <Spin size="large" style={{ display: 'block', textAlign: 'center', padding: '50px' }} />;
  }

  return (
    <div>
      <PageHeader 
        title="持仓管理" 
        actions={headerActions}
      />

      {positionsWithMonitors.length === 0 ? (
        <Empty description="暂无持仓数据" />
      ) : (
        <Row gutter={[16, 16]}>
          {positionsWithMonitors.map((position) => (
            <Col 
              xs={24} 
              sm={12} 
              md={8} 
              lg={6} 
              xl={4} 
              key={`${position.symbol}_${position.side}`}
            >
              <PositionCard
                position={position}
                onAction={handleAction}
                onViewDetails={openDetailDrawer}
              />
            </Col>
          ))}
        </Row>
      )}

      {/* 操作抽屉 */}
      <Drawer
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={{ fontSize: '16px', fontWeight: 600 }}>
              {ACTIONS[actionType]?.title}
            </span>
            {currentPosition && (
              <>
                <span style={{ fontSize: '18px', fontWeight: 700, color: '#1890ff' }}>
                  {currentPosition.symbol}
                </span>
                <span style={{ 
                  background: currentPosition.side === 'LONG' ? '#f6ffed' : '#fff2f0',
                  color: currentPosition.side === 'LONG' ? '#52c41a' : '#ff4d4f',
                  padding: '4px 8px',
                  borderRadius: '6px',
                  fontSize: '12px',
                  fontWeight: 600,
                  border: `1px solid ${currentPosition.side === 'LONG' ? '#b7eb8f' : '#ffb3b3'}`
                }}>
                  {currentPosition.side === 'LONG' ? '多头' : '空头'}
                </span>
                <span style={{ 
                  background: '#f0f9ff',
                  color: '#1890ff',
                  padding: '4px 8px',
                  borderRadius: '6px',
                  fontSize: '12px',
                  fontWeight: 600,
                  border: '1px solid #bae6fd'
                }}>
                  {currentPosition.leverage}×
                </span>
              </>
            )}
          </div>
        }
        placement="right"
        onClose={() => setDrawerVisible(false)}
        open={drawerVisible}
        width={window.innerWidth < 768 ? '100%' : 480}
        extra={
          <button 
            className={`drawer-confirm-btn ${
              actionType === 'add_position' ? 'primary' : 
              actionType === 'take_profit' ? 'success' : 'danger'
            }`}
            onClick={handleConfirm}
          >
            确认{ACTIONS[actionType]?.title}
          </button>
        }
      >
        {currentPosition && (
          <div>
            {/* 仓位信息卡片 */}
            <div style={{ 
              background: '#f8fafc', 
              padding: 16, 
              borderRadius: 8,
              marginBottom: 24,
              border: '1px solid #e2e8f0'
            }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                <div>
                  <Text type="secondary" style={{ fontSize: '12px', display: 'block' }}>当前价格</Text>
                  <Text strong style={{ fontSize: '14px', color: '#10b981' }}>
                    ${currentPrice.toFixed(4)}
                  </Text>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: '12px', display: 'block' }}>开仓价格</Text>
                  <Text strong style={{ fontSize: '14px', color: '#374151' }}>
                    ${currentPosition.entry_price.toFixed(4)}
                  </Text>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: '12px', display: 'block' }}>持仓数量</Text>
                  <Text strong style={{ fontSize: '14px', color: '#374151' }}>
                    {Math.abs(currentPosition.size).toFixed(4)}
                  </Text>
                </div>
                <div>
                  <Text type="secondary" style={{ fontSize: '12px', display: 'block' }}>未实现盈亏</Text>
                  <Text strong style={{ 
                    fontSize: '14px', 
                    color: currentPosition.unrealized_pnl >= 0 ? '#10b981' : '#ef4444' 
                  }}>
                    {currentPosition.unrealized_pnl >= 0 ? '+' : ''}{currentPosition.unrealized_pnl.toFixed(2)} USDT
                  </Text>
                </div>
              </div>
            </div>

            {/* 价格滑块 */}
            <PriceSlider
              action={actionType}
              currentPrice={currentPrice}
              entryPrice={currentPosition.entry_price}
              percentage={pricePercentage}
              onPercentageChange={handlePriceSliderChange}
              targetPrice={targetPrice}
              precision={coinPrecision}
              config={ACTIONS[actionType]}
            />

            {/* 数量滑块 */}
            <QuantitySlider
              action={actionType}
              quantity={quantity}
              maxQuantity={getMaxQuantity()}
              onQuantityChange={handleQuantitySliderChange}
              precision={coinPrecision}
              symbol={currentPosition.symbol}
              config={ACTIONS[actionType]}
            />

            {/* 金额显示 */}
            <div style={{ 
              background: '#f0f9ff', 
              padding: 16, 
              borderRadius: 8,
              marginTop: 24,
              border: '1px solid #bae6fd'
            }}>
              {actionType === 'add_position' ? (
                <>
                  <Text strong>预计使用: {usdtAmount.toFixed(2)} USDT</Text>
                  <br />
                  <Text type="secondary">
                    可用余额: {accountValue.usdt_free.toFixed(2)} USDT
                  </Text>
                </>
              ) : (
                <>
                  <Text strong>预计盈利: {usdtAmount.toFixed(2)} USDT</Text>
                  <br />
                  <Text type="secondary">
                    {(() => {
                      const margin = (currentPosition.entry_price * Math.abs(currentPosition.size)) / currentPosition.leverage;
                      const profitPercentage = margin > 0 ? (usdtAmount / margin * 100) : 0;
                      return `基于保证金: ${profitPercentage.toFixed(2)}%`;
                    })()}
                  </Text>
                </>
              )}
            </div>
          </div>
        )}
      </Drawer>

      {/* 详情抽屉 */}
      <Drawer
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={{ fontSize: '16px', fontWeight: 600, color: '#1f2937' }}>
              {detailPosition?.symbol}
            </span>
            <span style={{ 
              background: detailPosition?.side === 'LONG' ? '#f0fdf4' : '#fef2f2',
              color: detailPosition?.side === 'LONG' ? '#166534' : '#991b1b',
              padding: '3px 8px',
              borderRadius: '4px',
              fontSize: '10px',
              fontWeight: 600
            }}>
              {detailPosition?.side === 'LONG' ? '多头' : '空头'}
            </span>
            <span style={{ 
              background: '#f9fafb',
              color: '#6b7280',
              padding: '2px 6px',
              borderRadius: '4px',
              fontSize: '10px',
              fontWeight: 500
            }}>
              {positionEstimates.length}个监听
            </span>
          </div>
        }
        placement="right"
        onClose={closeDetailDrawer}
        open={detailDrawerVisible}
        width={window.innerWidth < 768 ? '100%' : 500}
        styles={{
          body: {
            paddingTop: 0,
            height: '100%'
          }
        }}
      >
        {detailPosition && (
          <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
            <div className="monitoring-list-container" style={{ paddingTop: '16px' }}>
              {estimatesLoading ? (
                <Spin style={{ display: 'block', textAlign: 'center', margin: '20px 0' }} />
              ) : positionEstimates.length === 0 ? (
                <Empty 
                  description="暂无价格监听" 
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  style={{ margin: '20px 0' }}
                />
              ) : (
                <div className="monitoring-list-content">
                  {positionEstimates.map((estimate) => (
                    <MonitoringCard
                      key={estimate.id}
                      estimate={estimate}
                      onDelete={handleDeleteEstimate}
                      onToggle={handleToggleEstimate}
                    />
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
};

export default Positions;
