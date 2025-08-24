import React from 'react';
import { Slider, Typography } from 'antd';

const { Text } = Typography;

/**
 * 通用价格滑块组件
 * @param {string} action - 操作类型
 * @param {number} currentPrice - 当前价格
 * @param {number} entryPrice - 开仓价格
 * @param {number} percentage - 价格百分比
 * @param {Function} onPercentageChange - 百分比变化回调
 * @param {number} targetPrice - 目标价格
 * @param {Object} precision - 精度信息
 * @param {Object} config - 操作配置
 */
const PriceSlider = ({ 
  action, 
  currentPrice, 
  entryPrice, 
  percentage, 
  onPercentageChange, 
  targetPrice, 
  precision,
  config 
}) => {
  const basePrice = config.priceBase === 'current' ? currentPrice : entryPrice;
  
  // 简化marks，避免文字重叠
  const simplifiedMarks = {
    [config.priceRange.min]: `${config.priceRange.min}%`,
    0: config.priceBase === 'current' ? '当前价格' : '开仓价格',
    [config.priceRange.max]: `+${config.priceRange.max}%`
  };

  return (
    <div style={{ marginBottom: 24 }}>
      <Text strong>{config.priceLabel}: </Text>
      <div style={{ margin: '8px 0', paddingLeft: 8, paddingRight: 8 }}>
        <Slider
          min={config.priceRange.min}
          max={config.priceRange.max}
          step={0.1}
          value={percentage}
          onChange={onPercentageChange}
          tooltip={{
            formatter: (value) => `${value > 0 ? '+' : ''}${value}%`
          }}
          marks={simplifiedMarks}
        />
      </div>
      
      {/* 分离的价格信息显示 */}
      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        fontSize: '11px', 
        color: '#666', 
        marginTop: 16,
        background: '#fafafa',
        padding: '8px 12px',
        borderRadius: '6px',
        border: '1px solid #f0f0f0'
      }}>
        <span style={{ fontWeight: 500, whiteSpace: 'nowrap' }}>
          目标价格: ${targetPrice.toFixed(4)}
        </span>
        <span style={{ fontWeight: 500, whiteSpace: 'nowrap' }}>
          {config.priceBase === 'current' ? '当前' : '开仓'}价格: ${basePrice.toFixed(4)}
        </span>
      </div>
    </div>
  );
};

export default PriceSlider;
