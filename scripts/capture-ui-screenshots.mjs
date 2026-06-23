import { chromium } from '../web/node_modules/@playwright/test/index.mjs';
import { mkdir } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const screenshotDir = join(root, 'docs', 'screenshots');
const baseURL = process.env.UI_SCREENSHOT_BASE_URL ?? 'http://127.0.0.1:17000';
const providerURL = process.env.UI_SCREENSHOT_PROVIDER_URL ?? 'http://127.0.0.1:17011';
const mcpURL = process.env.UI_SCREENSHOT_MCP_URL ?? 'http://127.0.0.1:17012/mcp';
const owner = {
  email: 'screenshots@example.com',
  password: 'very-secure-screenshot-password'
};

await mkdir(screenshotDir, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });

try {
  await setupOrLogin(page);
  await seedWorkspace(page);
  await page.reload();
  await page.getByRole('button', { name: /New Chat/i }).click();
  await sendChat(page, 'My name is Yuri.');
  await page.screenshot({ path: shot('ui-chat-desktop.png'), fullPage: false });

  await page.setViewportSize({ width: 1365, height: 768 });
  await page.getByPlaceholder('Search').first().fill('New');
  await page.screenshot({ path: shot('ui-sidebar-conversations.png'), fullPage: false });
  await page.getByPlaceholder('Search').first().fill('');

  await page.setViewportSize({ width: 1600, height: 900 });
  await openSettings(page, 'Providers');
  await page.screenshot({ path: shot('ui-settings-models.png'), fullPage: false });
  await page.getByRole('button', { name: 'AI Defaults' }).click();
  await page.waitForTimeout(250);
  await page.screenshot({ path: shot('ui-settings-defaults.png'), fullPage: false });

  await openNav(page, 'Memories');
  await page.screenshot({ path: shot('ui-memories-window.png'), fullPage: false });

  await openNav(page, 'Agents');
  await page.screenshot({ path: shot('ui-agents-window.png'), fullPage: false });

  await openNav(page, 'Tasks');
  await page.screenshot({ path: shot('ui-tasks-window.png'), fullPage: false });

  await openNav(page, 'Tools');
  await page.getByRole('button', { name: 'Discovered Tools' }).click();
  await page.screenshot({ path: shot('ui-tools-window.png'), fullPage: false });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await page.getByRole('heading', { name: 'Chat' }).waitFor();
  await page.screenshot({ path: shot('ui-mobile-chat.png'), fullPage: false });
} finally {
  await browser.close();
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
  const provider = await api(page, '/api/v1/providers', 'POST', {
    name: 'Mock Provider',
    base_url: providerURL,
    api_key: 'mock-key',
    enabled: true,
    request_timeout_ms: 60000,
    default_model: 'e2e-model',
    fallback_model: 'e2e-fallback'
  });
  await api(page, `/api/v1/providers/${provider.provider.id}/models/refresh`, 'POST');
  for (let attempt = 0; attempt < 40; attempt += 1) {
    const status = await api(page, `/api/v1/providers/${provider.provider.id}/models/refresh-status`);
    if (status.refresh.state === 'succeeded') break;
    await page.waitForTimeout(250);
  }
  await api(page, '/api/v1/agents', 'POST', {
    name: 'Research Agent',
    description: 'Compact agent profile for screenshot validation.',
    avatar: 'sparkles',
    system_prompt: 'Use selected memories and approved tools.',
    default_provider_id: provider.provider.id,
    default_model: 'e2e-model',
    fallback_model: 'e2e-fallback',
    temperature: 0.7,
    max_tool_iterations: 8,
    memory_access_mode: 'pinned_only',
    tool_permission_default: 'ask',
    active: true
  });
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
  const server = await api(page, '/api/v1/mcp-servers', 'POST', {
    name: 'Mock MCP',
    description: 'Mock tools for validation.',
    transport_type: 'http',
    http_url: mcpURL,
    enabled: true,
    startup_timeout_ms: 10000,
    request_timeout_ms: 30000
  });
  await api(page, `/api/v1/mcp-servers/${server.server.id}/discover`, 'POST');
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

async function sendChat(page, content) {
  await page.getByPlaceholder('Send a message...').fill(content);
  await page.getByRole('button', { name: 'Send' }).click();
  await page.getByText('Hello from the mock provider.').first().waitFor();
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
