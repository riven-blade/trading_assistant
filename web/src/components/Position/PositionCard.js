import React from 'react';

/**
 * 持仓卡片组件
 * @param {Object} position - 持仓信息
 * @param {number} currentPrice - 实时标记价格
 * @param {Function} onAction - 操作回调 (position, action)
 * @param {Function} onViewDetails - 查看详情回调
 */
const PositionCard = ({ position, currentPrice, onAction, onViewDetails }) => {
  // 使用实时价格计算盈亏
  const realTimeMarkPrice = currentPrice || position.mark_price;
  
  // 计算实时未实现盈亏
  const realTimeUnrealizedPnl = (() => {
    if (!realTimeMarkPrice || !position.entry_price || !position.size) return 0;
    
    // 确保所有值都是有效数字
    const markPrice = Number(realTimeMarkPrice) || 0;
    const entryPrice = Number(position.entry_price) || 0;
    const size = Number(position.size) || 0;
    
    if (markPrice === 0 || entryPrice === 0 || size === 0) return 0;
    
    const isLong = (position.side || 'LONG') === 'LONG';
    const priceDiff = isLong 
      ? (markPrice - entryPrice)
      : (entryPrice - markPrice);
    
    return priceDiff * size;
  })();
  
  const isProfit = (realTimeUnrealizedPnl || 0) >= 0;
  
  // 计算保证金 = 持仓价值 / 杠杆倍数
  const margin = (position.notional && position.leverage && position.leverage > 0) 
    ? position.notional / position.leverage 
    : 0;
  
  // 计算盈亏百分比 = 未实现盈亏 / 保证金 * 100%
  const pnlPercentage = (margin > 0 && !isNaN(realTimeUnrealizedPnl)) 
    ? (realTimeUnrealizedPnl / margin * 100) 
    : 0;

  // 格式化数字显示
  const formatSize = (size) => {
    if (size === undefined || size === null || isNaN(size)) return '0.000';
    const numSize = Number(size);
    const abs = Math.abs(numSize);
    if (abs >= 1000) return (abs / 1000).toFixed(2) + 'K';
    if (abs >= 1) return abs.toFixed(3);
    return abs.toFixed(6);
  };

  const formatPrice = (price) => {
    if (price === undefined || price === null || isNaN(price)) return '0.0000';
    const numPrice = Number(price);
    if (numPrice >= 1000) return numPrice.toFixed(2);
    if (numPrice >= 1) return numPrice.toFixed(4);
    return numPrice.toFixed(6);
  };

  const formatValue = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '0';
    const numValue = Number(value);
    const abs = Math.abs(numValue);
    if (abs >= 1000000) return (abs / 1000000).toFixed(1) + 'M';
    if (abs >= 1000) return (abs / 1000).toFixed(1) + 'K';
    return abs.toFixed(0);
  };

  return (
    <div className="position-card-clean">
      {/* 头部信息条 */}
      <div className="position-header-clean">
        <div className="position-info-row">
          <span className="position-symbol-clean">{position.symbol || 'N/A'}</span>
          <div className="position-badges">
            <span className={`side-badge ${(position.side || 'long').toLowerCase()}`}>
              {position.side?.toLowerCase() === 'long' ? 'L' : 'S'}
            </span>
            <span className="leverage-badge">{position.leverage || 1}×</span>
          </div>
        </div>
        <div className={`pnl-display ${isProfit ? 'positive' : 'negative'}`}>
          <div className="pnl-amount">
            {isProfit ? '+' : ''}{(realTimeUnrealizedPnl || 0).toFixed(2)}
          </div>
          <div className="pnl-percentage">
            {isProfit ? '+' : ''}{(pnlPercentage || 0).toFixed(2)}%
          </div>
        </div>
      </div>

      {/* 核心数据 */}
      <div className="position-data-section">
        <div className="data-row">
          <div className="data-group">
            <span className="data-label">仓位</span>
            <span className="data-value">{formatSize(position.size)}</span>
          </div>
          <div className="data-group">
            <span className="data-label">保证金</span>
            <span className="data-value">${formatValue(margin)}</span>
          </div>
        </div>
        <div className="data-row">
          <div className="data-group">
            <span className="data-label">开仓</span>
            <span className="data-value">${formatPrice(position.entry_price)}</span>
          </div>
          <div className="data-group">
            <span className="data-label">标记</span>
            <span className="data-value">${formatPrice(realTimeMarkPrice)}</span>
          </div>
        </div>
      </div>

      {/* 操作按钮 */}
      <div className="position-controls">
        <div className="control-group">
          <button 
            className="control-btn primary-btn"
            onClick={() => onAction(position, 'addition')}
          >
            加仓
          </button>
          <button 
            className="control-btn success-btn"
            onClick={() => onAction(position, 'take_profit')}
          >
            止盈
          </button>
          <button 
            className="control-btn danger-btn"
            onClick={() => onAction(position, 'stop_loss')}
          >
            止损
          </button>
        </div>
        <button 
          className="control-btn secondary-btn"
          onClick={() => onViewDetails(position)}
        >
          查看详情
          {position.monitorCount > 0 && (
            <span className="monitor-count-badge">
              {position.monitorCount}
            </span>
          )}
        </button>
      </div>
    </div>
  );
};

export default PositionCard;
