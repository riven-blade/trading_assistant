import React from 'react';
import { Drawer, Form, Typography, Tag, Select, Button } from 'antd';
import TradingSlider from './TradingSlider';

const { Text } = Typography;

/**
 * 交易抽屉组件 - 以数量为主
 * @param {boolean} visible - 是否显示
 * @param {Function} onClose - 关闭回调
 * @param {string} symbol - 交易对
 * @param {string} side - 交易方向 ('long' | 'short')
 * @param {Function} onSubmit - 提交回调
 * @param {Object} priceData - 价格数据
 * @param {Object} accountValue - 账户信息
 * @param {number} targetPrice - 目标价格
 * @param {Function} onPriceChange - 价格变化回调
 * @param {number} quantity - 开仓数量
 * @param {Function} onQuantityChange - 数量变化回调
 * @param {string} orderType - 订单类型
 * @param {Function} onOrderTypeChange - 订单类型变化回调
 * @param {number} selectedLeverage - 杠杆
 * @param {Function} onLeverageChange - 杠杆变化回调
 * @param {string} marginMode - 保证金模式
 * @param {Function} onMarginModeChange - 保证金模式变化回调
 */
const TradeDrawer = ({
  visible,
  onClose,
  symbol,
  side,
  onSubmit,
  priceData,
  accountValue,
  targetPrice,
  onPriceChange,
  quantity,
  onQuantityChange,
  orderType,
  onOrderTypeChange,
  selectedLeverage,
  onLeverageChange,
  marginMode,
  onMarginModeChange
}) => {
  // 获取当前价格
  const markPrice = priceData?.[symbol]?.markPrice || 0;
  const baseAsset = symbol?.replace('USDT', '') || '';
  
  // 计算预估USDT金额
  const getPriceForCalculation = () => {
    // 限价单使用目标价格，市价单使用当前价格
    if (orderType === 'limit') {
      return targetPrice > 0 ? targetPrice : markPrice;
    }
    return markPrice;
  };
  
  const priceForCalculation = getPriceForCalculation();
  const estimatedUsdtAmount = priceForCalculation > 0 && quantity > 0 ? (quantity * priceForCalculation) / selectedLeverage : 0;
  
  // 计算最大数量
  const getMaxQuantity = () => {
    if (!markPrice || !selectedLeverage || !accountValue?.usdt_free) return 1;
    const maxUsdtAmount = accountValue.usdt_free * 0.8; // 使用80%的可用余额
    // 使用与保证金计算相同的价格逻辑
    const priceToUse = getPriceForCalculation();
    if (priceToUse <= 0) return 1;
    const maxQuantity = (maxUsdtAmount * selectedLeverage) / priceToUse;
    return parseFloat(maxQuantity.toFixed(6));
  };
  
  // 获取价格范围 (当前价格的 ±100%)
  const getPriceRange = () => {
    if (!markPrice) return [0, 100];
    const range = markPrice * 1.0;
    return [markPrice - range, markPrice + range];
  };

  const [minPrice, maxPrice] = getPriceRange();

  return (
    <Drawer
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span>{side === 'long' ? '开多' : '开空'} {symbol && symbol.length > 8 ? symbol.substring(0, 8) + '...' : symbol}</span>
          <Tag color={side === 'long' ? 'green' : 'red'}>
            {side === 'long' ? 'LONG' : 'SHORT'}
          </Tag>
        </div>
      }
      placement="right"
      width={400}
      onClose={onClose}
      open={visible}
      extra={
        <Button type="primary" onClick={onSubmit}>
          确认{side === 'long' ? '开多' : '开空'}
        </Button>
      }
    >
      <Form layout="vertical">
        {/* 交易信息概要 */}
        <div style={{ 
          background: side === 'long' ? '#f6ffed' : '#fff2f0', 
          border: `2px solid ${side === 'long' ? '#b7eb8f' : '#ffccc7'}`,
          padding: 16, 
          borderRadius: 8, 
          marginBottom: 16 
        }}>
          {/* 标题行 */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <Text strong style={{ color: side === 'long' ? '#52c41a' : '#ff4d4f', fontSize: '16px' }}>
              交易概要
            </Text>
            <Text strong style={{ color: side === 'long' ? '#52c41a' : '#ff4d4f' }}>
              {side === 'long' ? '做多 (LONG)' : '做空 (SHORT)'}
            </Text>
          </div>
          
          {/* 价格信息 */}
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">当前价格:</Text>
            <Text strong style={{ color: '#1890ff', fontSize: '16px' }}>${markPrice?.toFixed(4) || '0.0000'}</Text>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">可用余额:</Text>
            <Text>${accountValue?.usdt_free?.toFixed(2) || '0.00'} USDT</Text>
          </div>
          
          {/* 交易参数 */}
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">开仓数量:</Text>
            <Text strong style={{ color: '#52c41a', fontSize: '16px' }}>{quantity?.toFixed(6) || '0.000000'} {baseAsset}</Text>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">杠杆倍数:</Text>
            <Text strong>{selectedLeverage}x</Text>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">保证金模式:</Text>
            <Text strong style={{ color: marginMode === 'CROSS' ? '#1890ff' : '#fa8c16' }}>
              {marginMode === 'CROSS' ? '全仓' : '逐仓'}
            </Text>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">预估保证金:</Text>
            <Text strong>{estimatedUsdtAmount?.toFixed(2) || '0.00'} USDT</Text>
          </div>
          {orderType === 'limit' && (
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
              <Text type="secondary">限价价格:</Text>
              <Text strong>${targetPrice?.toFixed(4) || markPrice?.toFixed(4)}</Text>
            </div>
          )}
        </div>

        {/* 订单类型选择 */}
        <Form.Item label="订单类型">
          <Select 
            value={orderType} 
            onChange={onOrderTypeChange}
            style={{ width: '100%' }}
          >
            <Select.Option value="market">市价单</Select.Option>
            <Select.Option value="limit">限价单</Select.Option>
          </Select>
        </Form.Item>

        {/* 限价单价格设置 */}
        {orderType === 'limit' && (
          <Form.Item>
            <TradingSlider
              title="价格调整"
              value={targetPrice || markPrice}
              min={minPrice}
              max={maxPrice}
              step={markPrice * 0.001}
              onChange={onPriceChange}
              marks={{
                [markPrice]: '市场价'
              }}
              displayLabel="下单价:"
              displayValue={`$${(targetPrice || markPrice)?.toFixed(4)}`}
              tooltipFormatter={(value) => `$${value?.toFixed(4)}`}
            />
          </Form.Item>
        )}

        {/* 开仓数量设置 */}
        <Form.Item>
          <TradingSlider
            title={`${baseAsset}数量`}
            value={quantity || 0.001}
            min={0.001}
            max={getMaxQuantity()}
            step={0.001}
            onChange={onQuantityChange}
            marks={{
              0.001: '0%',
              [getMaxQuantity() * 0.25]: '25%',
              [getMaxQuantity() * 0.5]: '50%',
              [getMaxQuantity() * 0.75]: '75%',
              [getMaxQuantity()]: '100%'
            }}
            displayLabel="开仓数量:"
            displayValue={`${(quantity || 0)?.toFixed(6)} ${baseAsset}`}
            tooltipFormatter={(value) => `${value?.toFixed(6)} ${baseAsset}`}
          />
        </Form.Item>



        {/* 杠杆设置 */}
        <Form.Item label="杠杆倍数">
          <Select 
            value={selectedLeverage} 
            onChange={onLeverageChange}
            style={{ width: '100%' }}
          >
            {[1, 2, 3, 4, 5].map(leverage => (
              <Select.Option key={leverage} value={leverage}>
                {leverage}x
              </Select.Option>
            ))}
          </Select>
        </Form.Item>

        {/* 保证金模式设置 */}
        <Form.Item label="保证金模式">
          <Select 
            value={marginMode} 
            onChange={onMarginModeChange}
            style={{ width: '100%' }}
          >
            <Select.Option value="ISOLATED">逐仓模式</Select.Option>
            <Select.Option value="CROSS">全仓模式</Select.Option>
          </Select>
        </Form.Item>


      </Form>
    </Drawer>
  );
};

export default TradeDrawer;