// Drives the custom date inputs in headless Chromium and reports what the page shows.
const base = 'http://127.0.0.1:9222';
const url = process.argv[2];
const tab = await (await fetch(`${base}/json/new?about:blank`, { method: 'PUT' })).json();
const ws = new WebSocket(tab.webSocketDebuggerUrl);
let id = 0;
const pending = new Map();
const errors = [];
const send = (method, params = {}) =>
	new Promise((resolve) => {
		const n = ++id;
		pending.set(n, resolve);
		ws.send(JSON.stringify({ id: n, method, params }));
	});
ws.onmessage = (ev) => {
	const m = JSON.parse(ev.data);
	if (m.id && pending.has(m.id)) pending.get(m.id)(m.result);
	if (m.method === 'Runtime.exceptionThrown') errors.push(m.params.exceptionDetails.text + ' ' + (m.params.exceptionDetails.exception?.description ?? ''));
	if (m.method === 'Runtime.consoleAPICalled' && m.params.type === 'error') errors.push(m.params.args.map((a) => a.value ?? a.description).join(' '));
};
await new Promise((r) => (ws.onopen = r));
await send('Runtime.enable');
await send('Page.enable');
await send('Page.navigate', { url });
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const evalJS = async (expression) => (await send('Runtime.evaluate', { expression, returnByValue: true })).result.value;
await wait(12000);

const before = await evalJS(`[...document.querySelectorAll('input[type=date]')].map(i => i.min + '|' + i.value)`);
console.log('inputs before', JSON.stringify(before));
await evalJS(`(() => {
  const [f, t] = document.querySelectorAll('input[type=date]');
  const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('change', { bubbles: true })); };
  set(f, '2023-06-01');
  set(t, '2023-06-30');
})()`);
await wait(2500);
const text = await evalJS('document.body.innerText');
console.log('mentions Jun 2023 range:', /6\/1\/2023|Jun 1/.test(text), '| 6/30/2023:', /6\/30\/2023|Jun 30/.test(text));
console.log('footer:', text.split('\n').filter((l) => l.includes('records since')).join(' / '));
console.log('summary requests:', JSON.stringify(await evalJS("performance.getEntriesByType('resource').map(r => r.name).filter(n => n.includes('api/summary')).map(n => n.replace(location.origin, ''))")));
console.log('errors:', JSON.stringify(errors));
await fetch(`${base}/json/close/${tab.id}`);
ws.close();
