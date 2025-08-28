import React from 'react';
import { Switch } from 'antd';
import { getDetailedActionText } from '../../utils/constants';

/**
 * 监听卡片组件
 * @param {Object} estimate - 监听数据
 * @param {Object} currentPosition - 当前持仓信息
 * @param {Function} onDelete - 删除回调
 * @param {Function} onToggle - 监听开关回调
 */
const MonitoringCard = ({ estimate, currentPosition, onDelete, onToggle }) => {
  // 使用详细的操作类型文本
  const actionText = getDetailedActionText(estimate.action_type, estimate.side);

  // 判断是否为基于仓位的操作
  const isPositionBasedAction = ['take_profit', 'stop_loss', 'close'].includes(estimate.action_type);
  
  // 计算预估盈利/止损
  const calculateEstimatedPnL = () => {
    // 开仓操作不计算基于仓位的盈亏
    if (!isPositionBasedAction) {
      return 0;
    }
    
    if (!currentPosition || !estimate.quantity || !estimate.target_price || !currentPosition.entry_price) {
      return 0;
    }

    const isLong = currentPosition.side === 'LONG';
    const priceDiff = isLong 
      ? (estimate.target_price - currentPosition.entry_price)
      : (currentPosition.entry_price - estimate.target_price);
    
    return priceDiff * estimate.quantity;
  };

  // 计算仓位占比
  const calculatePositionRatio = () => {
    // 开仓操作不计算仓位占比
    if (!isPositionBasedAction) {
      return null;
    }
    
    if (!currentPosition || !currentPosition.size || currentPosition.size === 0) {
      return null;
    }
    
    return ((estimate.quantity / Math.abs(currentPosition.size)) * 100).toFixed(1);
  };

  const estimatedPnL = calculateEstimatedPnL();
  const isProfit = estimatedPnL >= 0;
  const positionRatio = calculatePositionRatio();

  // 计算交易金额
  const calculateTradeAmount = () => {
    if (!estimate.quantity) return 0;
    
    let basePrice = 0;
    
    // 对于基于仓位的操作，使用开仓价格
    if (isPositionBasedAction && currentPosition && currentPosition.entry_price) {
      basePrice = currentPosition.entry_price;
    } 
    // 对于开仓操作，使用目标价格
    else if (estimate.target_price) {
      basePrice = estimate.target_price;
    }
    
    if (!basePrice) return 0;
    
    // 计算保证金金额：价格 × 数量 ÷ 杠杆
    if (estimate.leverage && estimate.leverage > 0) {
      return (estimate.quantity * basePrice) / estimate.leverage;
    }
    
    // 如果没有杠杆信息，返回名义金额
    return estimate.quantity * basePrice;
  };

  const tradeAmount = calculateTradeAmount();

  // 获取币种基础名称
  const getBaseAsset = (symbol) => {
    if (!symbol) return '';
    const baseAsset = symbol.replace('USDT', '');
    return baseAsset.length > 8 ? baseAsset.substring(0, 8) + '...' : baseAsset;
  };

  // 格式化数字显示
  return (
    <div className="monitoring-card-clean">
      {/* 头部信息 */}
      <div className="monitoring-header-clean">
        <div className="monitoring-info-row">
          <span className={`monitoring-action-tag ${estimate.action_type} ${estimate.side}`}>
            {actionText}
          </span>
          <span className="monitoring-order-type">{estimate.order_type}</span>
        </div>
        <div className="monitoring-controls">
          <div className="monitor-toggle-switch">
            <Switch
              size="small"
              checked={estimate.enabled}
              onChange={(checked) => onToggle && onToggle(estimate.id, checked)}
              title={estimate.enabled ? "关闭监听" : "开启监听"}
            />
          </div>
          <button 
            className="monitoring-delete-btn"
            onClick={() => onDelete(estimate.id)}
            title="删除监听"
          >
            ×
          </button>
        </div>
      </div>

      {/* 价格核心信息 */}
      <div className="monitoring-price-section">
        <div className="price-row main-prices">
          <div className="price-item">
            <span className="price-label">当前</span>
            <span className="price-value current">
              {estimate.current_price && estimate.current_price > 0 
                ? `$${estimate.current_price.toFixed(4)}`
                : "加载中..."
              }
            </span>
          </div>
          <div className="price-item">
            <span className="price-label">目标</span>
            <span className="price-value target">
              ${estimate.target_price.toFixed(4)}
            </span>
          </div>
        </div>

        {/* 价格变化百分比 */}
        <div className="price-change-indicator">
          {estimate.current_price && estimate.current_price > 0 ? (
            <>
              <span className={`price-change-value ${
                estimate.price_difference >= 0 ? 'positive' : 'negative'
              }`}>
                {estimate.price_difference >= 0 ? '+' : ''}{estimate.price_difference.toFixed(2)}%
              </span>
              {estimate.is_close_to_trigger && (
                <span className="trigger-warning">接近触发</span>
              )}
            </>
          ) : (
            <span className="price-change-value loading">等待价格数据...</span>
          )}
        </div>
      </div>

      {/* 交易详情 */}
      <div className="monitoring-details">
        <div className="detail-row">
          <span className="detail-label">交易数量</span>
          <span className="detail-value">
            {estimate.quantity?.toFixed(6)} {getBaseAsset(estimate.symbol)}
          </span>
        </div>
        <div className="detail-row">
          <span className="detail-label">交易金额</span>
          <span className="detail-value">
            {tradeAmount > 0 ? `${tradeAmount.toFixed(2)} USDT` : 'N/A'}
          </span>
        </div>
        {/* 仓位占比 - 只对基于仓位的操作显示 */}
        {isPositionBasedAction && (
          <div className="detail-row">
            <span className="detail-label">仓位占比</span>
            <span className="detail-value">
              {positionRatio ? `${positionRatio}%` : 'N/A'}
            </span>
          </div>
        )}
        
        {/* 预估盈亏 - 只对基于仓位的操作显示 */}
        {isPositionBasedAction && (
          <div className="detail-row">
            <span className="detail-label">预估盈亏</span>
            <span className={`detail-value ${isProfit ? 'profit' : 'loss'}`}>
              {estimatedPnL !== 0 
                ? `${isProfit ? '+' : ''}${estimatedPnL.toFixed(2)} USDT`
                : 'N/A'
              }
            </span>
          </div>
        )}
        <div className="detail-row">
          <span className="detail-label">创建时间</span>
          <span className="detail-value time">
            {new Date(estimate.created_at).toLocaleString()}
          </span>
        </div>
      </div>
    </div>
  );
};

export default MonitoringCard;
