import { useEffect, useRef, useState, useCallback } from 'react';
import { MetricsUpdate, WebSocketMessage } from '../types/metrics';

interface SubscriptionParams {
  targetId: string;
  keys: string[];
  splitView: boolean;
}

const useWebSocket = (url: string) => {
  const [isConnected, setIsConnected] = useState(false);
  const [metrics, setMetrics] = useState<Record<string, MetricsUpdate>>({});
  const [error, setError] = useState<Error | null>(null);
  const ws = useRef<WebSocket | null>(null);
  const subscriptionRef = useRef<SubscriptionParams>({ targetId: '', keys: [], splitView: false });

  useEffect(() => {
    const connect = () => {
      try {
        console.log(`Attempting to connect to WebSocket at: ${url}`);
        const socket = new WebSocket(url);
        
        // Cleanup on close
        socket.onclose = () => {
          console.log('WebSocket disconnected');
          setIsConnected(false);
          // Attempt to reconnect after a delay
          setTimeout(connect, 3000);
        };

        socket.onopen = () => {
          console.log('WebSocket connected');
          setIsConnected(true);
          setError(null);
          
          // Resubscribe with current parameters when reconnecting
          const { targetId, keys, splitView } = subscriptionRef.current;
          subscribe(targetId, keys, splitView);
        };

        socket.onmessage = (event) => {
          try {
            const message: WebSocketMessage = JSON.parse(event.data);
            if (message.metricsUpdate) {
              const update = message.metricsUpdate;
              const key = `${update.targetId}-${update.key}`;
              
              setMetrics(prev => ({
                ...prev,
                [key]: {
                  ...update,
                  lastUpdated: Date.now()
                }
              }));
            }
          } catch (err) {
            console.error('Error processing message:', err);
          }
        };

        // Moved to after socket creation for better organization

        socket.onerror = (event) => {
          console.error('WebSocket error event:', event);
          const error = new Error(`WebSocket connection error: ${event.type}`);
          console.error('WebSocket error details:', error);
          setError(error);
        };

        ws.current = socket;

        return () => {
          if (ws.current) {
            ws.current.close();
          }
        };
      } catch (err) {
        console.error('WebSocket connection failed:', err);
        setError(err instanceof Error ? err : new Error('Failed to connect to WebSocket'));
      }
    };

    connect();

    return () => {
      if (ws.current) {
        ws.current.close();
      }
    };
  }, [url]);

  const subscribe = useCallback((targetId: string, keys: string[], splitView: boolean) => {
    subscriptionRef.current = { targetId, keys, splitView };
    
    if (ws.current && ws.current.readyState === WebSocket.OPEN) {
      const message = {
        subscription: {
          targetId,
          keys,
          splitByMetadata: splitView
        }
      };
      
      try {
        ws.current.send(JSON.stringify(message));
        console.log('Sent subscription:', message);
      } catch (err) {
        console.error('Error sending subscription:', err);
      }
    }
  }, []);

  return {
    isConnected,
    metrics: Object.values(metrics),
    error,
    subscribe,
  };
};

export default useWebSocket;
