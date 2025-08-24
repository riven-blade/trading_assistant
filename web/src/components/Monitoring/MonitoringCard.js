import React from 'react';
import { Switch } from 'antd';
import { ACTION_TYPE_TEXT } from '../../utils/constants';

/**
 * 监听卡片组件
 * @param {Object} estimate - 监听数据
 * @param {Function} onDelete - 删除回调
 * @param {Function} onToggle - 监听开关回调
 */
const MonitoringCard = ({ estimate, onDelete, onToggle }) => {
  const actionText = ACTION_TYPE_TEXT[estimate.created_by] || estimate.created_by;

  // 格式化数字显示
  const formatNumber = (num, decimals = 2) => {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toFixed(decimals);
  };

  const formatQuantity = (qty) => {
    const abs = Math.abs(qty);
    if (abs >= 1000) return formatNumber(abs, 0);
    if (abs >= 1) return abs.toFixed(3);
    return abs.toFixed(6);
  };

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
              ${(estimate.current_price || 0).toFixed(4)}
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
          <span className={`price-change-value ${
            estimate.price_difference >= 0 ? 'positive' : 'negative'
          }`}>
            {estimate.price_difference >= 0 ? '+' : ''}{estimate.price_difference.toFixed(2)}%
          </span>
          {estimate.is_close_to_trigger && (
            <span className="trigger-warning">接近触发</span>
          )}
        </div>
      </div>

      {/* 交易详情 */}
      <div className="monitoring-details">
        <div className="detail-row">
          <span className="detail-label">数量</span>
          <span className="detail-value">{formatQuantity(estimate.quantity)}</span>
        </div>
        <div className="detail-row">
          <span className="detail-label">保证金</span>
          <span className="detail-value">${estimate.usdt_amount.toFixed(2)}</span>
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
