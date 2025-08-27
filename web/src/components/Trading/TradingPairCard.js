import React from 'react';
import { Tag } from 'antd';
import { LineChartOutlined } from '@ant-design/icons';

/**
 * 交易对卡片组件
 * @param {Object} pair - 交易对信息
 * @param {Object} priceInfo - 价格信息
 * @param {Function} onAction - 操作回调 (symbol, action)
 * @param {Function} hasPosition - 检查是否有仓位
 * @param {Function} hasAnyPosition - 检查是否有任意仓位
 * @param {Function} hasAnyEstimate - 检查是否有监听
 * @param {Object} symbolEstimates - 监听数量映射
 * @param {Function} canDeleteSymbol - 检查是否可删除
 * @param {Function} getDeleteDisabledReason - 获取删除禁用原因
 * @param {boolean} isMobile - 是否为移动端
 */
const TradingPairCard = ({ 
  pair, 
  priceInfo, 
  onAction, 
  hasPosition,
  hasAnyPosition,
  hasAnyEstimate,
  symbolEstimates,
  canDeleteSymbol,
  getDeleteDisabledReason,
  isMobile = false
}) => {
  const symbol = pair.symbol;
  const hasValidPrice = priceInfo && priceInfo.markPrice > 0;
  
  // 格式化价格显示
  const formatPrice = (price) => {
    if (!price) return 'N/A';
    const numPrice = Number(price) || 0;
    if (numPrice >= 1000) return numPrice.toFixed(2);
    if (numPrice >= 1) return numPrice.toFixed(4);
    return numPrice.toFixed(6);
  };

  // 格式化百分比
  const formatPercent = (percent) => {
    if (!percent) return null;
    const value = parseFloat(percent) || 0;
    return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;
  };

  return (
    <div className="trading-pair-card-clean">
      {/* 头部信息 */}
      <div className="trading-header-clean">
        <div className="trading-info-row">
          <span className="trading-symbol-clean">{symbol}</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', marginLeft: 'auto' }}>
            {hasAnyEstimate(symbol) && (
              <Tag size="small" color="blue" className="estimate-badge">
                监听 {(() => {
                  const estimates = symbolEstimates[symbol];
                  if (Array.isArray(estimates)) {
                    return estimates.length;
                  } else if (typeof estimates === 'number') {
                    return estimates;
                  }
                  return '';
                })()}
              </Tag>
            )}
            <button
              className="control-btn primary-btn kline-btn-icon"
              onClick={() => onAction(symbol, 'kline')}
              title="查看K线图"
              style={{
                padding: '0',
                fontSize: '12px',
                height: '24px',
                width: '24px',
                minWidth: '24px',
                minHeight: '24px',
                maxWidth: '24px',
                maxHeight: '24px',
                borderRadius: '50%',
                background: '#1890ff',
                border: '1px solid #1890ff',
                color: 'white',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 1px 2px rgba(0,0,0,0.1)',
                boxSizing: 'border-box',
                flexShrink: 0
              }}
            >
              <LineChartOutlined style={{ fontSize: '13px' }} />
            </button>
          </div>
        </div>
      </div>

      {/* 价格信息区域 */}
      <div className="trading-price-section">
        <div className="price-info-section">
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {/* 标记价格 */}
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: '18px', fontWeight: '700', color: '#1f2937', marginBottom: '4px' }}>
                ${formatPrice(priceInfo?.markPrice)}
              </div>
              {priceInfo?.priceChangePercent && (
                <div style={{ 
                  fontSize: '13px', 
                  fontWeight: '600',
                  color: parseFloat(priceInfo.priceChangePercent) >= 0 ? '#059669' : '#dc2626'
                }}>
                  {formatPercent(priceInfo.priceChangePercent)}
                </div>
              )}
            </div>
            
            {/* 资金费率 */}
            {!isMobile && priceInfo?.fundingRate !== undefined && (
              <div style={{ 
                textAlign: 'center',
                fontSize: '11px',
                color: '#6b7280'
              }}>
                <span>资金费率: </span>
                <span style={{ 
                  color: priceInfo.fundingRate >= 0 ? '#059669' : '#dc2626',
                  fontWeight: '600'
                }}>
                  {(priceInfo.fundingRate).toFixed(4)}%
                </span>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 操作按钮 */}
      <div className="trading-controls">
        <div className="trading-control-group">
          {hasValidPrice && (
            <>
              <button
                className={`control-btn ${hasPosition(symbol, 'long') ? 'secondary-btn' : 'success-btn'} trading-control-btn`}
                disabled={hasPosition(symbol, 'long')}
                onClick={() => onAction(symbol, 'long')}
              >
                {hasPosition(symbol, 'long') ? '已开多' : '开多'}
              </button>
              <button
                className={`control-btn ${hasPosition(symbol, 'short') ? 'secondary-btn' : 'danger-btn'} trading-control-btn`}
                disabled={hasPosition(symbol, 'short')}
                onClick={() => onAction(symbol, 'short')}
              >
                {hasPosition(symbol, 'short') ? '已开空' : '开空'}
              </button>
            </>
          )}
        </div>
        <button
          className={`control-btn ${!canDeleteSymbol(symbol) ? 'secondary-btn' : 'danger-btn'} trading-control-btn`}
          disabled={!canDeleteSymbol(symbol)}
          onClick={() => onAction(symbol, 'delete')}
          title={!canDeleteSymbol(symbol) ? getDeleteDisabledReason(symbol) : '删除交易对'}
          style={{ marginTop: '4px', fontSize: '11px', width: '100%' }}
        >
          删除
        </button>
      </div>
    </div>
  );
};

export default TradingPairCard;