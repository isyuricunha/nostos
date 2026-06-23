import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const root = resolve(new URL('..', import.meta.url).pathname);
const dataDir = mkdtempSync(join(tmpdir(), 'nostos-e2e-'));
const appPort = Number(process.env.E2E_APP_PORT ?? 17000);
const providerPort = Number(process.env.E2E_PROVIDER_PORT ?? 17011);
const mcpPort = Number(process.env.E2E_MCP_PORT ?? 17012);
const readyPort = Number(process.env.E2E_READY_PORT ?? 17099);
const children = [];

let providerRequests = [];
let toolExecutions = 0;
let runtimeReady = false;
const modelCatalog = [
  { id: 'e2e-model' },
  { id: 'e2e-fallback' },
  { id: 'NVIDIA NIM/openai/gpt-oss-120b' },
  { id: 'NVIDIA NIM/moonshotai/kimi-k2.6' }
];
for (let index = modelCatalog.length; index < 800; index += 1) {
  modelCatalog.push({ id: `Bifrost/generated/model-${String(index).padStart(3, '0')}` });
}

const provider = createServer(async (request, response) => {
  if (request.url === '/health') {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ ok: true }));
    return;
  }
  if (request.url === '/v1/models') {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ data: modelCatalog }));
    return;
  }
  if (request.url === '/v1/chat/completions') {
    const body = await readJSON(request);
    providerRequests.push(body);
    response.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache'
    });
    const messages = body.messages ?? [];
    const text = messages.map((message) => message.content ?? '').join('\n');
    const hasToolResult = messages.some((message) => message.role === 'tool');
    if ((body.tools ?? []).length > 0 && text.includes('approval tool') && !hasToolResult) {
      const toolName = body.tools[0].function.name;
      response.write(
        `data: {"choices":[{"delta":{"tool_calls":[{"id":"e2e_tool_call","type":"function","function":{"name":${JSON.stringify(toolName)},"arguments":"{\\\"service\\\":\\\"api\\\"}"}}]},"finish_reason":"tool_calls"}]}\n\n`
      );
      response.write('data: [DONE]\n\n');
      response.end();
      return;
    }
    if (text.includes('Older transcript to compact')) {
      streamText(response, 'Summary: Yuri said their name is Yuri and used the mock MCP tool.');
      return;
    }
    if (hasToolResult) {
      streamText(response, 'Tool-assisted answer from the mock provider.');
      return;
    }
    if (text.includes('Give me a short status update.')) {
      await streamTextSlow(response, 'Working through the current status update.');
      return;
    }
    if (text.includes('What is my name?') && text.includes('My name is Yuri.')) {
      streamText(response, 'Your name is Yuri.');
      return;
    }
    if (text.toLowerCase().includes('negative')) {
      streamText(response, 'Not really. I need more context before agreeing.');
      return;
    }
    streamText(response, 'Hello from the mock provider.');
    return;
  }
  response.writeHead(404);
  response.end('not found');
});

const mcp = createServer(async (request, response) => {
  if (request.url === '/health') {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ ok: true }));
    return;
  }
  const body = await readJSON(request);
  response.writeHead(200, { 'Content-Type': 'application/json' });
  if (body.method === 'tools/list') {
    response.end(
      JSON.stringify({
        jsonrpc: '2.0',
        id: body.id,
        result: {
          tools: [
            {
              name: 'lookup_status',
              description: 'Look up service status.',
              inputSchema: { type: 'object', properties: { service: { type: 'string' } } }
            }
          ]
        }
      })
    );
    return;
  }
  if (body.method === 'tools/call') {
    toolExecutions += 1;
    response.end(
      JSON.stringify({
        jsonrpc: '2.0',
        id: body.id,
        result: {
          content: [{ type: 'text', text: `api is healthy; execution ${toolExecutions}` }],
          isError: false
        }
      })
    );
    return;
  }
  response.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, error: { message: 'unknown method' } }));
});

const ready = createServer((request, response) => {
	if (request.url === '/health') {
		response.writeHead(runtimeReady ? 200 : 503, { 'Content-Type': 'application/json' });
		response.end(JSON.stringify({ ok: true, provider_requests: providerRequests.length, tool_executions: toolExecutions }));
		return;
	}
  response.writeHead(404);
  response.end('not found');
});

await listen(provider, providerPort);
await listen(mcp, mcpPort);
await listen(ready, readyPort);

const env = {
  ...process.env,
  APP_ENV: 'development',
  APP_HOST: '127.0.0.1',
  APP_PORT: String(appPort),
  APP_BASE_URL: `http://127.0.0.1:${appPort}`,
  APP_ENCRYPTION_KEY: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
  APP_SESSION_SECRET: 'e2e-session-secret-with-at-least-thirty-two-chars',
  DATABASE_DRIVER: 'sqlite',
  DATABASE_URL: join(dataDir, 'nostos.db'),
  DATA_DIR: dataDir,
  MIGRATIONS_DIR: join(root, 'migrations'),
  WEB_DIST_DIR: join(root, 'web', 'dist'),
  WORKER_CONCURRENCY: '2',
  WORKER_POLL_INTERVAL: '500ms',
  CHAT_CONTEXT_THRESHOLD: '4000',
  CHAT_RECENT_MESSAGE_LIMIT: '12',
  APP_LOG_LEVEL: 'warn'
};

children.push(spawn('go', ['run', './cmd/app', 'server'], { cwd: root, env, stdio: 'inherit' }));
await waitForURL(`http://127.0.0.1:${appPort}/health/live`, 120000);
children.push(spawn('go', ['run', './cmd/app', 'worker'], { cwd: root, env, stdio: 'inherit' }));
runtimeReady = true;

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

function streamText(response, text) {
  response.write(`data: {"choices":[{"delta":{"content":${JSON.stringify(text)}}}]}\n\n`);
  response.write('data: [DONE]\n\n');
  response.end();
}

async function streamTextSlow(response, text) {
  for (const chunk of text.match(/.{1,12}/g) ?? [text]) {
    response.write(`data: {"choices":[{"delta":{"content":${JSON.stringify(chunk)}}}]}\n\n`);
    await new Promise((resolveWait) => setTimeout(resolveWait, 250));
  }
  response.write('data: [DONE]\n\n');
  response.end();
}

function readJSON(request) {
  return new Promise((resolveJSON) => {
    let data = '';
    request.on('data', (chunk) => {
      data += chunk;
    });
    request.on('end', () => {
      try {
        resolveJSON(data ? JSON.parse(data) : {});
      } catch {
        resolveJSON({});
      }
    });
  });
}

function listen(server, port) {
  return new Promise((resolveListen) => server.listen(port, '127.0.0.1', resolveListen));
}

async function waitForURL(url, timeoutMs) {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    try {
      const response = await fetch(url);
      if (response.ok) return;
    } catch {
      // Retry until the server is ready.
    }
    await new Promise((resolveWait) => setTimeout(resolveWait, 250));
  }
  throw new Error(`Timed out waiting for ${url}`);
}

function shutdown() {
  for (const child of children) {
    child.kill('SIGTERM');
  }
  provider.close();
  mcp.close();
  ready.close();
  rmSync(dataDir, { recursive: true, force: true });
  process.exit(0);
}
