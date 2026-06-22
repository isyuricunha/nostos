import { mount } from 'svelte';

import './styles.css';
import App from './App.svelte';

const target = document.getElementById('app');

if (!target) {
  throw new Error('Application target element was not found.');
}

const app = mount(App, {
  target
});

export default app;
