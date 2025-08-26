import React from 'react';
import { Switch } from 'antd';
import { ACTION_TYPE_TEXT } from '../../utils/constants';

/**
 * 监听卡片组件
 * @param {Object} estimate - 监听数据
 * @param {Object} currentPosition - 当前持仓信息
 * @param {Function} onDelete - 删除回调
 * @param {Function} onToggle - 监听开关回调
 */
const MonitoringCard = ({ estimate, currentPosition, onDelete, onToggle }) => {
  const actionText = ACTION_TYPE_TEXT[estimate.created_by] || estimate.created_by;

  // 计算预估盈利/止损
  const calculateEstimatedPnL = () => {
    if (!currentPosition || !estimate.quantity || !estimate.target_price || !currentPosition.entry_price) {
      return 0;
    }

    const isLong = currentPosition.side === 'LONG';
    const priceDiff = isLong 
      ? (estimate.target_price - currentPosition.entry_price)
      : (currentPosition.entry_price - estimate.target_price);
    
    return priceDiff * estimate.quantity;
  };

  const estimatedPnL = calculateEstimatedPnL();
  const isProfit = estimatedPnL >= 0;

  // 格式化数字显示
  return (
    <div className="monitoring-card-clean">
      {/* 头部信息 */}
      <div className="monitoring-header-clean">
        <div className="monitoring-info-row">
          <span className={`monitoring-action-tag ${estimate.created_by}`}>
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
          <span className="detail-value">{estimate.quantity?.toFixed(6)} {estimate.symbol?.replace('USDT', '') || ''}</span>
        </div>
        <div className="detail-row">
          <span className="detail-label">仓位占比</span>
          <span className="detail-value">
            {currentPosition && currentPosition.size > 0 
              ? `${((estimate.quantity / Math.abs(currentPosition.size)) * 100).toFixed(1)}%`
              : 'N/A'
            }
          </span>
        </div>
        <div className="detail-row">
          <span className="detail-label">预估盈亏</span>
          <span className={`detail-value ${isProfit ? 'profit' : 'loss'}`}>
            {estimatedPnL !== 0 
              ? `${isProfit ? '+' : ''}${estimatedPnL.toFixed(2)} USDT`
              : 'N/A'
            }
          </span>
        </div>
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
