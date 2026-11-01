import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { mkdtemp, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { runAgent } from './agent.mjs';
import { fileURLToPath } from 'node:url';

const secret = 'pi-key-must-never-appear-in-results';
const scriptPath = fileURLToPath(new URL('./agent.mjs', import.meta.url));
const call = (name, input) => ({ type: 'tool_use', id: `call-${name}-${Math.random().toString(36).slice(2)}`, name, input });

function stream(res, blocks, stop = 'end_turn') {
  res.writeHead(200, { 'content-type': 'text/event-stream' });
  const send = (type, value) => res.write(`event: ${type}\ndata: ${JSON.stringify({ type, ...value })}\n\n`);
  send('message_start', { message: {
    id: 'msg-test', type: 'message', role: 'assistant', model: 'test-model', content: [],
    stop_reason: null, stop_sequence: null,
    usage: { input_tokens: 11, output_tokens: 0, cache_read_input_tokens: 2, cache_creation_input_tokens: 3 },
  } });
  blocks.forEach((block, index) => {
    if (block.type === 'tool_use') {
      send('content_block_start', { index, content_block: { ...block, input: {} } });
      send('content_block_delta', { index, delta: { type: 'input_json_delta', partial_json: JSON.stringify(block.input) } });
    } else {
      send('content_block_start', { index, content_block: { type: 'text', text: '' } });
      send('content_block_delta', { index, delta: { type: 'text_delta', text: block.text } });
    }
    send('content_block_stop', { index });
  });
  send('message_delta', { delta: { stop_reason: stop, stop_sequence: null }, usage: { output_tokens: 7 } });
  send('message_stop', {});
  res.end();
}

async function fixture(t, respond, protocol = 'anthropic') {
  const dir = await mkdtemp(path.join(tmpdir(), 'study-pi-'));
  const cwd = process.cwd();
  process.chdir(dir);
  let count = 0;
  const server = createServer(async (req, res) => {
    let raw = '';
    for await (const chunk of req) raw += chunk;
    try {
      assert.equal(req.url.split('?')[0], protocol === 'openai' ? '/compatible/v1/chat/completions' : '/anthropic/v1/messages');
      assert.equal(protocol === 'openai' ? req.headers.authorization : req.headers['x-api-key'], protocol === 'openai' ? `Bearer ${secret}` : secret);
      const body = JSON.parse(raw);
      assert.equal(body.model, 'custom-gateway-model');
      assert.equal(body.stream, true, 'both protocols must request an upstream stream');
      await respond(res, body, count++);
    } catch (error) {
      t.diagnostic(error.message);
      res.writeHead(500); res.end(JSON.stringify({ error: { message: error.message } }));
    }
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(async () => {
    process.chdir(cwd);
    await new Promise((resolve) => server.close(resolve));
    await rm(dir, { recursive: true, force: true });
  });
  const env = {
      HOME: dir, PATH: process.env.PATH, ANTHROPIC_AUTH_TOKEN: secret,
      ANTHROPIC_MODEL: 'custom-gateway-model',
      ...(protocol === 'openai' ? { AI_STUDY_TUTOR_AI_PROTOCOL: 'openai' } : {}),
      ANTHROPIC_BASE_URL: `http://127.0.0.1:${server.address().port}/${protocol === 'openai' ? 'compatible/v1' : 'anthropic'}`,
  };
  return {
    dir,
    env,
    run: (tools, onProgress) => runAgent({ prompt: 'Read the supplied material and produce the required files.', tools, env, onProgress }),
    count: () => count,
  };
}

test('real Pi reads an image and transcript, writes URL files, and aggregates usage without saved sessions', async (t) => {
  let sawImage = false;
  let sawTranscript = false;
  const f = await fixture(t, (res, body, round) => {
    assert.deepEqual(body.tools.map((tool) => tool.name).sort(), ['read', 'write']);
    const payload = JSON.stringify(body.messages);
    if (round > 0) sawImage ||= payload.includes('"type":"image"') && payload.includes('"media_type":"image/png"');
    if (round > 1) sawTranscript ||= payload.includes('完整字幕：推理延迟取决于计算和带宽。');
    if (round === 0) return stream(res, [call('read', { path: 'frames.png' })], 'tool_use');
    if (round === 1) return stream(res, [call('read', { path: 'transcript.md' })], 'tool_use');
    if (round === 2) return stream(res, [
      call('write', { path: 'extracted_title.txt', content: '推理性能' }),
      call('write', { path: 'extracted_raw_text.md', content: '## 画面内容\n性能图。\n## 综合总结\n计算与带宽。' }),
      call('write', { path: 'extracted_note.txt', content: '字幕与采样画面。' }),
    ], 'tool_use');
    stream(res, [{ type: 'text', text: 'done' }]);
  });
  await writeFile('frames.png', Buffer.from('iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAIAAACQkWg2AAAAF0lEQVR4nGP4z8BAEiJN9aiGUQ1DSgMAkPn/Afnh+ngAAAAASUVORK5CYII=', 'base64'));
  await writeFile('transcript.md', '完整字幕：推理延迟取决于计算和带宽。');
  const result = await f.run(['read', 'write']);
  assert.equal(result.is_error, false, JSON.stringify(result));
  assert.equal(result.result, 'done');
  assert.ok(sawImage && sawTranscript, 'actual image pixels and transcript must reach the provider');
  assert.equal(await readFile('extracted_title.txt', 'utf8'), '推理性能');
  assert.match(await readFile('extracted_raw_text.md', 'utf8'), /综合总结/);
  assert.deepEqual(result.usage, { input_tokens: 44, output_tokens: 28, cache_read_input_tokens: 8, cache_creation_input_tokens: 12 });
  assert.equal(f.count(), 4);
  assert.ok(!JSON.stringify(result).includes(secret));
  assert.deepEqual((await readdir(f.dir)).sort(), ['extracted_note.txt', 'extracted_raw_text.md', 'extracted_title.txt', 'frames.png', 'transcript.md']);
});

test('real Pi generates quiz.json and validates it through bash', async (t) => {
  const quiz = { title: '测验', questions: [{ stem: '推理受哪些因素影响？' }] };
  const f = await fixture(t, (res, body, round) => {
    assert.deepEqual(body.tools.map((tool) => tool.name).sort(), ['bash', 'read', 'write']);
    if (round === 0) return stream(res, [call('read', { path: 'raw_text.md' }), call('read', { path: 'student_notes.md' })], 'tool_use');
    if (round === 1) return stream(res, [call('write', { path: 'quiz.json', content: JSON.stringify(quiz) })], 'tool_use');
    if (round === 2) return stream(res, [call('bash', { command: 'python3 -m json.tool quiz.json > /dev/null' })], 'tool_use');
    assert.ok(!JSON.stringify(body.messages).includes('Exit code: 1'));
    stream(res, [{ type: 'text', text: 'done' }]);
  });
  await writeFile('raw_text.md', '推理取决于计算与带宽。');
  await writeFile('student_notes.md', '');
  assert.equal((await f.run(['bash', 'read', 'write'])).is_error, false);
  assert.deepEqual(JSON.parse(await readFile('quiz.json', 'utf8')), quiz);
  assert.equal(f.count(), 4);
});

for (const status of [401, 403, 429, 503]) {
  test(`HTTP ${status} is preserved without leaking upstream error text or retrying`, async (t) => {
    const f = await fixture(t, (res) => {
      res.writeHead(status, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ type: 'error', error: { type: 'api_error', message: `echoed secret: ${secret}` } }));
    });
    const result = await f.run(['read']);
    assert.equal(result.is_error, true);
    assert.equal(result.api_error_status, status);
    assert.equal(f.count(), 1);
    assert.ok(!JSON.stringify(result).includes(secret));
  });
}

test('a truncated response is a failure even when prompt resolves', async (t) => {
  const f = await fixture(t, (res) => stream(res, [{ type: 'text', text: 'partial output' }], 'max_tokens'));
  const result = await f.run(['read']);
  assert.equal(result.is_error, true);
  assert.equal(result.result, '');
});

for (const status of [200, 401]) {
  test(`Node entry point emits one JSON envelope and the correct exit status (${status})`, async (t) => {
    const f = await fixture(t, (res) => {
      if (status === 200) return stream(res, [{ type: 'text', text: 'done' }]);
      res.writeHead(status, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: { type: 'authentication_error', message: secret } }));
    });
    const child = spawn(process.execPath, [scriptPath, '--tools', 'read', '--prompt', 'Reply done.'], { cwd: f.dir, env: f.env });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (chunk) => { stdout += chunk; });
    child.stderr.on('data', (chunk) => { stderr += chunk; });
    const code = await new Promise((resolve, reject) => { child.once('error', reject); child.once('close', resolve); });
    assert.equal(code, status === 200 ? 0 : 1);
    assert.equal(stdout.trim().split('\n').length, 1);
    assert.equal(JSON.parse(stdout).is_error, status !== 200);
    const progress = stderr.trim().split('\n').map((line) => JSON.parse(line));
    assert.ok(progress.length > 0);
    assert.ok(progress.every((event) => event.type === 'pi_progress'));
    assert.ok(!stderr.includes(secret));
    assert.ok(!stdout.includes(secret));
  });
}

