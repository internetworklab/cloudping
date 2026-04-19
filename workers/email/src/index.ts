import { EmailMessage } from 'cloudflare:email';
import { createMimeMessage } from 'mimetext';

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
	async email(message, env, ctx) {
		const serviceTokenId = await env['TUI_SERVICE_TOKEN_ID'].get();
		const serviceTokenSecret = await env['TUI_SERVICE_TOKEN_SECRET'].get();

		const upstreamResponse = await fetch('https://test-tui.ping2.sh/', {
			method: 'POST',
			body: message.raw,
			headers: {
				'Content-Type': message.headers.get('Content-Type') ?? '',
				'CF-Access-Client-Id': serviceTokenId,
				'CF-Access-Client-Secret': serviceTokenSecret,
			},
		});

		const upstreamText = await upstreamResponse.text();

		const senderOfReply = 'cli@ping2.sh';
		const nameOfSenderOfReply = 'Cloudping';
		const subjectOfReply = 'Re: ' + message.headers.get('Subject');

		const msg = createMimeMessage();
		msg.setHeader('In-Reply-To', message.headers.get('Message-ID'));
		msg.setSender({ name: nameOfSenderOfReply, addr: senderOfReply });
		msg.setRecipient(message.from);
		msg.setSubject(subjectOfReply);
		msg.addMessage({
			contentType: 'text/html',
			data: upstreamText,
		});

		const replyMessage = new EmailMessage(senderOfReply, message.from, msg.asRaw());

		await message.reply(replyMessage);
	},
	async fetch(request, env, ctx): Promise<Response> {
		const url = new URL(request.url);
		switch (url.pathname) {
			default:
				return new Response('Not Found', { status: 404 });
		}
	},
} satisfies ExportedHandler<Env>;
