import React from 'react';
import TradingSlider from './TradingSlider';

/**
 * 通用数量滑块组件
 * @param {string} action - 操作类型
 * @param {number} quantity - 当前数量
 * @param {number} maxQuantity - 最大数量
 * @param {Function} onQuantityChange - 数量变化回调
 * @param {string} symbol - 交易对符号
 * @param {Object} config - 操作配置
 */
const QuantitySlider = ({ 
  action, 
  quantity, 
  maxQuantity, 
  onQuantityChange, 
  symbol,
  config 
}) => {
  const baseAsset = symbol.replace('USDT', '');
  
  // 根据操作类型决定是否使用百分比模式
  const isPercentageMode = action === 'take_profit' || action === 'stop_loss';
  
  if (isPercentageMode) {
    const currentPercentage = maxQuantity > 0 ? (quantity / maxQuantity * 100) : 50;
    
    const handlePercentageChange = (percentage) => {
      const newQuantity = (maxQuantity * percentage) / 100;
      onQuantityChange(newQuantity);
    };

    return (
      <TradingSlider
        title="仓位占比"
        value={currentPercentage}
        min={0}
        max={100}
        step={1}
        onChange={handlePercentageChange}
        marks={{
          25: '25%',
          50: '50%',
          75: '75%'
        }}
        displayLabel="仓位占比:"
        displayValue={`${Math.round(currentPercentage)}%`}
        tooltipFormatter={(value) => `${Math.round(value)}%`}
        action={action}
      />
    );
  } else {
    // 数量模式：原有逻辑
    const optimizedMarks = {
      [maxQuantity * 0.1]: '10',
      [maxQuantity * 0.2]: '20',
      [maxQuantity * 0.3]: '30',
      [maxQuantity * 0.4]: '40',
      [maxQuantity * 0.5]: '50',
      [maxQuantity * 0.6]: '60',
      [maxQuantity * 0.7]: '70',
      [maxQuantity * 0.8]: '80',
      [maxQuantity * 0.9]: '90',
      [maxQuantity]: '100'
    };

    return (
      <TradingSlider
        title={config.quantityLabel || `${baseAsset}数量`}
        value={quantity}
        min={0.001}
        max={maxQuantity}
        step={0.001}
        onChange={onQuantityChange}
        marks={optimizedMarks}
        displayLabel="当前数量:"
        displayValue={`${quantity.toFixed(6)} ${baseAsset}`}
        tooltipFormatter={(value) => `${value?.toFixed(6)} ${baseAsset}`}
        action={action}
      />
    );
  }
};

export default QuantitySlider;