test('CLI forwards safe progress before the upstream stream finishes', { timeout: 10000 }, async (t) => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  t.after(() => release());
  const f = await fixture(t, async (res) => {
    res.writeHead(200, { 'content-type': 'text/event-stream' });
    const send = (type, value) => res.write(`event: ${type}\ndata: ${JSON.stringify({ type, ...value })}\n\n`);
    send('message_start', { message: { id: 'live', type: 'message', role: 'assistant', model: 'custom-gateway-model', content: [], stop_reason: null, stop_sequence: null, usage: { input_tokens: 1, output_tokens: 0 } } });
    send('content_block_start', { index: 0, content_block: { type: 'text', text: '' } });
    send('content_block_delta', { index: 0, delta: { type: 'text_delta', text: `private material ${secret}` } });
    await gate;
    send('content_block_stop', { index: 0 });
    send('message_delta', { delta: { stop_reason: 'end_turn' }, usage: { output_tokens: 3 } });
    send('message_stop', {});
    res.end();
  });
  const child = spawn(process.execPath, [scriptPath, '--tools', 'read', '--prompt', 'Reply done.'], { cwd: f.dir, env: f.env });
  t.after(() => child.kill());
  let stdout = '';
  let stderr = '';
  let buffered = '';
  const observed = new Promise((resolve) => child.stderr.on('data', (chunk) => {
    stderr += chunk;
    buffered += chunk;
    const lines = buffered.split('\n');
    buffered = lines.pop();
    for (const line of lines) {
      const event = JSON.parse(line);
      if (event.phase === 'generating' && event.characters > 0) resolve(event);
    }
  }));
  child.stdout.on('data', (chunk) => { stdout += chunk; });
  const closed = new Promise((resolve, reject) => { child.once('error', reject); child.once('close', resolve); });
  const progress = await observed;
  assert.equal(stdout, '', 'no final envelope before upstream completion');
  assert.equal(progress.turns, 1);
  assert.ok(!stderr.includes('private material') && !stderr.includes(secret));
  release();
  assert.equal(await closed, 0);
  assert.equal(JSON.parse(stdout).is_error, false);
});

