import React, { useState, useEffect, useCallback } from 'react';
import { 
  Row, 
  Col, 
  Drawer, 
  Typography, 
  message,
  Spin,
  Empty
} from 'antd';
import api, { toggleEstimateEnabled } from '../services/api';
import { calculateUsdtAmount, calculatePnl } from '../utils/precision';

// 通用组件和Hooks
import PageHeader from '../components/Common/PageHeader';
import PositionCard from '../components/Position/PositionCard';
import PriceSlider from '../components/Trading/PriceSlider';
import QuantitySlider from '../components/Trading/QuantitySlider';
import MonitoringCard from '../components/Monitoring/MonitoringCard';
import useAccountData from '../hooks/useAccountData';
import useEstimates from '../hooks/useEstimates';
import usePriceData from '../hooks/usePriceData';
import { ACTIONS } from '../utils/constants';

const { Text } = Typography;

const Positions = () => {
  // 操作相关状态
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [currentPosition, setCurrentPosition] = useState(null);
  const [actionType, setActionType] = useState('');
  const [currentPrice, setCurrentPrice] = useState(0);
  const [targetPrice, setTargetPrice] = useState(0);
  const [quantity, setQuantity] = useState(0);
  const [pricePercentage, setPricePercentage] = useState(0);
  const [confirmLoading, setConfirmLoading] = useState(false);

  // 详情抽屉相关状态
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [detailPosition, setDetailPosition] = useState(null);
  const [positionEstimates, setPositionEstimates] = useState([]);
  const [estimatesLoading, setEstimatesLoading] = useState(false);

  // 使用自定义Hooks
  const { 
    accountValue, 
    positions, 
    loading: positionsLoading
  } = useAccountData();

  // 监听数量状态
  const [positionsWithMonitors, setPositionsWithMonitors] = useState([]);

  const { estimates, deleteEstimate, getEstimatesBySymbol } = useEstimates();

  // 使用全局价格数据
  const { 
    getPriceBySymbol,
    loading: priceDataLoading,
    priceData,
    hasPriceData
  } = usePriceData();



  // 获取所有持仓的监听数量
  const fetchPositionsWithMonitors = useCallback((positionsData) => {
    try {
      const positionsWithCount = positionsData.map((position) => {
        // 从全局estimates数据中过滤出该持仓相关的监听
        const filteredEstimates = getEstimatesBySymbol(position.symbol, 'listening').filter(estimate => 
          estimate.side === position.side.toLowerCase()
        );
        
        return {
          ...position,
          monitorCount: filteredEstimates.length
        };
      });
      
      setPositionsWithMonitors(positionsWithCount);
    } catch (error) {
      console.error('计算持仓监听数量失败:', error);
      setPositionsWithMonitors(positionsData.map(pos => ({ ...pos, monitorCount: 0 })));
    }
  }, [getEstimatesBySymbol]);

  // 获取当前价格 - 使用全局价格数据
  const getCurrentPrice = useCallback((symbol) => {
    // 如果价格数据还在加载中，返回0
    if (priceDataLoading) {
      return 0;
    }
    
    // 检查是否有该币种的价格数据
    if (!hasPriceData(symbol)) {
      console.warn(`[价格] ${symbol} 的价格数据不可用`);
      return 0;
    }
    
    const priceInfo = getPriceBySymbol(symbol);
    const price = priceInfo?.markPrice || priceInfo?.currentPrice || 0;
    
    return price;
  }, [getPriceBySymbol, priceDataLoading, hasPriceData]);

  // 统一的操作处理器
  const handleAction = async (position, action) => {
    // 验证参数
    if (!position) {
      console.error('handleAction: position 参数不能为空');
      return;
    }
    
    if (!action || typeof action !== 'string') {
      console.error('handleAction: action 参数无效:', action);
      return;
    }
    
    const config = ACTIONS[action];
    if (!config) {
      console.error('handleAction: 未知的操作类型:', action);
      return;
    }
    
    setCurrentPosition(position);
    setActionType(action);
    
    const price = getCurrentPrice(position.symbol);
    setCurrentPrice(price);
    
    const entryPrice = position.entry_price || price;
    const isLong = position.side === 'LONG';
    
    // 设置默认价格百分比
    let defaultPercentage = 0;
    if (action === 'take_profit') {
      defaultPercentage = isLong ? 5 : -5;
    } else if (action === 'stop_loss') {
      defaultPercentage = isLong ? -3 : 3;
    }
    
    // 计算目标价格，确保 config 和 config.priceBase 存在
    const priceBase = config.priceBase || 'current';
    const basePrice = priceBase === 'current' ? price : entryPrice;
    let defaultTargetPrice = basePrice * (1 + defaultPercentage / 100);

    // 设置默认数量（持仓数量的50%）
    let defaultQuantity = Math.abs(position.size) * 0.5;
    
    setPricePercentage(defaultPercentage);
    setTargetPrice(defaultTargetPrice);
    setQuantity(defaultQuantity);
    setDrawerVisible(true);
  };

  // 价格滑块变化处理
  const handlePriceSliderChange = (percentage) => {
    setPricePercentage(percentage);
    
    const config = ACTIONS[actionType];
    if (!config) {
      console.error('handlePriceSliderChange: 未知的操作类型:', actionType);
      return;
    }
    
    const priceBase = config.priceBase || 'current';
    const basePrice = priceBase === 'current' ? currentPrice : currentPosition.entry_price;
    let newTargetPrice = basePrice * (1 + percentage / 100);
    
    setTargetPrice(newTargetPrice);
    
    // 对于加仓，价格变化时需要调整数量以保持在最大范围内
    if (actionType === 'addition') {
      const maxUsdtAmount = accountValue.usdt_free * 0.6;
      const newMaxQuantity = (maxUsdtAmount * currentPosition.leverage) / newTargetPrice;
      
      // 如果当前数量超过了新的最大数量，调整到最大数量
      if (quantity > newMaxQuantity) {
        const adjustedQuantity = newMaxQuantity;
        setQuantity(adjustedQuantity);
      }
    }
  };

  // 数量滑块变化处理
  const handleQuantitySliderChange = (newQuantity) => {
    let finalQuantity = newQuantity;
    setQuantity(finalQuantity);
  };

  // 获取最大数量
  const getMaxQuantity = () => {
    if (!currentPosition) return 1;
    
    const positionSize = Math.abs(currentPosition.size);
    if (actionType === 'addition') {
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
    if (confirmLoading) return;
    
    setConfirmLoading(true);
    try {
      const orderData = {
        symbol: currentPosition.symbol,
        side: currentPosition.side.toLowerCase(),
        action_type: actionType,
        target_price: targetPrice,
        quantity: quantity,
        leverage: currentPosition.leverage,
        margin_mode: currentPosition.margin_mode || 'isolated',
        order_type: 'limit',
        trigger_type: 'condition'
        // created_by字段已移除
      };

      await api.post('/estimates', orderData);
      
      message.success(`${ACTIONS[actionType].title}监听已创建`);
      setDrawerVisible(false);
      
      // 数据会通过全局estimates自动更新，无需手动刷新
    } catch (error) {
      console.error('创建订单失败:', error);
      message.error('创建订单失败');
    } finally {
      setConfirmLoading(false);
    }
  };

  // 获取仓位详情（从全局estimates数据中过滤）
  const fetchPositionDetails = useCallback((position) => {
    setEstimatesLoading(true);
    try {
      // 从全局estimates数据中过滤出该持仓相关的监听
      const filteredEstimates = getEstimatesBySymbol(position.symbol, 'listening').filter(estimate => 
        estimate.side === position.side.toLowerCase()
      );
      
      // 获取当前价格并计算差异
      const currentPrice = getCurrentPrice(position.symbol);
      
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
      console.error('获取仓位详情失败:', error);
      setPositionEstimates([]);
    } finally {
      setEstimatesLoading(false);
    }
  }, [getEstimatesBySymbol, getCurrentPrice]);

  // 监听positions变化，获取监听数量
  useEffect(() => {
    if (positions.length > 0) {
      fetchPositionsWithMonitors(positions);
    } else {
      setPositionsWithMonitors([]);
    }
  }, [positions, fetchPositionsWithMonitors]);

  // 监听estimates数据变化，实时更新持仓监听数量
  useEffect(() => {
    if (positions.length > 0) {
      fetchPositionsWithMonitors(positions);
    }
  }, [estimates, positions, fetchPositionsWithMonitors]);

  // 监听estimates数据变化，实时更新详情抽屉中的数据
  useEffect(() => {
    if (detailPosition && detailDrawerVisible) {
      fetchPositionDetails(detailPosition);
    }
  }, [estimates, detailPosition, detailDrawerVisible, fetchPositionDetails]);

  // 监听价格数据加载完成，更新详情抽屉中的价格
  useEffect(() => {
    if (detailPosition && detailDrawerVisible && !priceDataLoading && Object.keys(priceData).length > 0) {
      fetchPositionDetails(detailPosition);
    }
  }, [priceDataLoading, priceData, detailPosition, detailDrawerVisible, fetchPositionDetails]);

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
    await deleteEstimate(estimateId);
    // 数据会通过全局estimates自动更新，无需手动刷新
  };

  const handleToggleEstimate = async (estimateId, enabled) => {
    try {
      await toggleEstimateEnabled(estimateId, enabled);
      message.success(`监听已${enabled ? '开启' : '关闭'}`);
      
      // 数据会通过全局estimates自动更新，无需手动刷新
    } catch (error) {
      message.error('切换监听状态失败');
    }
  };

  // 页面操作配置
  const headerActions = [];

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
                currentPrice={getCurrentPrice(position.symbol)}
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
              actionType === 'addition' ? 'primary' : 
              actionType === 'take_profit' ? 'success' : 'danger'
            }`}
            onClick={handleConfirm}
            disabled={confirmLoading}
          >
            {confirmLoading ? '处理中...' : `确认${ACTIONS[actionType]?.title}`}
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
              config={ACTIONS[actionType]}
            />

            {/* 数量滑块 */}
            <QuantitySlider
              action={actionType}
              quantity={quantity}
              maxQuantity={getMaxQuantity()}
              onQuantityChange={handleQuantitySliderChange}
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
              {actionType === 'addition' ? (
                (() => {
                  const marginRequired = calculateUsdtAmount(quantity, targetPrice, currentPosition.leverage);
                  return (
                    <>
                      <Text strong>预计使用: {marginRequired.toFixed(2)} USDT</Text>
                      <br />
                      <Text type="secondary">
                        可用余额: {accountValue.usdt_free.toFixed(2)} USDT
                      </Text>
                    </>
                  );
                })()
              ) : (
                (() => {
                  const expectedPnl = calculatePnl(quantity, currentPosition.entry_price, targetPrice, currentPosition.side);
                  return (
                    <Text strong>预计盈亏: {expectedPnl.toFixed(2)} USDT</Text>
                  );
                })()
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
                      currentPosition={detailPosition}
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
