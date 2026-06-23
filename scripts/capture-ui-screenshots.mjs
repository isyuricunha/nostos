import { chromium } from '../web/node_modules/@playwright/test/index.mjs';
import { createServer } from 'node:http';
import { mkdir, readFile, stat } from 'node:fs/promises';
import { dirname, extname, join, normalize, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const screenshotDir = join(root, 'docs', 'screenshots');
const baseURL = process.env.UI_SCREENSHOT_BASE_URL ?? 'http://127.0.0.1:17000';
const providerURL = process.env.UI_SCREENSHOT_PROVIDER_URL ?? 'http://127.0.0.1:17011';
const mcpURL = process.env.UI_SCREENSHOT_MCP_URL ?? 'http://127.0.0.1:17012/mcp';
const referenceDir = process.env.ODYSSEUS_REFERENCE_DIR ?? '/tmp/odysseus-ui-reference';
const referencePort = Number(process.env.ODYSSEUS_REFERENCE_PORT ?? '18101');
const owner = {
  email: 'screenshots@example.com',
  password: 'very-secure-screenshot-password'
};

await mkdir(screenshotDir, { recursive: true });

const referenceServer = await startReferenceServer(referenceDir, referencePort);
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });

try {
  await setupOrLogin(page);
  await seedWorkspace(page);
  await page.reload();
  await page.getByRole('heading', { name: 'Chat' }).waitFor();
  await prepareChat(page);

  await page.setViewportSize({ width: 1600, height: 900 });
  await page.screenshot({ path: shot('fidelity-chat-desktop.png'), fullPage: false });

  await captureStreamingState(page);
  await page.screenshot({ path: shot('fidelity-chat-streaming.png'), fullPage: false });

  await openAssistantMessageMenu(page);
  await page.screenshot({ path: shot('fidelity-message-menu.png'), fullPage: false });

  await page.keyboard.press('Escape');
  await openAssistantMessageDetails(page);
  await page.screenshot({ path: shot('fidelity-message-stats.png'), fullPage: false });

  await page.keyboard.press('Escape');
  await page.getByPlaceholder('Search').first().fill('New');
  await page.screenshot({ path: shot('fidelity-sidebar.png'), fullPage: false });
  await page.getByPlaceholder('Search').first().fill('');

  await openModelPicker(page);
  await page.screenshot({ path: shot('fidelity-model-picker.png'), fullPage: false });
  await page.keyboard.press('Escape');

  await openSettings(page, 'Add Provider');
  await page.screenshot({ path: shot('fidelity-settings-add-provider.png'), fullPage: false });
  await page.getByRole('button', { name: 'Providers' }).click();
  await page.waitForTimeout(250);
  await page.screenshot({ path: shot('fidelity-settings-providers.png'), fullPage: false });
  await page.getByRole('button', { name: 'AI Defaults' }).click();
  await page.waitForTimeout(250);
  await page.screenshot({ path: shot('fidelity-settings-defaults.png'), fullPage: false });
  await page.getByRole('button', { name: 'Appearance' }).click();
  await page.waitForTimeout(250);
  await page.screenshot({ path: shot('fidelity-settings-appearance.png'), fullPage: false });

  await openNav(page, 'Memories');
  await page.screenshot({ path: shot('fidelity-memories.png'), fullPage: false });

  await openNav(page, 'Agents');
  await page.screenshot({ path: shot('fidelity-agents.png'), fullPage: false });

  await openNav(page, 'Tasks');
  await page.screenshot({ path: shot('fidelity-tasks.png'), fullPage: false });

  await openNav(page, 'Tools');
  await page.screenshot({ path: shot('fidelity-tools.png'), fullPage: false });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await page.getByRole('heading', { name: 'Chat' }).waitFor();
  await page.screenshot({ path: shot('fidelity-mobile-chat.png'), fullPage: false });
  await page.getByRole('button', { name: 'Open navigation' }).click();
  await page.getByLabel('Workspace windows').getByRole('button', { name: 'Memories', exact: true }).click();
  await page.getByRole('dialog', { name: 'Memories' }).waitFor();
  await page.waitForTimeout(400);
  await page.screenshot({ path: shot('fidelity-mobile-window.png'), fullPage: false });

  if (referenceServer) {
    await captureReferenceScreens(browser, referenceServer.url);
  }
} finally {
  await browser.close();
  await referenceServer?.close();
}

