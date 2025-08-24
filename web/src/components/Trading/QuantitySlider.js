import React from 'react';
import { Slider, Typography } from 'antd';

const { Text } = Typography;

/**
 * 通用数量滑块组件
 * @param {string} action - 操作类型
 * @param {number} quantity - 当前数量
 * @param {number} maxQuantity - 最大数量
 * @param {Function} onQuantityChange - 数量变化回调
 * @param {Object} precision - 精度信息
 * @param {string} symbol - 交易对符号
 * @param {Object} config - 操作配置
 */
const QuantitySlider = ({ 
  action, 
  quantity, 
  maxQuantity, 
  onQuantityChange, 
  precision, 
  symbol,
  config 
}) => {
  const baseAsset = symbol.replace('USDT', '');
  


  // 优化marks显示
  const optimizedMarks = {
    0: '0%',
    [maxQuantity * 0.25]: '25%',
    [maxQuantity * 0.5]: '50%', 
    [maxQuantity * 0.75]: '75%',
    [maxQuantity]: '100%'
  };

  return (
    <div style={{ marginBottom: 24 }}>
      <Text strong>{config.quantityLabel || `${baseAsset}数量`}: </Text>
      <div style={{ margin: '8px 0', paddingLeft: 8, paddingRight: 8 }}>
        <Slider
          min={precision ? parseFloat(precision.min_qty) : 0.001}
          max={maxQuantity}
          step={precision ? parseFloat(precision.step_size) : 0.001}
          value={quantity}
          onChange={onQuantityChange}
          tooltip={{
            formatter: (value) => `${value?.toFixed(precision?.quantity_precision || 6)} ${baseAsset}`
          }}
          marks={optimizedMarks}
        />
      </div>
      
      {/* 分离的信息显示区域 */}
      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        fontSize: '11px', 
        color: '#666', 
        marginTop: 16,
        paddingLeft: 8,
        paddingRight: 8,
        background: '#fafafa',
        padding: '8px 12px',
        borderRadius: '6px',
        border: '1px solid #f0f0f0'
      }}>
        <span style={{ fontWeight: 500 }}>
          当前: {quantity.toFixed(precision?.quantity_precision || 4)}
        </span>
        <span style={{ fontWeight: 500 }}>
          最大: {maxQuantity.toFixed(precision?.quantity_precision || 4)}
        </span>
      </div>
    </div>
  );
};

export default QuantitySlider;
