import axios, { AxiosInstance } from 'axios';
import { EventEmitter } from 'eventemitter3';
import { WebSocketClient, WSMessage } from './websocket';
import {
  Agent,
  Message,
  Capability,
  MessageType,
  DeliveryGuarantee,
  SendResult,
  AgentStatus,
  Subscription,
  SubType,
  SubStatus,
  SubscriptionFilter,
} from './models';
import {
  AgentMsgError,
  ConnectionError,
  AuthenticationError,
  RateLimitError,
  TimeoutError,
} from './errors';

export interface ClientConfig {
  apiKey: string;
  agentId: string;
  endpoint?: string;
  reconnect?: boolean;
  maxRetries?: number;
}

export class Client extends EventEmitter {
  private config: ClientConfig;
  private http: AxiosInstance;
  private ws: WebSocketClient | null = null;
  private messageHandlers: ((msg: Message) => void)[] = [];

  constructor(config: ClientConfig) {
    super();
    this.config = {
      endpoint: 'https://api.agentmsg.cloud',
      reconnect: true,
      maxRetries: 5,
      ...config,
    };

    const baseURL = this.config.endpoint!.replace('wss://', 'https://').replace('ws://', 'https://');

    this.http = axios.create({
      baseURL,
      headers: {
        Authorization: `Bearer ${this.config.apiKey}`,
        'X-Agent-ID': this.config.agentId,
      },
      timeout: 30000,
    });
  }

  async connect(): Promise<void> {
    const wsUrl = `${this.config.endpoint!.replace('https://', 'wss://')}/api/v1/ws?token=${this.config.apiKey}&agent_id=${this.config.agentId}`;

    this.ws = new WebSocketClient(wsUrl, this.config.reconnect, this.config.maxRetries);

    this.ws.on('message', (msg: WSMessage) => {
      if (msg.type === 'message' && msg.content) {
        const message = this.parseMessage(msg);
        this.messageHandlers.forEach(handler => handler(message));
        this.emit('message', message);
      } else if (msg.type === 'ack') {
        this.emit('ack', msg);
      }
    });

    this.ws.on('disconnected', () => {
      this.emit('disconnected');
    });

    this.ws.on('reconnecting', (info: { attempt: number; delay: number }) => {
      this.emit('reconnecting', info);
    });

    try {
      await this.ws.connect();
      this.emit('connected');
    } catch (error) {
      throw new ConnectionError(`Failed to connect: ${error}`);
    }
  }

  async disconnect(): Promise<void> {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.emit('disconnected');
  }

  private parseMessage(wsMsg: WSMessage): Message {
    return {
      id: wsMsg.id || '',
      conversationId: wsMsg.conversationId || '',
      messageType: (wsMsg.subType as MessageType) || MessageType.GENERIC,
      senderId: wsMsg.sender?.agentId || '',
      recipients: wsMsg.recipients || [],
      content: wsMsg.content,
      metadata: wsMsg.metadata,
      deliveryGuarantee: (wsMsg.deliveryGuarantee as DeliveryGuarantee) || DeliveryGuarantee.AT_LEAST_ONCE,
      status: 'received',
      tenantId: '',
      createdAt: wsMsg.createdAt?.toString() || new Date().toISOString(),
    };
  }

  async registerCapabilities(capabilities: Capability[]): Promise<Agent> {
    try {
      const response = await this.http.post<Agent>(
        `/api/v1/agents/${this.config.agentId}/capabilities`,
        capabilities
      );
      return response.data;
    } catch (error: any) {
      if (error.response?.status === 401) {
        throw new AuthenticationError('Invalid API key');
      }
      if (error.response?.status === 429) {
        throw new RateLimitError('Rate limit exceeded');
      }
      throw new AgentMsgError(`Failed to register capabilities: ${error}`);
    }
  }

  async sendHeartbeat(): Promise<void> {
    try {
      await this.http.post(`/api/v1/agents/${this.config.agentId}/heartbeat`);
    } catch (error) {
      throw new AgentMsgError(`Failed to send heartbeat: ${error}`);
    }
  }

  async sendMessage(
    content: any,
    recipients: string[],
    messageType: MessageType = MessageType.GENERIC,
    delivery: DeliveryGuarantee = DeliveryGuarantee.AT_LEAST_ONCE,
    metadata?: Record<string, any>
  ): Promise<SendResult> {
    if (!this.ws || !this.ws.isConnected()) {
      throw new ConnectionError('Not connected');
    }

    const msg: WSMessage = {
      type: 'message',
      id: this.generateId(),
      conversationId: this.generateId(),
      subType: messageType,
      recipients,
      content,
      metadata: metadata || {},
      deliveryGuarantee: delivery,
    };

    this.ws.send(msg);

    return {
      messageId: msg.id!,
      status: 'sent',
    };
  }

  async sendTaskRequest(
    taskDescription: string,
    recipients: string[] = [],
    priority: number = 0
  ): Promise<SendResult> {
    return this.sendMessage(
      { description: taskDescription },
      recipients,
      MessageType.TASK_REQUEST,
      DeliveryGuarantee.AT_LEAST_ONCE,
      { priority }
    );
  }

  onMessage(handler: (msg: Message) => void): void {
    this.messageHandlers.push(handler);
  }

  async discoverAgents(
    capabilityType?: string,
    minSuccessRate?: number,
    maxLatencyMs?: number
  ): Promise<Agent[]> {
    try {
      const params: Record<string, any> = {};
      if (capabilityType) params.capabilityType = capabilityType;
      if (minSuccessRate) params.minSuccessRate = minSuccessRate;
      if (maxLatencyMs) params.maxLatencyMs = maxLatencyMs;

      const response = await this.http.get<{ agents: Agent[] }>('/api/v1/discovery/query', { params });
      return response.data.agents;
    } catch (error) {
      throw new AgentMsgError(`Discovery failed: ${error}`);
    }
  }

  async createSubscription(
    type: SubType,
    filter: SubscriptionFilter
  ): Promise<Subscription> {
    try {
      const response = await this.http.post<Subscription>('/api/v1/subscriptions', {
        type,
        filter,
      });
      return response.data;
    } catch (error) {
      throw new AgentMsgError(`Failed to create subscription: ${error}`);
    }
  }

  async listSubscriptions(): Promise<Subscription[]> {
    try {
      const response = await this.http.get<Subscription[]>('/api/v1/subscriptions');
      return response.data;
    } catch (error) {
      throw new AgentMsgError(`Failed to list subscriptions: ${error}`);
    }
  }

  async deleteSubscription(subscriptionId: string): Promise<void> {
    try {
      await this.http.delete(`/api/v1/subscriptions/${subscriptionId}`);
    } catch (error) {
      throw new AgentMsgError(`Failed to delete subscription: ${error}`);
    }
  }

  private generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }
}

export * from './models';
export * from './errors';
export { Client as AgentMsgClient };
