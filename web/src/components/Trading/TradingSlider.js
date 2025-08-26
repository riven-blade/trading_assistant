import React from 'react';
import { Slider } from 'antd';
import './TradingSlider.css';

/**
 * 交易滑块组件，按照设计图样式
 * @param {string} title - 滑块标题
 * @param {number} value - 当前值
 * @param {number} min - 最小值
 * @param {number} max - 最大值
 * @param {number} step - 步长
 * @param {Function} onChange - 值变化回调
 * @param {Object} marks - 滑块标记
 * @param {string} displayValue - 显示的值文本
 * @param {string} displayLabel - 显示的标签
 * @param {string} leftLabel - 左侧标签
 * @param {Function} tooltipFormatter - tooltip格式化函数
 * @param {boolean} disabled - 是否禁用
 * @param {string} action - 操作类型 (take_profit, stop_loss, addition, 等)
 */
const TradingSlider = ({
  title,
  value,
  min,
  max,
  step = 1,
  onChange,
  marks,
  displayValue,
  displayLabel,
  leftLabel,
  tooltipFormatter,
  disabled = false,
  action
}) => {
  // 根据操作类型确定显示标签
  const getPriceLabel = () => {
    // 如果有明确的displayLabel，直接使用
    if (displayLabel) {
      return displayLabel;
    }
    
    // 只有价格滑块才需要根据action显示不同标签
    if (action === 'take_profit') return '止盈价:';
    if (action === 'stop_loss') return '止损价:';
    if (action === 'addition') return '加仓价:';
    return '开仓价:'; // 默认开仓
  };
  return (
    <div className="trading-slider-container">
      <div className="trading-slider-header">
        <span className="trading-slider-title">{title}</span>
      </div>
      
      <div className="trading-slider-wrapper">
        <Slider
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={onChange}
          marks={marks}
          disabled={disabled}
          tooltip={{
            formatter: tooltipFormatter
          }}
          className="trading-slider"
        />
      </div>

      {displayValue && (
        <div className="trading-slider-display">
          <span className="trading-slider-display-label">{getPriceLabel()}</span>
          <span className="trading-slider-display-value"> {displayValue}</span>
        </div>
      )}
    </div>
  );
};

export default TradingSlider;