function openaiStream(res, blocks) {
  res.writeHead(200, { 'content-type': 'text/event-stream' });
  const chunk = (delta, finish = null, usage) => res.write(`data: ${JSON.stringify({ id: 'chat-test', object: 'chat.completion.chunk', model: 'custom-gateway-model', choices: [{ index: 0, delta, finish_reason: finish }], ...(usage ? { usage } : {}) })}\n\n`);
  chunk({ role: 'assistant' });
  blocks.forEach((block, index) => {
    if (block.type === 'tool_use') {
      chunk({ tool_calls: [{ index, id: block.id, type: 'function', function: { name: block.name, arguments: '' } }] });
      chunk({ tool_calls: [{ index, function: { arguments: JSON.stringify(block.input) } }] });
    } else chunk({ content: block.text });
  });
  chunk({}, blocks.some((block) => block.type === 'tool_use') ? 'tool_calls' : 'stop', { prompt_tokens: 11, completion_tokens: 7, prompt_tokens_details: { cached_tokens: 3 } });
  res.end('data: [DONE]\n\n');
}

test('OpenAI protocol carries real image pixels and tool results through Pi', async (t) => {
  let imageSent = false;
  const f = await fixture(t, (res, body, round) => {
    assert.equal(body.max_tokens, 8192);
    assert.ok(!('store' in body));
    if (round === 0) return openaiStream(res, [call('read', { path: 'frames.png' })]);
    if (round === 1) {
      imageSent = JSON.stringify(body.messages).includes('data:image/png;base64,');
      return openaiStream(res, [call('write', { path: 'extracted_raw_text.md', content: '画面显示推理性能图。' })]);
    }
    assert.ok(JSON.stringify(body.messages).includes('Successfully wrote'));
    openaiStream(res, [{ type: 'text', text: 'done' }]);
  }, 'openai');
  await writeFile('frames.png', Buffer.from('iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAIAAACQkWg2AAAAF0lEQVR4nGP4z8BAEiJN9aiGUQ1DSgMAkPn/Afnh+ngAAAAASUVORK5CYII=', 'base64'));
  const result = await f.run(['read', 'write']);
  assert.equal(result.is_error, false, JSON.stringify(result));
  assert.ok(imageSent, 'OpenAI must receive the image, not just its filename');
  assert.equal(await readFile('extracted_raw_text.md', 'utf8'), '画面显示推理性能图。');
  assert.equal(f.count(), 3);
  assert.deepEqual(result.usage, { input_tokens: 24, output_tokens: 21, cache_read_input_tokens: 9, cache_creation_input_tokens: 0 });
});

test('OpenAI authentication errors remain sanitized', async (t) => {
  const f = await fixture(t, (res) => {
    res.writeHead(401, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ error: { message: secret, type: 'authentication_error' } }));
  }, 'openai');
  const result = await f.run(['read']);
  assert.equal(result.is_error, true);
  assert.equal(result.api_error_status, 401);
  assert.ok(!JSON.stringify(result).includes(secret));
  assert.equal(f.count(), 1);
});

for (const protocol of ['anthropic', 'openai']) {
  test(`${protocol} quota exhaustion is distinguished from authentication failure without exposing balances`, async (t) => {
    const f = await fixture(t, (res) => {
      res.writeHead(403, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: { code: 'insufficient_user_quota', type: 'new_api_error', message: `remaining balance: -0.130036, token: ${secret}` } }));
    }, protocol);
    const result = await f.run(['read']);
    assert.equal(result.is_error, true);
    assert.equal(result.api_error_status, 403);
    assert.equal(result.api_error_code, 'claude_quota_exhausted');
    assert.ok(!JSON.stringify(result).includes(secret));
    assert.ok(!JSON.stringify(result).includes('-0.130036'));
    assert.equal(f.count(), 1);
  });
}
