const base = 'http://127.0.0.1:9222';

async function main() {
	const tab = await (await fetch(`${base}/json/new?about:blank`, { method: 'PUT' })).json();
	const ws = new WebSocket(tab.webSocketDebuggerUrl);
	let id = 0;
	const send = (method, params = {}) => ws.send(JSON.stringify({ id: ++id, method, params }));

	ws.onmessage = (ev) => {
		const msg = JSON.parse(ev.data);
		if (msg.method === 'Runtime.consoleAPICalled') {
			console.log('CONSOLE', msg.params.type, msg.params.args.map((a) => a.value ?? a.description).join(' '));
		}
		if (msg.method === 'Runtime.exceptionThrown') {
			console.log('EXCEPTION', msg.params.exceptionDetails.text, msg.params.exceptionDetails.exception?.description ?? '');
		}
		if (msg.method === 'Network.responseReceived') {
			const { url, status } = msg.params.response;
			if (url.includes('/api/')) console.log('API', status, url);
		}
		if (msg.method === 'Network.loadingFailed') {
			console.log('FAILED', msg.params.errorText, msg.params.requestId);
		}
	};

	await new Promise((r) => (ws.onopen = r));
	send('Runtime.enable');
	send('Network.enable');
	send('Page.enable');
	send('Page.navigate', { url: process.argv[2] });
	await new Promise((r) => setTimeout(r, 9000));
	const reqId = ++id;
	ws.send(JSON.stringify({ id: reqId, method: 'Runtime.evaluate', params: { expression: 'document.body.innerText.slice(0, 2000)', returnByValue: true } }));
	const text = await new Promise((resolve) => {
		const handler = (ev) => {
			const m = JSON.parse(ev.data);
			if (m.id === reqId) { ws.removeEventListener('message', handler); resolve(m.result?.result?.value); }
		};
		ws.addEventListener('message', handler);
	});
	console.log('--- BODY ---');
	console.log(text);
	ws.close();
}

main();
