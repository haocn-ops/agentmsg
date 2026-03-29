export class AgentMsgError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AgentMsgError';
  }
}

export class ConnectionError extends AgentMsgError {
  constructor(message: string) {
    super(message);
    this.name = 'ConnectionError';
  }
}

export class TimeoutError extends AgentMsgError {
  constructor(message: string) {
    super(message);
    this.name = 'TimeoutError';
  }
}

export class RateLimitError extends AgentMsgError {
  constructor(message: string) {
    super(message);
    this.name = 'RateLimitError';
  }
}

export class AuthenticationError extends AgentMsgError {
  constructor(message: string) {
    super(message);
    this.name = 'AuthenticationError';
  }
}

export class ValidationError extends AgentMsgError {
  constructor(message: string) {
    super(message);
    this.name = 'ValidationError';
  }
}
