const test = require('node:test');
const assert = require('node:assert/strict');

const { Client, MessageType, DeliveryGuarantee, AgentMsgError, AuthenticationError } = require('../dist');

test('connect checks health endpoint', async () => {
  const client = new Client({
    apiKey: 'token',
    agentId: 'agent-1',
    endpoint: 'https://example.com',
  });

  let called = false;
  client.http = {
    get: async (path) => {
      called = true;
      assert.equal(path, '/health');
      return { data: { status: 'healthy' } };
    },
  };

  await client.connect();
  assert.equal(called, true);
});

test('sendMessage returns REST result payload', async () => {
  const client = new Client({
    apiKey: 'token',
    agentId: 'agent-1',
    endpoint: 'https://example.com',
  });

  client.http = {
    post: async (path, body) => {
      assert.equal(path, '/api/v1/messages');
      assert.deepEqual(body, {
        messageType: MessageType.GENERIC,
        recipients: ['agent-2'],
        content: { text: 'hello' },
        metadata: {},
        deliveryGuarantee: DeliveryGuarantee.AT_LEAST_ONCE,
      });
      return { data: { messageId: 'msg-123', status: 'pending' } };
    },
  };

  const result = await client.sendMessage(
    { text: 'hello' },
    ['agent-2'],
    MessageType.GENERIC,
    DeliveryGuarantee.AT_LEAST_ONCE,
  );

  assert.deepEqual(result, { messageId: 'msg-123', status: 'pending' });
});

test('registerCapabilities maps 404 to a clear sdk error', async () => {
  const client = new Client({
    apiKey: 'token',
    agentId: 'agent-1',
    endpoint: 'https://example.com',
  });

  client.http = {
    put: async () => {
      const error = new Error('not found');
      error.response = { status: 404 };
      throw error;
    },
  };

  await assert.rejects(
    () => client.registerCapabilities([{ type: 'text-generation', description: 'text' }]),
    (error) =>
      error instanceof AgentMsgError &&
      error.message.includes('Agent must already exist before capabilities can be updated'),
  );
});

test('sendMessage maps 401 to AuthenticationError', async () => {
  const client = new Client({
    apiKey: 'token',
    agentId: 'agent-1',
    endpoint: 'https://example.com',
  });

  client.http = {
    post: async () => {
      const error = new Error('unauthorized');
      error.response = { status: 401 };
      throw error;
    },
  };

  await assert.rejects(
    () => client.sendMessage({ text: 'hello' }, ['agent-2']),
    AuthenticationError,
  );
});

test('onMessage is explicitly unavailable without realtime server support', () => {
  const client = new Client({
    apiKey: 'token',
    agentId: 'agent-1',
    endpoint: 'https://example.com',
  });

  assert.throws(
    () => client.onMessage(() => {}),
    (error) =>
      error instanceof AgentMsgError &&
      error.message.includes('Realtime message handlers are not available'),
  );
});
