import { expect, test, type Page } from '@playwright/test';

const owner = {
  email: 'owner@example.com',
  password: 'very-secure-e2e-password'
};

test.beforeEach(async ({ page }) => {
  const errors: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' && !message.text().includes('401 (Unauthorized)')) errors.push(message.text());
  });
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('requestfailed', (request) => {
    const url = request.url();
    if (url.includes('/api/') || url.includes('/health/')) {
      errors.push(`${request.method()} ${url} failed: ${request.failure()?.errorText ?? 'unknown'}`);
    }
  });
  page.on('close', () => {
    expect(errors, `browser errors:\n${errors.join('\n')}`).toEqual([]);
  });
});

test('owner workspace release flow', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Create owner account' })).toBeVisible();
  await page.getByLabel('Email').fill(owner.email);
  await page.getByLabel('Display name').fill('E2E Owner');
  await page.getByLabel('Password', { exact: true }).fill(owner.password);
  await page.getByLabel('Confirm password').fill(owner.password);
  await page.getByRole('button', { name: 'Create owner' }).click();
  await expect(page.getByRole('heading', { name: 'Chat' })).toBeVisible();

  await page.getByRole('button', { name: 'Sign out' }).click();
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
  await page.getByLabel('Email').fill(owner.email);
  await page.getByLabel('Password', { exact: true }).fill(owner.password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('heading', { name: 'Chat' })).toBeVisible();

  await visitPrimaryScreens(page);
  await createProvider(page);
  const agentId = await createAgent(page);
  const { serverId, toolId } = await createMCPServerAndDiscoverTool(page);
  await api(page, `/api/v1/agents/${agentId}/mcp-servers`, 'PUT', { server_ids: [serverId] });

  await page.getByRole('button', { name: 'Chat' }).click();
  await page.getByRole('button', { name: 'New conversation' }).click();
  await page.getByLabel('Agent').selectOption({ label: 'E2E Agent' });
  await sendChat(page, 'My name is Yuri.');
  await expect(page.getByText('Hello from the mock provider.').first()).toBeVisible();
  await sendChat(page, 'What is my name?');
  await expect(page.getByText('Your name is Yuri.').first()).toBeVisible();

  await createMemory(page);
  await page.getByRole('button', { name: 'Chat' }).click();
  await sendChat(page, 'Use the pinned memory and answer briefly.');
  await expect(page.getByText('Memories used in this response')).toBeVisible();

  await openFirstMessageActions(page, 'Report response');
  await page.getByRole('button', { name: 'Helpful' }).first().click();
  await page.getByLabel('Negative feedback reason').first().selectOption('Too long');
  await page.getByRole('button', { name: 'Needs work' }).first().click();
  await openFirstMessageActions(page, 'Regenerate');
  await expect(page.getByText('Regeneration instruction')).toHaveCount(0);

  await openFirstMessageActions(page, 'Draft reply');
  await page.getByLabel('Preset').selectOption({ label: 'Negative' });
  await page.getByRole('button', { name: 'Generate draft' }).click();
  await expect(page.getByLabel('Generated reply draft')).toHaveValue(/Not really/);

  await page.getByRole('button', { name: 'MCP' }).click();
  await page.getByLabel(`Permission for lookup_status`).selectOption('ask');
  await page.getByRole('button', { name: 'Chat' }).click();
  await sendChat(page, 'Please use the approval tool to check API status.');
  await expect(page.getByText('lookup_status', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Approve once' }).click();
  await expect(page.getByText('Tool-assisted answer from the mock provider.')).toBeVisible();

  await api(page, `/api/v1/agents/${agentId}/mcp-tools/${toolId}/permission`, 'PUT', { permission_mode: 'allow' });
  await createAndRunTask(page);

  await page.getByRole('button', { name: 'Chat' }).click();
  await page.locator('summary').filter({ hasText: 'Conversation summary' }).click();
  await page.getByRole('button', { name: 'Regenerate summary' }).click();
  await expect(page.getByText(/Conversation summary regeneration queued|Conversation summary is already queued/)).toBeVisible();

  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'Refresh' }).first().click();
  await expect(page.getByText('worker heartbeat')).toBeVisible();
  await expect(page.getByText('provider health')).toBeVisible();
});

async function visitPrimaryScreens(page: Page): Promise<void> {
  for (const name of ['Providers', 'Agents', 'Memories', 'Tasks', 'MCP', 'Settings', 'Chat']) {
    await page.getByRole('button', { name, exact: true }).click();
    await expect(page.getByRole('heading', { level: 1, name })).toBeVisible();
  }
}

async function createProvider(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Providers' }).click();
  await page.getByRole('button', { name: 'New provider' }).click();
  await page.getByLabel('Name', { exact: true }).fill('Mock Provider');
  await page.getByLabel('Base URL').fill('http://127.0.0.1:17011');
  await page.getByLabel('API key').fill('mock-key');
  await page.getByLabel('Default model').fill('e2e-model');
  await page.getByLabel('Fallback model').fill('e2e-fallback');
  await page.getByRole('button', { name: 'Add provider' }).click();
  await expect(page.getByText('Mock Provider')).toBeVisible();
  await page.getByRole('button', { name: 'Test' }).click();
  await expect(page.getByText('Provider connection succeeded.')).toBeVisible();
  await page.getByRole('button', { name: 'Refresh models' }).click();
  await expect(page.getByText('Models refreshed.')).toBeVisible();
  const models = await api<{ models: Array<{ model_id: string; provider_id: string }> }>(page, '/api/v1/models?limit=1000&include_unavailable=true');
  expect(models.models.length).toBeGreaterThanOrEqual(800);
  expect(models.models.some((model) => model.model_id === 'NVIDIA NIM/openai/gpt-oss-120b')).toBeTruthy();
}

