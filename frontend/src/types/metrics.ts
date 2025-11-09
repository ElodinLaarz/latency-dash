export interface MetricsUpdate {
  targetId: string;
  key: string;
  min: number;
  max: number;
  avg: number;
  p90: number;
  count: number;
  lastUpdated: number;
  metadata: Record<string, string>;
}

export interface SubscriptionMessage {
  targetId: string;
  splitByMetadata: boolean;
  keys: string[];
}

export interface WebSocketMessage {
  metricsUpdate?: MetricsUpdate;
  subscription?: SubscriptionMessage;
}

export interface MetricsState {
  [key: string]: MetricsUpdate;
}
