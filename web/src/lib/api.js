export async function getJSON(url) {
	const res = await fetch(url);
	if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
	return res.json();
}