function shot(name) {
  return join(screenshotDir, name);
}

async function setupOrLogin(page) {
  await page.goto(baseURL);
  if (await page.getByRole('heading', { name: 'Create owner account' }).isVisible().catch(() => false)) {
    await page.getByLabel('Email').fill(owner.email);
    await page.getByLabel('Display name').fill('Screenshot Owner');
    await page.getByLabel('Password', { exact: true }).fill(owner.password);
    await page.getByLabel('Confirm password').fill(owner.password);
    await page.getByRole('button', { name: 'Create owner' }).click();
    await page.getByRole('heading', { name: 'Chat' }).waitFor();
    return;
  }
  if (await page.getByRole('heading', { name: 'Sign in' }).isVisible().catch(() => false)) {
    await page.getByLabel('Email').fill(owner.email);
    await page.getByLabel('Password', { exact: true }).fill(owner.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.getByRole('heading', { name: 'Chat' }).waitFor();
  }
}

async function seedWorkspace(page) {
  const provider = await ensureProvider(page);
  await ensureModels(page, provider.id);
  await ensureAgent(page, provider.id);
  await ensureMemory(page);
  const serverId = await ensureMCPServer(page);
  await ensureTask(page);

  const agents = await api(page, '/api/v1/agents');
  const agentId = agents.agents?.find((agent) => agent.name === 'Research Agent')?.id;
  if (agentId && serverId) {
    await api(page, `/api/v1/agents/${agentId}/mcp-servers`, 'PUT', { server_ids: [serverId] }).catch(() => undefined);
  }
}

async function ensureProvider(page) {
  const existing = await api(page, '/api/v1/providers').catch(() => ({ providers: [] }));
  const found = existing.providers?.find((provider) => provider.name === 'Screenshot Provider');
  if (found) return found;
  const response = await api(page, '/api/v1/providers', 'POST', {
    name: 'Screenshot Provider',
    base_url: providerURL,
    api_key: 'mock-key',
    enabled: true,
    request_timeout_ms: 60000,
    default_model: 'e2e-model',
    fallback_model: 'e2e-fallback'
  });
  return response.provider;
}

async function ensureModels(page, providerId) {
  await api(page, `/api/v1/providers/${providerId}/models/refresh`, 'POST').catch(() => undefined);
  for (let attempt = 0; attempt < 50; attempt += 1) {
    const status = await api(page, `/api/v1/providers/${providerId}/models/refresh-status`).catch(() => undefined);
    if (!status?.refresh || status.refresh.state === 'succeeded' || status.refresh.state === 'failed') break;
    await page.waitForTimeout(250);
  }
}

async function ensureAgent(page, providerId) {
  const existing = await api(page, '/api/v1/agents').catch(() => ({ agents: [] }));
  if (existing.agents?.some((agent) => agent.name === 'Research Agent')) return;
  await api(page, '/api/v1/agents', 'POST', {
    name: 'Research Agent',
    description: 'Compact agent profile for screenshot validation.',
    avatar: 'sparkles',
    system_prompt: 'Use selected memories and approved tools.',
    default_provider_id: providerId,
    default_model: 'e2e-model',
    fallback_model: 'e2e-fallback',
    temperature: 0.7,
    max_tool_iterations: 8,
    memory_access_mode: 'pinned_only',
    tool_permission_default: 'ask',
    active: true
  });
}

async function ensureMemory(page) {
  const existing = await api(page, '/api/v1/memories').catch(() => ({ memories: [] }));
  if (existing.memories?.some((memory) => memory.title === 'Preferred name')) return;
  await api(page, '/api/v1/memories', 'POST', {
    title: 'Preferred name',
    content: 'The user prefers to be called Yuri.',
    tags: ['identity', 'preference'],
    scope: 'global',
    importance: 80,
    pinned: true,
    active: true,
    source: 'manual'
  });
}

async function ensureMCPServer(page) {
  const existing = await api(page, '/api/v1/mcp-servers').catch(() => ({ servers: [] }));
  const found = existing.servers?.find((server) => server.name === 'Mock MCP');
  const server = found ?? (await api(page, '/api/v1/mcp-servers', 'POST', {
    name: 'Mock MCP',
    description: 'Mock tools for validation.',
    transport_type: 'http',
    http_url: mcpURL,
    enabled: true,
    startup_timeout_ms: 10000,
    request_timeout_ms: 30000
  })).server;
  await api(page, `/api/v1/mcp-servers/${server.id}/discover`, 'POST').catch(() => undefined);
  return server.id;
}

async function ensureTask(page) {
  const existing = await api(page, '/api/v1/tasks').catch(() => ({ tasks: [] }));
  if (existing.tasks?.some((record) => record.task?.name === 'Daily summary')) return;
  await api(page, '/api/v1/tasks', 'POST', {
    name: 'Daily summary',
    description: 'Summarize workspace signals.',
    task_type: 'agent',
    state: 'enabled',
    prompt: 'Summarize the latest workspace context.',
    tool_policy: 'use_preapproved_tools_only',
    max_retries: 3,
    timeout_ms: 600000,
    concurrency_policy: 'skip',
    schedule_mode: 'manual',
    timezone: 'UTC'
  });
}

async function prepareChat(page) {
  await page.getByRole('button', { name: /New Chat/i }).click();
  await sendChat(page, 'My name is Yuri.');
  await sendChat(page, 'What is my name?');
}

async function captureStreamingState(page) {
  await page.getByPlaceholder('Send a message...').fill('Give me a short status update.');
  await page.getByRole('button', { name: 'Send' }).click();
  await page.waitForTimeout(100);
}

async function sendChat(page, content) {
  await page.getByPlaceholder('Send a message...').fill(content);
  await page.getByRole('button', { name: 'Send' }).click();
  await page.getByText(/Hello from the mock provider\.|Your name is Yuri\./).last().waitFor();
  await waitForIdleComposer(page);
}

async function waitForIdleComposer(page) {
  await page
    .locator('.generation-indicator')
    .waitFor({ state: 'detached', timeout: 8000 })
    .catch(() => undefined);
  await page.getByRole('button', { name: 'Send' }).waitFor({ timeout: 8000 }).catch(() => undefined);
}

async function openAssistantMessageMenu(page) {
  const assistant = page.locator('.chat-message.assistant').last();
  await assistant.hover();
  await assistant.getByRole('button', { name: 'Message menu' }).click();
  await page.getByRole('menu').waitFor();
}

async function openAssistantMessageDetails(page) {
  const assistant = page.locator('.chat-message.assistant').last();
  await assistant.hover();
  await assistant.getByRole('button', { name: 'View message details' }).click();
  await page.locator('.message-details-panel').waitFor();
}

async function openModelPicker(page) {
  await page.getByRole('button', { name: /Chat model/i }).click();
  await page.getByPlaceholder('Search provider, model, capability').fill('NVIDIA NIM/openai/gpt-oss-120b');
  await page.getByRole('dialog', { name: 'Chat model picker' }).waitFor();
}

async function openSettings(page, section) {
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await page.getByRole('dialog', { name: 'Settings' }).waitFor();
  await page.getByRole('button', { name: section }).click();
  await page.waitForTimeout(250);
}

async function openNav(page, name) {
  await page.getByLabel('Workspace windows').getByRole('button', { name, exact: true }).click();
  if (name !== 'Chat') {
    await page.getByRole('dialog', { name }).waitFor();
  }
}

async function api(page, path, method = 'GET', body) {
  return page.evaluate(
    async ({ path: requestPath, method: requestMethod, body: requestBody }) => {
      const cookie = document.cookie
        .split(';')
        .map((value) => value.trim())
        .find((value) => value.startsWith('nostos_csrf='));
      const csrf = cookie ? decodeURIComponent(cookie.slice('nostos_csrf='.length)) : '';
      const response = await fetch(requestPath, {
        method: requestMethod,
        credentials: 'include',
        headers: {
          Accept: 'application/json',
          ...(requestBody === undefined ? {} : { 'Content-Type': 'application/json' }),
          ...(csrf ? { 'X-CSRF-Token': csrf } : {})
        },
        body: requestBody === undefined ? undefined : JSON.stringify(requestBody)
      });
      if (!response.ok) {
        throw new Error(`${requestMethod} ${requestPath} failed: ${await response.text()}`);
      }
      return response.json();
    },
    { path, method, body }
  );
}

async function startReferenceServer(dir, port) {
  if (!(await exists(join(dir, 'static', 'index.html')))) return null;
  const server = createServer(async (request, response) => {
    try {
      const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
      const relative = url.pathname === '/' ? 'static/index.html' : url.pathname.replace(/^\/+/, '');
      const filePath = resolve(dir, normalize(relative));
      if (!filePath.startsWith(resolve(dir))) {
        response.writeHead(403);
        response.end('forbidden');
        return;
      }
      const file = await readFile(filePath);
      response.writeHead(200, { 'Content-Type': contentType(filePath) });
      response.end(file);
    } catch {
      response.writeHead(404, { 'Content-Type': 'text/plain' });
      response.end('not found');
    }
  });
  await new Promise((resolveListen) => server.listen(port, '127.0.0.1', resolveListen));
  return {
    url: `http://127.0.0.1:${port}`,
    close: () => new Promise((resolveClose) => server.close(resolveClose))
  };
}

async function captureReferenceScreens(browser, referenceURL) {
  const ref = await browser.newPage({ viewport: { width: 1600, height: 900 } });
  try {
    await ref.goto(`${referenceURL}/static/index.html`, { waitUntil: 'domcontentloaded' });
    await ref.waitForTimeout(2500);
    await ref.screenshot({ path: shot('odysseus-reference-main-chat.png'), fullPage: false });

    await ref.locator('#tool-memory-btn').click().catch(() => undefined);
    await ref.waitForTimeout(400);
    await ref.screenshot({ path: shot('odysseus-reference-memories.png'), fullPage: false });

    await ref.keyboard.press('Escape');
    await ref.locator('#tool-tasks-btn').click().catch(() => undefined);
    await ref.waitForTimeout(400);
    await ref.screenshot({ path: shot('odysseus-reference-tasks.png'), fullPage: false });

    await ref.keyboard.press('Escape');
    await ref.locator('#user-bar-settings').click().catch(() => undefined);
    await ref.waitForTimeout(400);
    await ref.screenshot({ path: shot('odysseus-reference-settings.png'), fullPage: false });

    await ref.setViewportSize({ width: 390, height: 844 });
    await ref.reload({ waitUntil: 'domcontentloaded' });
    await ref.waitForTimeout(2000);
    await ref.screenshot({ path: shot('odysseus-reference-mobile-chat.png'), fullPage: false });
  } finally {
    await ref.close();
  }
}

async function exists(path) {
  return stat(path)
    .then(() => true)
    .catch(() => false);
}

function contentType(path) {
  const extension = extname(path);
  if (extension === '.html') return 'text/html';
  if (extension === '.css') return 'text/css';
  if (extension === '.js') return 'text/javascript';
  if (extension === '.svg') return 'image/svg+xml';
  if (extension === '.png') return 'image/png';
  if (extension === '.woff2') return 'font/woff2';
  if (extension === '.json' || extension === '.webmanifest') return 'application/json';
  return 'application/octet-stream';
}
