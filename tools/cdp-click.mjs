import { writeFileSync } from 'node:fs';

const base = 'http://127.0.0.1:9222';
const [url, clickText, out] = process.argv.slice(2);

const tab = await (await fetch(`${base}/json/new?about:blank`, { method: 'PUT' })).json();
const ws = new WebSocket(tab.webSocketDebuggerUrl);
let id = 0;
const pending = new Map();
const send = (method, params = {}) =>
	new Promise((resolve) => {
		const reqId = ++id;
		pending.set(reqId, resolve);
		ws.send(JSON.stringify({ id: reqId, method, params }));
	});

ws.onmessage = (ev) => {
	const msg = JSON.parse(ev.data);
	if (msg.id && pending.has(msg.id)) {
		pending.get(msg.id)(msg.result);
		pending.delete(msg.id);
	}
	if (msg.method === 'Runtime.exceptionThrown')
		console.log('EXCEPTION', msg.params.exceptionDetails.exception?.description);
	if (msg.method === 'Runtime.consoleAPICalled' && msg.params.type === 'error')
		console.log('CONSOLE ERROR', msg.params.args.map((a) => a.value ?? a.description).join(' '));
};

await new Promise((r) => (ws.onopen = r));
await send('Runtime.enable');
await send('Page.enable');
await send('Emulation.setDeviceMetricsOverride', { width: 1280, height: 1200, deviceScaleFactor: 1, mobile: false });
await send('Page.navigate', { url });
await new Promise((r) => setTimeout(r, 4000));

if (clickText) {
	const res = await send('Runtime.evaluate', {
		expression: `(() => { const b = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === ${JSON.stringify(clickText)}); if (!b) return 'not-found'; b.click(); return 'clicked'; })()`,
		returnByValue: true
	});
	console.log('click:', res.result.value);
	await new Promise((r) => setTimeout(r, 5000));
}

const text = await send('Runtime.evaluate', { expression: 'document.body.innerText.slice(0,400)', returnByValue: true });
console.log(text.result.value);
const shot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
if (out && shot.data) writeFileSync(out, Buffer.from(shot.data, 'base64'));
ws.close();
