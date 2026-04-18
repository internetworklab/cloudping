/**
 * Welcome to Cloudflare Workers! This is your first worker.
 *
 * - Run `npm run dev` in your terminal to start a development server
 * - Open a browser tab at http://localhost:8787/ to see your worker in action
 * - Run `npm run deploy` to publish your worker
 *
 * Bind resources to your worker in `wrangler.jsonc`. After adding bindings, a type definition for the
 * `Env` object can be regenerated with `npm run cf-typegen`.
 *
 * Learn more at https://developers.cloudflare.com/workers/
 */

export default {
	async fetch(request, env, ctx): Promise<Response> {
		const url = new URL(request.url);
		switch (url.pathname) {
			case '/message':
				return new Response('Hello, World!');
			case '/list':
				const serviceTokenId = await env['TUI_SERVICE_TOKEN_ID'].get();
				const serviceTokenSecret = await env['TUI_SERVICE_TOKEN_SECRET'].get();

				const upstreamResponse = await fetch('https://test-tui.ping2.sh/list', {
					headers: { 'CF-Access-Client-Id': serviceTokenId, 'CF-Access-Client-Secret': serviceTokenSecret },
				});

				const upstreamText = await upstreamResponse.text();

				// Send a welcome email
				// const response = await env['EMAIL'].send({
				// 	to: 'i@exploro.one',
				// 	from: 'hello@ping2.sh',
				// 	subject: 'Test command list',
				// 	html: upstreamText,
				// });

				return new Response(upstreamResponse.body, {
					status: response.status,
					statusText: response.statusText,
					headers: response.headers,
				});

			case '/random':
				return new Response(crypto.randomUUID());
			default:
				return new Response('Not Found', { status: 404 });
		}
	},
} satisfies ExportedHandler<Env>;
