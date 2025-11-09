import React, { useEffect, useState, useRef } from 'react';
import { Table, Card, Tag, Space, Typography, Alert, Spin } from 'antd';
import { ThunderboltOutlined, ClockCircleOutlined } from '@ant-design/icons';
import useWebSocket from './hooks/useWebSocket';
import { MetricsUpdate } from './types/metrics';
import './App.css';

const { Title } = Typography;

const App: React.FC = () => {
  const [splitView, setSplitView] = useState(false);
  const [flashingRows, setFlashingRows] = useState<Set<string>>(new Set());
  const flashTimeouts = useRef<Map<string, NodeJS.Timeout>>(new Map());
  // Use localhost for development
  const wsUrl = process.env.NODE_ENV === 'development' 
    ? 'ws://localhost:8080/ws' 
    : `ws://${window.location.hostname}:8080/ws`;
    
  const { isConnected, metrics, error, subscribe } = useWebSocket(wsUrl);

  useEffect(() => {
    // Subscribe to all keys from all targets (empty arrays mean "all")
    subscribe('', [], splitView);
  }, [subscribe, splitView]);
  
  // Track metric updates for flash animation
  const prevMetricsRef = useRef<Record<string, MetricsUpdate>>({});
  useEffect(() => {
    metrics.forEach(metric => {
      const key = `${metric.targetId}-${metric.key}`;
      const prevMetric = prevMetricsRef.current[key];
      
      // If this is an update (not initial load), trigger flash
      if (prevMetric && prevMetric.lastUpdated !== metric.lastUpdated) {
        setFlashingRows(prev => new Set(prev).add(key));
        
        // Clear existing timeout for this row
        const existingTimeout = flashTimeouts.current.get(key);
        if (existingTimeout) {
          clearTimeout(existingTimeout);
        }
        
        // Remove flash after animation completes
        const timeout = setTimeout(() => {
          setFlashingRows(prev => {
            const next = new Set(prev);
            next.delete(key);
            return next;
          });
          flashTimeouts.current.delete(key);
        }, 500);
        
        flashTimeouts.current.set(key, timeout);
      }
      
      prevMetricsRef.current[key] = metric;
    });
  }, [metrics]);
  
  // Display connection status
  if (!isConnected) {
    return (
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        height: '100vh',
        flexDirection: 'column',
        gap: '16px'
      }}>
        <Spin size="large" />
        <Typography.Text>Connecting to metrics server...</Typography.Text>
        {error && (
          <Alert 
            message="Connection Error" 
            description={error.message} 
            type="error" 
            showIcon 
            style={{ maxWidth: '500px' }}
          />
        )}
      </div>
    );
  }

  // Group metrics by target
  const metricsByTarget = metrics.reduce((acc, metric) => {
    if (!acc[metric.targetId]) {
      acc[metric.targetId] = [];
    }
    acc[metric.targetId].push(metric);
    return acc;
  }, {} as Record<string, MetricsUpdate[]>);

  const columns = [
    {
      title: 'Key',
      dataIndex: 'key',
      key: 'key',
      render: (text: string) => <strong>{text}</strong>,
      width: 200,
    },
    {
      title: 'Min (ms)',
      dataIndex: 'min',
      key: 'min',
      render: (value: number) => value != null ? value.toFixed(2) : '-',
      sorter: (a: MetricsUpdate, b: MetricsUpdate) => (a.min || 0) - (b.min || 0),
      width: 100,
    },
    {
      title: 'Max (ms)',
      dataIndex: 'max',
      key: 'max',
      render: (value: number) => value != null ? value.toFixed(2) : '-',
      sorter: (a: MetricsUpdate, b: MetricsUpdate) => (a.max || 0) - (b.max || 0),
      width: 100,
    },
    {
      title: 'Avg (ms)',
      dataIndex: 'avg',
      key: 'avg',
      render: (value: number) => value != null ? value.toFixed(2) : '-',
      sorter: (a: MetricsUpdate, b: MetricsUpdate) => (a.avg || 0) - (b.avg || 0),
      width: 100,
    },
    {
      title: 'P90 (ms)',
      dataIndex: 'p90',
      key: 'p90',
      render: (value: number) => value != null ? value.toFixed(2) : '-',
      sorter: (a: MetricsUpdate, b: MetricsUpdate) => (a.p90 || 0) - (b.p90 || 0),
      width: 100,
    },
    {
      title: 'Count',
      dataIndex: 'count',
      key: 'count',
      sorter: (a: MetricsUpdate, b: MetricsUpdate) => a.count - b.count,
      width: 80,
    },
    {
      title: 'Last Updated',
      key: 'lastUpdated',
      render: (_: any, record: MetricsUpdate) => (
        <Space>
          <ClockCircleOutlined />
          {new Date(record.lastUpdated).toLocaleTimeString()}
        </Space>
      ),
      sorter: (a: MetricsUpdate, b: MetricsUpdate) => a.lastUpdated - b.lastUpdated,
      width: 150,
    },
  ];

  const expandedRowRender = (record: MetricsUpdate) => {
    if (!record.metadata) return null;
    
    return (
      <div style={{ padding: '16px 32px' }}>
        <h4>Metadata</h4>
        <pre style={{ margin: 0 }}>
          {JSON.stringify(record.metadata, null, 2)}
        </pre>
      </div>
    );
  };

  return (
    <div className="app">
      <header className="app-header">
        <Space>
          <ThunderboltOutlined style={{ fontSize: '24px', color: '#1890ff' }} />
          <Title level={2} style={{ margin: 0 }}>Latency Dashboard</Title>
        </Space>
        <div className="controls">
          <Tag color={isConnected ? 'success' : 'error'}>
            {isConnected ? 'Connected' : 'Disconnected'}
          </Tag>
        </div>
      </header>

      <main className="app-content">
        {error && (
          <Alert
            message="Connection Error"
            description={error.message}
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}

        <Card style={{ marginBottom: 16 }}>
          <div style={{ marginBottom: 0 }}>
            <span style={{ marginRight: 8 }}>View:</span>
            <Tag.CheckableTag
              checked={!splitView}
              onChange={() => setSplitView(false)}
            >
              Combined
            </Tag.CheckableTag>
            <Tag.CheckableTag
              checked={splitView}
              onChange={() => setSplitView(true)}
            >
              Split by Metadata
            </Tag.CheckableTag>
          </div>
        </Card>

        {isConnected ? (
          Object.keys(metricsByTarget).length > 0 ? (
            Object.entries(metricsByTarget).map(([targetId, targetMetrics]) => (
              <Card 
                key={targetId}
                title={
                  <Space>
                    <Tag color="blue">{targetId}</Tag>
                    <span style={{ fontWeight: 'normal', fontSize: '14px' }}>
                      {targetMetrics.length} {targetMetrics.length === 1 ? 'key' : 'keys'}
                    </span>
                  </Space>
                }
                style={{ marginBottom: 16 }}
              >
                <Table
                  columns={columns}
                  dataSource={targetMetrics.map((m) => ({ 
                    ...m, 
                    key: `${m.targetId}-${m.key}` 
                  }))}
                  pagination={false}
                  size="small"
                  expandable={{
                    expandedRowRender,
                    rowExpandable: (record) =>
                      !!record.metadata && Object.keys(record.metadata).length > 0,
                  }}
                  rowClassName={(record) => 
                    flashingRows.has(`${record.targetId}-${record.key}`) ? 'row-flash' : ''
                  }
                  scroll={{ x: 'max-content' }}
                />
              </Card>
            ))
          ) : (
            <Card>
              <div style={{ textAlign: 'center', padding: '40px 0' }}>
                <Spin size="large" />
                <div style={{ marginTop: 16 }}>Waiting for metrics...</div>
              </div>
            </Card>
          )
        ) : (
          <Card>
            <div style={{ textAlign: 'center', padding: '40px 0' }}>
              <Spin size="large" />
              <div style={{ marginTop: 16 }}>Connecting to server...</div>
            </div>
          </Card>
        )}
      </main>
    </div>
  );
};

export default App;
