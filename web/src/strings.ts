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
  }
} as const;