async function createAgent(page: Page): Promise<string> {
  await page.getByRole('button', { name: 'Agents' }).click();
  await page.getByRole('button', { name: 'New agent' }).click();
  await page.getByLabel('Name', { exact: true }).fill('E2E Agent');
  await page.getByLabel('Description').fill('Agent used by browser E2E.');
  await page.getByLabel('System prompt').fill('Use the selected memories and approved tools.');
  await selectModel(page, 'Agent default model', 'e2e-model');
  await page.getByLabel('Default tool permission').selectOption('ask');
  await page.getByRole('button', { name: 'Add agent' }).click();
  await expect(page.getByText('E2E Agent')).toBeVisible();
  const response = await api<{ agents: Array<{ id: string; name: string }> }>(page, '/api/v1/agents');
  return response.agents.find((agent) => agent.name === 'E2E Agent')?.id ?? '';
}

async function createMCPServerAndDiscoverTool(page: Page): Promise<{ serverId: string; toolId: string }> {
  await page.getByRole('button', { name: 'MCP' }).click();
  await page.getByRole('button', { name: 'New MCP server' }).click();
  await page.getByLabel('Name', { exact: true }).fill('Mock MCP');
  await page.getByLabel('Description').fill('Mock MCP server.');
  await page.getByLabel('HTTP URL').fill('http://127.0.0.1:17012/mcp');
  await page.getByRole('button', { name: 'Add MCP server' }).click();
  await expect(page.getByText('Mock MCP')).toBeVisible();
  await page.getByRole('button', { name: 'Discover tools' }).click();
  await expect(page.getByText('lookup_status')).toBeVisible();
  const servers = await api<{ servers: Array<{ id: string; name: string }> }>(page, '/api/v1/mcp-servers');
  const serverId = servers.servers.find((server) => server.name === 'Mock MCP')?.id ?? '';
  const tools = await api<{ tools: Array<{ id: string; name: string }> }>(page, '/api/v1/mcp-tools');
  const toolId = tools.tools.find((tool) => tool.name === 'lookup_status')?.id ?? '';
  return { serverId, toolId };
}

async function createMemory(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Memories' }).click();
  await page.getByRole('button', { name: 'New memory' }).click();
  await page.getByLabel('Title').fill('Preferred name');
  await page.getByLabel('Content').fill('The user prefers to be called Yuri.');
  await page.getByLabel('Tags').fill('identity, preference');
  await page.getByRole('button', { name: 'Add memory' }).click();
  await expect(page.getByText('Preferred name')).toBeVisible();
}

async function createAndRunTask(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Tasks' }).click();
  await page.getByRole('button', { name: 'New task' }).click();
  await page.getByLabel('Name', { exact: true }).fill('E2E task');
  await page.getByLabel('Type').selectOption('agent');
  await page.getByLabel('Prompt').fill('Use approval tool to check API status.');
  await page.getByRole('combobox', { name: 'Agent', exact: true }).selectOption({ label: 'E2E Agent' });
  await page.getByLabel('Schedule').selectOption('manual');
  await page.getByRole('button', { name: 'Add task' }).click();
  await expect(page.getByText('E2E task')).toBeVisible();
  const tasks = await api<{ tasks: Array<{ task: { id: string; name: string } }> }>(page, '/api/v1/tasks');
  const taskId = tasks.tasks.find((record) => record.task.name === 'E2E task')?.task.id ?? '';
  await page
    .locator('article')
    .filter({ has: page.getByText('E2E task', { exact: true }) })
    .getByRole('button', { name: 'Run now' })
    .click();
  await expect
    .poll(async () => {
      const response = await api<{ runs: Array<{ state: string }> }>(page, `/api/v1/task-runs?task_id=${taskId}`);
      return response.runs[0]?.state;
    })
    .toBe('succeeded');
  const runs = await api<{ runs: Array<{ id: string }> }>(page, `/api/v1/task-runs?task_id=${taskId}`);
  const record = await api<{ tool_calls: Array<{ tool_name: string }> }>(page, `/api/v1/task-runs/${runs.runs[0]?.id}`);
  expect(record.tool_calls.some((call) => call.tool_name === 'lookup_status')).toBeTruthy();
}

async function sendChat(page: Page, content: string): Promise<void> {
  await page.getByPlaceholder('Send a message...').fill(content);
  await page.getByRole('button', { name: 'Send' }).click();
}

async function selectModel(page: Page, label: string, modelId: string): Promise<void> {
  await page.getByRole('button', { name: new RegExp(label, 'i') }).click();
  await page.getByPlaceholder('Search provider, model, capability').fill(modelId);
  await page.getByRole('option', { name: new RegExp(modelId.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) }).first().click();
}

async function openFirstMessageActions(page: Page, actionName: string): Promise<void> {
  const menus = page.getByRole('button', { name: 'Actions' });
  const count = await menus.count();
  for (let index = 0; index < count; index += 1) {
    await menus.nth(index).click();
    const action = page.getByRole('button', { name: actionName }).first();
    if (await action.isVisible().catch(() => false)) {
      await action.click();
      return;
    }
    await menus.nth(index).click();
  }
  throw new Error(`Message action not found: ${actionName}`);
}

async function api<T = unknown>(page: Page, path: string, method = 'GET', body?: unknown): Promise<T> {
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
  ) as Promise<T>;
}
