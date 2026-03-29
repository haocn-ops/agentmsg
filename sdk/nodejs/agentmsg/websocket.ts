import WebSocket from 'ws';
import { EventEmitter } from 'eventemitter3';

export interface WSMessage {
  type: string;
  id?: string;
  conversationId?: string;
  subType?: string;
  recipients?: string[];
  content?: any;
  metadata?: any;
  deliveryGuarantee?: string;
  taskContext?: any;
  sender?: any;
  createdAt?: number;
}

export class WebSocketClient extends EventEmitter {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnect: boolean;
  private maxRetries: number;
  private retries: number = 0;
  private reconnectDelay: number = 1000;
  private pingInterval: NodeJS.Timeout | null = null;
  private pongTimeout: NodeJS.Timeout | null = null;
  private isIntentionallyClosed: boolean = false;

  constructor(url: string, reconnect: boolean = true, maxRetries: number = 5) {
    super();
    this.url = url;
    this.reconnect = reconnect;
    this.maxRetries = maxRetries;
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.isIntentionallyClosed = false;
      this.ws = new WebSocket(this.url);

      this.ws.on('open', () => {
        this.retries = 0;
        this.startHeartbeat();
        this.emit('connected');
        resolve();
      });

      this.ws.on('message', (data: WebSocket.Data) => {
        try {
          const message = JSON.parse(data.toString()) as WSMessage;
          this.emit('message', message);
        } catch (e) {
          this.emit('error', new Error('Failed to parse message'));
        }
      });

      this.ws.on('close', () => {
        this.stopHeartbeat();
        this.emit('disconnected');

        if (!this.isIntentionallyClosed && this.reconnect) {
          this.reconnectWithBackoff();
        }
      });

      this.ws.on('error', (error: Error) => {
        this.emit('error', error);
        reject(error);
      });

      this.ws.on('pong', () => {
        if (this.pongTimeout) {
          clearTimeout(this.pongTimeout);
          this.pongTimeout = null;
        }
      });
    });
  }

  private startHeartbeat(): void {
    this.pingInterval = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.ping();

        this.pongTimeout = setTimeout(() => {
          if (this.ws) {
            this.ws.terminate();
          }
        }, 10000);
      }
    }, 30000);
  }

  private stopHeartbeat(): void {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
    if (this.pongTimeout) {
      clearTimeout(this.pongTimeout);
      this.pongTimeout = null;
    }
  }

  private reconnectWithBackoff(): void {
    if (this.retries >= this.maxRetries) {
      this.emit('error', new Error('Max reconnection attempts reached'));
      return;
    }

    this.retries++;
    const delay = this.reconnectDelay * Math.pow(2, this.retries - 1);

    setTimeout(() => {
      this.emit('reconnecting', { attempt: this.retries, delay });
      this.connect().catch(() => {});
    }, delay);
  }

  send(message: WSMessage): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      throw new Error('WebSocket is not connected');
    }
  }

  close(): void {
    this.isIntentionallyClosed = true;
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }
}
