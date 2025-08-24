import React from 'react';
import PriceDisplay from '../Common/PriceDisplay';

/**
 * 交易对卡片组件
 * @param {Object} pair - 交易对信息
 * @param {Object} priceData - 价格数据
 * @param {Object} positions - 持仓信息
 * @param {Object} symbolEstimates - 监听统计
 * @param {Function} onDelete - 删除回调
 * @param {Function} onTrade - 交易回调
 * @param {Function} hasPosition - 检查仓位函数
 * @param {Function} canDeleteSymbol - 检查是否可删除函数
 * @param {Function} getDeleteDisabledReason - 获取删除禁用原因函数
 * @param {Function} hasAnyEstimate - 检查是否有监听函数
 */
const TradingPairCard = ({
  pair,
  priceData,
  positions,
  symbolEstimates,
  onDelete,
  onTrade,
  hasPosition,
  canDeleteSymbol,
  getDeleteDisabledReason,
  hasAnyEstimate
}) => {
  const price = priceData[pair.symbol];

  return (
    <div className="trading-pair-card-clean">
      {/* 头部信息 */}
      <div className="trading-header-clean">
        <div className="trading-info-row">
          <span className="trading-symbol-clean">{pair.symbol}</span>
          {hasAnyEstimate(pair.symbol) && (
            <span className="estimate-badge">
              {symbolEstimates[pair.symbol]}
            </span>
          )}
        </div>
        <button 
          className="delete-btn-clean"
          disabled={!canDeleteSymbol(pair.symbol)}
          onClick={() => onDelete(pair.symbol)}
          title={!canDeleteSymbol(pair.symbol) ? getDeleteDisabledReason(pair.symbol) : '删除交易对'}
        >
          ×
        </button>
      </div>

      {/* 价格信息 */}
      <div className="trading-price-section">
        <PriceDisplay 
          priceData={price}
          loading={!price}
          symbol={pair.symbol}
        />
      </div>

      {/* 交易按钮 */}
      {price && price.hasValidData && (
        <div className="trading-controls">
          <div className="trading-control-group">
            <button 
              className={`trading-control-btn long-btn ${hasPosition(pair.symbol, 'long') ? 'disabled' : ''}`}
              disabled={hasPosition(pair.symbol, 'long')}
              onClick={() => onTrade(pair.symbol, 'long')}
              title={hasPosition(pair.symbol, 'long') ? '已有多头仓位' : '开多头仓位'}
            >
              {hasPosition(pair.symbol, 'long') ? '已开多' : '开多'}
            </button>
            <button 
              className={`trading-control-btn short-btn ${hasPosition(pair.symbol, 'short') ? 'disabled' : ''}`}
              disabled={hasPosition(pair.symbol, 'short')}
              onClick={() => onTrade(pair.symbol, 'short')}
              title={hasPosition(pair.symbol, 'short') ? '已有空头仓位' : '开空头仓位'}
            >
              {hasPosition(pair.symbol, 'short') ? '已开空' : '开空'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

export default TradingPairCard;
