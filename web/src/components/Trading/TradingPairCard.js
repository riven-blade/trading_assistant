import React from 'react';
import { Tag } from 'antd';
import { LineChartOutlined, CloseOutlined } from '@ant-design/icons';

/**
 * 交易对卡片组件
 * @param {Object} pair - 交易对信息
 * @param {Object} priceInfo - 价格信息
 * @param {Function} onAction - 操作回调 (symbol, action)
 * @param {Function} hasPosition - 检查是否有仓位
 * @param {Function} hasAnyPosition - 检查是否有任意仓位
 * @param {Function} hasAnyEstimate - 检查是否有监听
 * @param {Function} hasOpenEstimate - 检查是否有开仓监听
 * @param {Object} symbolEstimates - 监听数量映射
 * @param {Function} canDeleteSymbol - 检查是否可删除
 * @param {Function} getDeleteDisabledReason - 获取删除禁用原因
 * @param {boolean} isMobile - 是否为移动端
 * @param {number} volumeRank - 成交额排名
 */
const TradingPairCard = ({ 
  pair, 
  priceInfo, 
  onAction, 
  hasPosition,
  hasAnyPosition,
  hasAnyEstimate,
  hasOpenEstimate,
  symbolEstimates,
  canDeleteSymbol,
  getDeleteDisabledReason,
  isMobile = false,
  volumeRank
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

  // 格式化交易额
  const formatVolume = (volume) => {
    if (!volume) return 'N/A';
    const numVolume = Number(volume) || 0;
    if (numVolume >= 1000000000) return `${(numVolume / 1000000000).toFixed(1)}B`;
    if (numVolume >= 1000000) return `${(numVolume / 1000000).toFixed(1)}M`;
    if (numVolume >= 1000) return `${(numVolume / 1000).toFixed(1)}K`;
    return numVolume.toFixed(0);
  };

  // 判断是否可以删除：没有仓位且没有监听
  const canRemove = canDeleteSymbol(symbol);

  return (
    <div className="trading-pair-card-clean">
      {/* 头部信息 */}
      <div className="trading-header-clean">
        <div className="trading-info-row">
          <span className="trading-symbol-clean">
            {symbol.length > 8 ? symbol.substring(0, 8) + '...' : symbol}
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', marginLeft: 'auto' }}>
            {hasAnyEstimate(symbol) && (() => {
              const estimates = symbolEstimates[symbol];
              let displayText = '';
              let titleText = "查看监控详情";
              let tagColor = "blue";
              
              if (Array.isArray(estimates)) {
                const enabledCount = estimates.filter(estimate => estimate.enabled).length;
                const totalCount = estimates.length;
                displayText = `${enabledCount}/${totalCount}`;
                titleText = `启用监听: ${enabledCount}个，总监听: ${totalCount}个，点击查看详情`;
                
                // 如果有未启用的监听，显示黄色
                if (enabledCount < totalCount) {
                  tagColor = "gold";
                }
              } else if (typeof estimates === 'number') {
                displayText = estimates;
              }
              
              return (
                <Tag 
                  size="small" 
                  color={tagColor} 
                  className="estimate-badge" 
                  style={{ cursor: 'pointer' }}
                  onClick={() => onAction(symbol, 'monitor')}
                  title={titleText}
                >
                  监听 {displayText}
                </Tag>
              );
            })()}
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
            {/* 删除按钮 - 只在可以删除时显示 */}
            {canRemove && (
              <button
                className="control-btn primary-btn kline-btn-icon"
                onClick={() => onAction(symbol, 'delete')}
                title="取消选中"
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
                  background: '#fef2f2',
                  border: '1px solid #fecaca',
                  color: '#dc2626',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  boxShadow: '0 1px 2px rgba(0,0,0,0.1)',
                  boxSizing: 'border-box',
                  flexShrink: 0,
                  transition: 'all 0.15s ease'
                }}
              >
                <CloseOutlined style={{ fontSize: '12px' }} />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* 价格信息区域 */}
      <div className="trading-price-section">
        <div className="price-info-section">
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {/* 标记价格 */}
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: '18px', fontWeight: '700', color: '#1f2937' }}>
                ${formatPrice(priceInfo?.markPrice)}
              </div>
            </div>
            
            {/* 交易额显示 */}
            {pair.quote_volume && (
              <div style={{ 
                textAlign: 'center',
                fontSize: '11px',
                color: '#6b7280',
                marginBottom: '4px'
              }}>
                <span>成交额: </span>
                <span style={{ 
                  color: '#374151',
                  fontWeight: '600'
                }}>
                  ${formatVolume(pair.quote_volume)}
                </span>
                {volumeRank && (
                  <span style={{ 
                    marginLeft: '8px',
                    padding: '1px 4px',
                    backgroundColor: '#f3f4f6',
                    borderRadius: '4px',
                    fontSize: '10px',
                    color: '#6b7280',
                    fontWeight: '500'
                  }}>
                    #{volumeRank}
                  </span>
                )}
              </div>
            )}

            {/* 资金费率 - 移动端也显示 */}
            {priceInfo?.fundingRate !== undefined && (
              <div style={{ 
                textAlign: 'center',
                fontSize: isMobile ? '10px' : '11px',
                color: '#6b7280'
              }}>
                <span>资金费率: </span>
                <span style={{ 
                  color: priceInfo.fundingRate >= 0 ? '#059669' : '#dc2626',
                  fontWeight: '600'
                }}>
                  {(priceInfo.fundingRate * 100).toFixed(4)}%
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
                className={`control-btn ${(hasPosition(symbol, 'long') || hasOpenEstimate(symbol, 'long')) ? 'secondary-btn' : 'success-btn'} trading-control-btn`}
                disabled={hasPosition(symbol, 'long') || hasOpenEstimate(symbol, 'long')}
                onClick={() => onAction(symbol, 'long')}
              >
                {hasPosition(symbol, 'long') ? '已开多' : 
                 hasOpenEstimate(symbol, 'long') ? '监听中' : '开多'}
              </button>
              <button
                className={`control-btn ${(hasPosition(symbol, 'short') || hasOpenEstimate(symbol, 'short')) ? 'secondary-btn' : 'danger-btn'} trading-control-btn`}
                disabled={hasPosition(symbol, 'short') || hasOpenEstimate(symbol, 'short')}
                onClick={() => onAction(symbol, 'short')}
              >
                {hasPosition(symbol, 'short') ? '已开空' : 
                 hasOpenEstimate(symbol, 'short') ? '监听中' : '开空'}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default TradingPairCard;