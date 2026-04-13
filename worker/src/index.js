import PostalMime from 'postal-mime';

export default {
  async email(message, env, ctx) {
    const toAddress = message.to;
    const plusMatch = toAddress.match(/home\+(\w+)@/);
    const tag = plusMatch ? plusMatch[1] : null;

    // Parse the full email with postal-mime
    const rawEmail = await new Response(message.raw).arrayBuffer();
    const parser = new PostalMime();
    const parsed = await parser.parse(rawEmail);

    const payload = {
      from: message.from,
      to: toAddress,
      subject: parsed.subject || '',
      tag: tag,
      body: (parsed.text || '').substring(0, 5000),
      received_at: new Date().toISOString(),
    };

    try {
      const response = await fetch(env.WEBHOOK_URL, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${env.INGEST_SECRET}`,
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        await message.forward(env.FALLBACK_EMAIL);
      }
    } catch (err) {
      await message.forward(env.FALLBACK_EMAIL);
    }
  },
};
