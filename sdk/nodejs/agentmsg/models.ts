export enum MessageType {
  TASK_REQUEST = 'task.request',
  TASK_RESPONSE = 'task.response',
  TASK_STATUS_UPDATE = 'task.status-update',
  TASK_DELEGATE = 'task.delegate',
  CAPABILITY_QUERY = 'capability.query',
  CAPABILITY_ADVERT = 'capability.advertise',
  ERROR_REPORT = 'error.report',
  HEARTBEAT = 'heartbeat',
  GENERIC = 'generic',
}

export enum DeliveryGuarantee {
  AT_MOST_ONCE = 'at_most_once',
  AT_LEAST_ONCE = 'at_least_once',
  EXACTLY_ONCE = 'exactly_once',
}

export enum AgentStatus {
  ONLINE = 'online',
  AWAY = 'away',
  BUSY = 'busy',
  OFFLINE = 'offline',
}

export enum AckStatus {
  RECEIVED = 'received',
  PROCESSED = 'processed',
  REJECTED = 'rejected',
  FAILED = 'failed',
}

export interface Capability {
  type: string;
  description: string;
  examples?: string[];
  parameters?: CapabilityParams;
  constraints?: CapabilityConstraints;
  quality?: CapabilityQuality;
}

export interface CapabilityParams {
  inputFormat: string;
  outputFormat: string;
  maxDurationMs?: number;
  maxTokens?: number;
}

export interface CapabilityConstraints {
  rateLimit?: number;
  costPerCall?: number;
  quota?: number;
}

export interface CapabilityQuality {
  successRate?: number;
  avgLatencyMs?: number;
  rating?: number;
}

export interface Agent {
  id: string;
  tenantId: string;
  did: string;
  publicKey: string;
  name?: string;
  version?: string;
  provider?: string;
  tier?: string;
  capabilities: Capability[];
  endpoints?: Endpoint[];
  trustLevel: number;
  status: AgentStatus;
  lastHeartbeat: string;
  createdAt: string;
}

export interface Endpoint {
  type: string;
  url: string;
  weight?: number;
  isPrimary?: boolean;
}

export interface Message {
  id: string;
  conversationId: string;
  messageType: MessageType;
  senderId: string;
  recipients: string[];
  content: any;
  contentSize?: number;
  contentType?: string;
  metadata?: MessageMetadata;
  deliveryGuarantee: DeliveryGuarantee;
  status: string;
  taskContext?: TaskContext;
  traceId?: string;
  tenantId: string;
  createdAt: string;
  expiresAt?: string;
  processedAt?: string;
}

export interface MessageMetadata {
  tags?: Record<string, string>;
  correlationId?: string;
  replyTo?: string;
  routingHints?: RoutingHints;
  compression?: string;
  encoding?: string;
}

export interface RoutingHints {
  preferredAgents?: string[];
  excludedAgents?: string[];
  maxHops?: number;
  priceLimit?: number;
}

export interface TaskContext {
  taskId: string;
  parentTaskId?: string;
  rootTaskId?: string;
  priority: number;
  deadline?: string;
  dependencies?: string[];
  blocking?: boolean;
  retryPolicy?: RetryPolicy;
  retryCount?: number;
}

export interface RetryPolicy {
  maxAttempts: number;
  initialDelayMs: number;
  maxDelayMs: number;
  backoffMultiplier: number;
}

export interface SendResult {
  messageId: string;
  status: string;
  deliveredAt?: number;
}

export interface Subscription {
  id: string;
  agentId: string;
  tenantId: string;
  type: SubType;
  filter: SubscriptionFilter;
  status: SubStatus;
  createdAt: string;
}

export enum SubType {
  DIRECT = 'direct',
  CAPABILITY = 'capability',
  TOPIC = 'topic',
  PATTERN = 'pattern',
}

export enum SubStatus {
  ACTIVE = 'active',
  PAUSED = 'paused',
  CANCELLED = 'cancelled',
}

export interface SubscriptionFilter {
  agentIds?: string[];
  capabilityTypes?: string[];
  topics?: string[];
  messageTypes?: string[];
  tags?: Record<string, string>;
}
