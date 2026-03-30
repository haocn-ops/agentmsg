import axios, { AxiosInstance } from 'axios';
import { EventEmitter } from 'eventemitter3';
import {
  Agent,
  Capability,
  Message,
  MessageType,
  DeliveryGuarantee,
  SendResult,
  Subscription,
  SubType,
  SubscriptionFilter,
} from './models';
import {
  AgentMsgError,
  ConnectionError,
  AuthenticationError,
  RateLimitError,
} from './errors';

export interface ClientConfig {
  apiKey: string;
  agentId: string;
  endpoint?: string;
  publicKey?: string;
  name?: string;
  version?: string;
  provider?: string;
  reconnect?: boolean;
  maxRetries?: number;
}

export class Client extends EventEmitter {
  private config: ClientConfig;
  private http: AxiosInstance;

  constructor(config: ClientConfig) {
    super();
    this.config = {
      endpoint: 'https://api.agentmsg.cloud',
      version: '0.1.0',
      provider: 'agentmsg-nodejs-sdk',
      reconnect: true,
      maxRetries: 5,
      ...config,
    };

    const baseURL = this.config.endpoint!.replace('wss://', 'https://').replace('ws://', 'https://');

    this.http = axios.create({
      baseURL,
      headers: {
        Authorization: `Bearer ${this.config.apiKey}`,
      },
      timeout: 30000,
    });
  }

  async connect(): Promise<void> {
    try {
      await this.http.get('/health');
      this.emit('connected');
    } catch (error) {
      throw new ConnectionError(`Failed to connect: ${error}`);
    }
  }

  async disconnect(): Promise<void> {
    this.emit('disconnected');
  }

  async registerCapabilities(capabilities: Capability[]): Promise<Agent> {
    try {
      const response = await this.http.put<Agent>(`/api/v1/agents/${this.config.agentId}`, {
        name: this.config.name || this.config.agentId,
        version: this.config.version,
        provider: this.config.provider,
        capabilities,
      });
      return response.data;
    } catch (error: any) {
      if (error.response?.status === 401) {
        throw new AuthenticationError('Invalid API key');
      }
      if (error.response?.status === 404) {
        throw new AgentMsgError('Agent must already exist before capabilities can be updated');
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
    try {
      const response = await this.http.post<SendResult>('/api/v1/messages', {
        messageType,
        recipients,
        content,
        metadata: metadata || {},
        deliveryGuarantee: delivery,
      });
      return response.data;
    } catch (error: any) {
      if (error.response?.status === 401) {
        throw new AuthenticationError('Invalid token');
      }
      if (error.response?.status === 429) {
        throw new RateLimitError('Rate limit exceeded');
      }
      throw new AgentMsgError(`Failed to send message: ${error}`);
    }
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
    void handler;
    throw new AgentMsgError('Realtime message handlers are not available in the current server build');
  }

  async discoverAgents(
    capabilityType?: string,
    minSuccessRate?: number,
    maxLatencyMs?: number
  ): Promise<Agent[]> {
    try {
      const tags: Record<string, string> = {};
      if (minSuccessRate !== undefined) tags.minSuccessRate = String(minSuccessRate);
      if (maxLatencyMs !== undefined) tags.maxLatencyMs = String(maxLatencyMs);

      const response = await this.http.post<{ agents: Agent[] }>('/api/v1/discovery/query', {
        capabilities: capabilityType ? [capabilityType] : [],
        tags,
      });
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
      const response = await this.http.get<{ subscriptions: Subscription[] }>('/api/v1/subscriptions');
      return response.data.subscriptions;
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
}

export * from './models';
export * from './errors';
export { Client as AgentMsgClient };
