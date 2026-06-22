export const strings = {
  appName: 'Nostos',
  nav: {
    chat: 'Chat',
    agents: 'Agents',
    memories: 'Memories',
    tasks: 'Tasks',
    mcp: 'MCP',
    providers: 'Providers',
    settings: 'Settings'
  },
  auth: {
    setupTitle: 'Create owner account',
    setupSubtitle: 'Setup is available only until the first owner is created.',
    loginTitle: 'Sign in',
    loginSubtitle: 'Use the local owner account for this workspace.',
    email: 'Email',
    displayName: 'Display name',
    password: 'Password',
    confirmPassword: 'Confirm password',
    createOwner: 'Create owner',
    signIn: 'Sign in',
    signOut: 'Sign out',
    sessions: 'Sessions',
    revoke: 'Revoke',
    currentUser: 'Current user'
  },
  workspace: {
    title: 'AI command center',
    subtitle: 'Local workspace for chat, agents, memories, MCP tools, providers, tasks, and security controls.',
    diagnostics: 'Diagnostics',
    emptyScreen: 'This screen is connected and ready for Version 0.1 data.',
    status: 'System status'
  },
  chat: {
    newConversation: 'New conversation',
    send: 'Send',
    stop: 'Stop',
    composerPlaceholder: 'Send a message...',
    noMessages: 'No messages yet.',
    noConversations: 'No conversations yet.',
    memoriesUsed: 'Memories used in this response',
    remember: 'Remember'
  },
  providers: {
    title: 'Providers',
    add: 'Add provider',
    test: 'Test',
    refreshModels: 'Refresh models',
    noProviders: 'No providers configured yet.',
    apiKeyHelp: 'API keys are encrypted before storage. Use env:NAME to read from an environment variable.'
  },
  agents: {
    add: 'Add agent',
    noAgents: 'No agents configured yet.',
    duplicate: 'Duplicate'
  },
  memories: {
    add: 'Add memory',
    noMemories: 'No memories yet.',
    pin: 'Pin',
    delete: 'Delete'
  },
  tasks: {
    add: 'Add task',
    noTasks: 'No tasks configured yet.',
    noRuns: 'No task runs yet.',
    runNow: 'Run now',
    cancel: 'Cancel',
    retry: 'Retry',
    events: 'Run events'
  },
  mcp: {
    add: 'Add MCP server',
    discover: 'Discover tools',
    noServers: 'No MCP servers configured yet.',
    noTools: 'No tools discovered yet.'
  }
} as const;
